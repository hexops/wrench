package wrench

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/wrench/api"
	"github.com/jxskiss/base62"
	"github.com/keegancsmith/sqlf"

	_ "modernc.org/sqlite" // from https://gitlab.com/cznic/sqlite
)

type Store struct {
	db *sql.DB
}

func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, errors.Wrap(err, "Open")
	}
	s := &Store{db: db}
	if err := s.ensureSchema(); err != nil {
		db.Close()
		return nil, errors.Wrap(err, "ensureSchema")
	}
	return s, nil
}

func (s *Store) ensureSchema() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS logs (
			logid INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
			timestamp TIMESTAMP NOT NULL,
			id TEXT NOT NULL,
			message TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS runners (
			id TEXT PRIMARY KEY NOT NULL,
			arch TEXT NOT NULL,
			env TEXT NOT NULL,
			registered_at TIMESTAMP NOT NULL,
			last_seen_at TIMESTAMP NOT NULL
		);
		CREATE TABLE IF NOT EXISTS cache (
			cache_name TEXT NOT NULL,
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP NOT NULL,
			expires_at TIMESTAMP,
			PRIMARY KEY (cache_name, key)
		);
		CREATE TABLE IF NOT EXISTS runner_jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
			state TEXT NOT NULL,
			title TEXT NOT NULL,
			target_runner_id TEXT NOT NULL,
			target_runner_arch TEXT NOT NULL,
			payload TEXT NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP NOT NULL
		);
	`)
	return err
}

func (s *Store) Log(ctx context.Context, id, message string) error {
	q := sqlf.Sprintf(
		"INSERT INTO logs(timestamp, id, message) VALUES(%v, %v, %v)",
		time.Now(),
		id,
		strings.TrimSpace(message),
	)
	_, err := s.db.ExecContext(ctx, q.Query(sqlf.SimpleBindVar), q.Args()...)
	return err
}

type Log struct {
	Time    time.Time
	Message string
}

func (s *Store) Logs(ctx context.Context, id string) ([]Log, error) {
	q := sqlf.Sprintf(`SELECT timestamp, message FROM logs WHERE id=%v ORDER BY timestamp`, id)

	rows, err := s.db.QueryContext(ctx, q.Query(sqlf.SimpleBindVar), q.Args()...)
	if err != nil {
		return nil, errors.Wrap(err, "QueryContext")
	}

	var logs []Log
	for rows.Next() {
		var log Log
		if err = rows.Scan(&log.Time, &log.Message); err != nil {
			return nil, errors.Wrap(err, "Scan")
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

func (s *Store) LogIDs(ctx context.Context) ([]string, error) {
	q := sqlf.Sprintf(`SELECT DISTINCT id FROM logs ORDER BY id`)

	rows, err := s.db.QueryContext(ctx, q.Query(sqlf.SimpleBindVar), q.Args()...)
	if err != nil {
		return nil, errors.Wrap(err, "QueryContext")
	}

	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, errors.Wrap(err, "Scan")
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) RunnerSeen(ctx context.Context, id, arch string, env api.RunnerEnv) error {
	now := time.Now()
	envJSON, err := json.Marshal(env)
	if err != nil {
		return errors.Wrap(err, "Marshal")
	}
	q := sqlf.Sprintf(
		`INSERT INTO runners(id, arch, registered_at, last_seen_at, env) VALUES (%v, %v, %v, %v, %v)
		ON CONFLICT(id) DO UPDATE SET last_seen_at = %v, env = %v WHERE id=%v`,
		id, arch, now, now, string(envJSON),
		now, string(envJSON), id,
	)
	_, err = s.db.ExecContext(ctx, q.Query(sqlf.SimpleBindVar), q.Args()...)
	return err
}

func (s *Store) Runners(ctx context.Context) ([]api.Runner, error) {
	q := sqlf.Sprintf(`SELECT id, arch, env, registered_at, last_seen_at FROM runners ORDER BY id`)

	rows, err := s.db.QueryContext(ctx, q.Query(sqlf.SimpleBindVar), q.Args()...)
	if err != nil {
		return nil, errors.Wrap(err, "QueryContext")
	}

	var runners []api.Runner
	for rows.Next() {
		var runner api.Runner
		var envJSON string
		if err = rows.Scan(&runner.ID, &runner.Arch, &envJSON, &runner.RegisteredAt, &runner.LastSeenAt); err != nil {
			return nil, errors.Wrap(err, "Scan")
		}
		if err := json.Unmarshal([]byte(envJSON), &runner.Env); err != nil {
			return nil, errors.Wrap(err, "Unmarshal")
		}
		runners = append(runners, runner)
	}
	return runners, rows.Err()
}

func (s *Store) NewRunnerJob(ctx context.Context, job api.Job) (api.JobID, error) {
	now := time.Now()
	job.State = api.JobStateReady
	job.Updated = now
	job.Created = now
	if job.Title == "" {
		return "", errors.New("Job.Title missing")
	}
	payload, err := json.Marshal(job.Payload)
	if err != nil {
		return "", errors.Wrap(err, "Marshal")
	}
	q := sqlf.Sprintf(
		`INSERT INTO runner_jobs(
			state,
			title,
			target_runner_id,
			target_runner_arch,
			payload,
			updated_at,
			created_at
		) VALUES (%v, %v, %v, %v, %v, %v, %v)
		RETURNING id`,
		job.State,
		job.Title,
		job.TargetRunnerID,
		job.TargetRunnerArch,
		string(payload),
		job.Updated,
		job.Created,
	)
	row := s.db.QueryRowContext(ctx, q.Query(sqlf.SimpleBindVar), q.Args()...)
	id, err := s.scanUint64(row.Scan)
	if err != nil {
		return "", errors.Wrap(err, "scanJob")
	}
	return encodeJobID(id), nil
}

func encodeJobID(id uint64) api.JobID {
	return api.JobID(base62.EncodeToString(base62.FormatUint(id)))
}

func mustDecodeJobID(id api.JobID) uint64 {
	bytes, err := base62.DecodeString(string(id))
	if err != nil {
		panic("DecodeString: encountered illegal base62 string")
	}
	v, err := base62.ParseUint(bytes)
	if err != nil {
		panic("ParseUint: encountered illegal base62 string")
	}
	return v
}

func (s *Store) UpsertRunnerJob(ctx context.Context, job api.Job) error {
	now := time.Now()
	job.Updated = now
	job.Created = now
	if job.State == "" {
		return errors.New("Job.State missing")
	}
	if job.Title == "" {
		return errors.New("Job.Title missing")
	}
	payload, err := json.Marshal(job.Payload)
	if err != nil {
		return errors.Wrap(err, "Marshal")
	}
	q := sqlf.Sprintf(
		`INSERT INTO runner_jobs(
			id,
			state,
			title,
			target_runner_id,
			target_runner_arch,
			payload,
			updated_at,
			created_at
		) VALUES (%v, %v, %v, %v, %v, %v, %v, %v)
		ON CONFLICT(id) DO UPDATE SET
			state = %v,
			title = %v,
			target_runner_id = %v,
			target_runner_arch = %v,
			payload = %v,
			updated_at = %v
		WHERE id = %v`,
		mustDecodeJobID(job.ID),
		job.State,
		job.Title,
		job.TargetRunnerID,
		job.TargetRunnerArch,
		string(payload),
		job.Updated,
		job.Created,
		job.State,
		job.Title,
		job.TargetRunnerID,
		job.TargetRunnerArch,
		string(payload),
		job.Updated,
		mustDecodeJobID(job.ID),
	)
	_, err = s.db.ExecContext(ctx, q.Query(sqlf.SimpleBindVar), q.Args()...)
	return err
}

const jobFields = `
	id,
	state,
	title,
	target_runner_id,
	target_runner_arch,
	payload,
	updated_at,
	created_at
