package wrench

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/hexops/wrench/internal/errors"
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
	`)
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS runners (
			id TEXT PRIMARY KEY NOT NULL,
			arch TEXT NOT NULL,
			registered_at TIMESTAMP NOT NULL,
			last_seen_at TIMESTAMP NOT NULL
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

func (s *Store) RunnerSeen(ctx context.Context, id, arch string) error {
	now := time.Now()
	q := sqlf.Sprintf(
		`INSERT OR IGNORE INTO runners(id, arch, registered_at, last_seen_at) VALUES (%v, %v, %v, %v)
		UPDATE runners SET registered_at = %v, last_seen_at = %v WHERE id=%v`,
		id, arch, now, now,
		now, now, id,
	)
	_, err := s.db.ExecContext(ctx, q.Query(sqlf.SimpleBindVar), q.Args()...)
	return err
}

type Runner struct {
	ID, Arch                 string
	RegisteredAt, LastSeenAt time.Time
}

func (s *Store) Runners(ctx context.Context, id string) ([]Runner, error) {
	q := sqlf.Sprintf(`SELECT id, arch, registered_at, last_seen_at FROM runners ORDER BY id`)

	rows, err := s.db.QueryContext(ctx, q.Query(sqlf.SimpleBindVar), q.Args()...)
	if err != nil {
		return nil, errors.Wrap(err, "QueryContext")
	}

	var runners []Runner
	for rows.Next() {
		var runner Runner
		if err = rows.Scan(&runner.ID, &runner.Arch, &runner.RegisteredAt, &runner.LastSeenAt); err != nil {
			return nil, errors.Wrap(err, "Scan")
		}
		runners = append(runners, runner)
	}
	return runners, rows.Err()
}

func (s *Store) Close() error {
	return s.db.Close()
}