`

var ErrNotFound = errors.New("not found")

func (s *Store) JobByID(ctx context.Context, id api.JobID) (api.Job, error) {
	jobs, err := s.Jobs(ctx, JobsFilter{ID: id})
	if err != nil {
		return api.Job{}, err
	}
	if len(jobs) != 1 {
		return api.Job{}, ErrNotFound
	}
	return jobs[0], nil
}

type JobsFilter struct {
	State, NotState api.JobState
	Title, NotTitle string
	TargetRunnerID  string
	ID              api.JobID
}

func (s *Store) Jobs(ctx context.Context, filters ...JobsFilter) ([]api.Job, error) {
	var conds []*sqlf.Query
	for _, where := range filters {
		if where.State != "" {
			conds = append(conds, sqlf.Sprintf("state = %v", where.State))
		}
		if where.NotState != "" {
			conds = append(conds, sqlf.Sprintf("state != %v", where.NotState))
		}
		if where.Title != "" {
			conds = append(conds, sqlf.Sprintf("title = %v", where.Title))
		}
		if where.NotTitle != "" {
			conds = append(conds, sqlf.Sprintf("title != %v", where.NotTitle))
		}
		if where.TargetRunnerID != "" {
			conds = append(conds, sqlf.Sprintf("target_runner_id = %v", where.TargetRunnerID))
		}
		if where.ID != "" {
			conds = append(conds, sqlf.Sprintf("id = %v", mustDecodeJobID(where.ID)))
		}
	}

	whereClause := sqlf.Sprintf("")
	if len(conds) > 0 {
		whereClause = sqlf.Sprintf("WHERE %v", sqlf.Join(conds, "AND"))
	}
	q := sqlf.Sprintf(`SELECT `+jobFields+` FROM runner_jobs %s ORDER BY id DESC`, whereClause)

	rows, err := s.db.QueryContext(ctx, q.Query(sqlf.SimpleBindVar), q.Args()...)
	if err != nil {
		return nil, errors.Wrap(err, "QueryContext")
	}

	var jobs []api.Job
	for rows.Next() {
		job, err := s.scanJob(rows.Scan)
		if err != nil {
			return nil, errors.Wrap(err, "scanJob")
		}
		jobs = append(jobs, *job)
	}
	return jobs, rows.Err()
}

func (s *Store) scanJob(scan func(...any) error) (*api.Job, error) {
	var j api.Job
	var payload string
	var id uint64
	if err := scan(
		&id,
		&j.State,
		&j.Title,
		&j.TargetRunnerID,
		&j.TargetRunnerArch,
		&payload,
		&j.Updated,
		&j.Created,
	); err != nil {
		return nil, errors.Wrap(err, "Scan")
	}
	j.ID = encodeJobID(id)
	if err := json.Unmarshal([]byte(payload), &j.Payload); err != nil {
		return nil, errors.Wrap(err, "Unmarshal")
	}
	return &j, nil
}

func (s *Store) scanUint64(scan func(...any) error) (uint64, error) {
	var v uint64
	if err := scan(&v); err != nil {
		return 0, errors.Wrap(err, "Scan")
	}
	return v, nil
}

func (s *Store) CacheSet(ctx context.Context, cacheName, key, value string, expires *time.Time) error {
	now := time.Now()
	q := sqlf.Sprintf(
		`INSERT INTO cache(cache_name, key, value, updated_at, created_at, expires_at) VALUES (%v, %v, %v, %v, %v, %v)
		ON CONFLICT(cache_name, key) DO UPDATE SET
			value = %v,
			updated_at = %v,
			expires_at = %v
		WHERE cache_name = %v AND key = %v`,
		cacheName, key, value, now, now, expires,
		value, now, expires,
		cacheName, key,
	)
	_, err := s.db.ExecContext(ctx, q.Query(sqlf.SimpleBindVar), q.Args()...)
	return err
}

type CacheEntry struct {
	Value   string
	Updated time.Time
	Created time.Time
	Expires *time.Time
}

func (s *Store) CacheKey(ctx context.Context, cacheName, key string) (*CacheEntry, error) {
	q := sqlf.Sprintf(`SELECT value, updated_at, created_at, expires_at
		FROM cache WHERE cache_name = %v AND key = %v`, cacheName, key)

	row := s.db.QueryRowContext(ctx, q.Query(sqlf.SimpleBindVar), q.Args()...)
	var e CacheEntry
	if err := row.Scan(&e.Value, &e.Updated, &e.Created, &e.Expires); err != nil {
		return nil, errors.Wrap(err, "Scan")
	}
	return &e, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}
