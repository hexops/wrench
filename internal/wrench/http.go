package wrench

import (
	"context"
	"crypto/subtle"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/google/go-github/github"
	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/wrench/api"
	"github.com/hexops/wrench/internal/wrench/scripts"
	"golang.org/x/crypto/acme/autocert"
)

type handlerFunc func(w http.ResponseWriter, r *http.Request) error

func (b *Bot) httpStart() error {
	if b.Config.Address == "" {
		b.logf("http: disabled (Config.Address not configured)")
		return nil
	}

	handler := func(prefix string, handle handlerFunc) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := handle(w, r)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "error: %s", err.Error())
				b.logf("http: %s: %v", prefix, err)
			}
		})
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Let's fix this!"))
	})
	mux.Handle("/webhook/github/self", handler("webhook", b.httpServeWebHookGitHubSelf))
	mux.Handle("/rebuild", handler("rebuild", b.httpBasicAuthMiddleware(b.httpServeRebuild)))
	mux.Handle("/logs/", handler("logs", b.httpServeLogs))
	mux.Handle("/runners/", handler("runners", b.httpServeRunners))
	mux.Handle("/api/runner/poll", handler("api-runner-poll", botHttpAPI(b, b.httpServeRunnerPoll)))
	mux.Handle("/api/runner/job-update", handler("api-runner-job-update", botHttpAPI(b, b.httpServeRunnerJobUpdate)))
	mux.Handle("/api/runner/list", handler("api-runner-list", botHttpAPI(b, b.httpServeRunnerList)))
	mux.Handle("/api/secrets/list", handler("api-secrets-list", botHttpAPI(b, b.httpServeSecretsList)))
	mux.Handle("/api/secrets/delete", handler("api-secrets-delete", botHttpAPI(b, b.httpServeSecretsDelete)))
	mux.Handle("/api/secrets/upsert", handler("api-secrets-upsert", botHttpAPI(b, b.httpServeSecretsUpsert)))

	b.logf("http: listening on %v - %v", b.Config.Address, b.Config.ExternalURL)
	if strings.HasSuffix(b.Config.Address, ":443") || strings.HasSuffix(b.Config.Address, ":https") {
		// Serve HTTPS using LetsEncrypt
		u, err := url.Parse(b.Config.ExternalURL)
		if err != nil {
			return fmt.Errorf("expected valid config.ExternalURL for LetsEncrypt, found: %v", b.Config.ExternalURL)
		}
		certManager := autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			Cache:      autocert.DirCache(b.Config.LetsEncryptCacheDir),
			Email:      b.Config.LetsEncryptEmail,
			HostPolicy: autocert.HostWhitelist(u.Hostname()),
		}

		server := &http.Server{
			Addr: ":https",
			TLSConfig: &tls.Config{
				GetCertificate: certManager.GetCertificate,
			},
			Handler: mux,
		}

		go http.ListenAndServe(":http", certManager.HTTPHandler(nil))

		// Key and cert are provided by LetsEncrypt
		go server.ListenAndServeTLS("", "")
		return nil
	}
	go http.ListenAndServe(b.Config.Address, mux)
	return nil
}

func (b *Bot) httpStop() error {
	if b.discordSession == nil {
		return nil
	}
	return b.discordSession.Close()
}

func (b *Bot) httpServeWebHookGitHubSelf(w http.ResponseWriter, r *http.Request) error {
	if b.Config.GitHubWebHookSecret == "" {
		b.logf("http: webhook: ignored: /webhook/github/self (config.GitHubWebHookSecret not set)")
		return nil
	}

	payload, err := github.ValidatePayload(r, []byte(b.Config.GitHubWebHookSecret))
	if err != nil {
		return errors.Wrap(err, "ValidatePayload")
	}
	defer r.Body.Close()

	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		return errors.Wrap(err, "parsing webhook")
	}

	_, ok := event.(*github.PushEvent)
	if !ok {
		return fmt.Errorf("unexpected event type: %s", github.WebHookType(r))
	}

	return b.runRebuild()
}

func (b *Bot) httpServeRebuild(w http.ResponseWriter, r *http.Request) error {
	return b.runRebuild()
}

func (b *Bot) runRebuild() error {
	b.webHookGitHubSelf.Lock()
	defer b.webHookGitHubSelf.Unlock()

	logID := "restart-self"
	w := b.idWriter(logID)

	b.idLogf(logID, "👀 I see new changes")

	err := scripts.Sequence(
		scripts.Exec("wrench script install-go", scripts.WorkDir(b.Config.WrenchDir)),
		scripts.Exec("wrench script rebuild-only", scripts.WorkDir(b.Config.WrenchDir)),
	)(w)
	if err != nil {
		b.discord("Oops, looks like I can't build myself? Logs: " + b.Config.ExternalURL + "/logs/restart-self")
		b.idLogf(logID, "build failure!")
		return nil
	}
	b.idLogf(logID, "build success! restarting..")
	return scripts.Exec("wrench svc restart").IgnoreError()(w)
}

func (b *Bot) httpServeLogs(w http.ResponseWriter, r *http.Request) error {
	_, id := path.Split(r.URL.Path)
	if id == "" {
		logIDs, err := b.store.LogIDs(r.Context())
		if err != nil {
			return errors.Wrap(err, "LogIDs")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<ul>`)
		for _, id := range logIDs {
			fmt.Fprintf(w, `<li><a href="%s/logs/%s">%s</a></li>`, b.Config.ExternalURL, id, id)
		}
		fmt.Fprintf(w, `</ul>`)
		return nil
	}

	logs, err := b.store.Logs(r.Context(), id)
	if err != nil {
		return errors.Wrap(err, "Logs")
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	for _, log := range logs {
		fmt.Fprintf(w, "%v %v\n", log.Time.UTC().Format(time.RFC3339), log.Message)
	}
	return nil
}

func (b *Bot) httpServeRunners(w http.ResponseWriter, r *http.Request) error {
	_, id := path.Split(r.URL.Path)
	if id != "" {
		runners, err := b.store.Runners(r.Context())
		if err != nil {
			return errors.Wrap(err, "Runners")
		}
		var runner *api.Runner
		for _, r := range runners {
			if r.ID == id {
				runner = &r
				break
			}
		}
		if runner == nil {
			return errors.New("no such runner")
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<h2>Runner %s:%s</h2>", runner.ID, runner.Arch)
		fmt.Fprintf(w, `<ul>`)
		for _, pair := range [][2]string{
			{"Registered", runner.RegisteredAt.UTC().Format(time.RFC3339)},
			{"Last seen", humanizeTimeRecent(runner.LastSeenAt)},
			{"Wrench version", runner.Env.WrenchVersion},
			{"Wrench commit title", runner.Env.WrenchCommitTitle},
			{"Wrench date", runner.Env.WrenchDate},
			{"Wrench Go version", runner.Env.WrenchGoVersion},
		} {
			fmt.Fprintf(w, `<li><strong>%s</strong>: %s</li>`, pair[0], pair[1])
		}
		fmt.Fprintf(w, `</ul>`)
		return nil
	}

	runners, err := b.store.Runners(r.Context())
	if err != nil {
		return errors.Wrap(err, "Runners")
	}
	jobs, err := b.store.Jobs(r.Context(),
		JobsFilter{NotState: api.JobStateSuccess},
		JobsFilter{NotState: api.JobStateError},
	)
	if err != nil {
		return errors.Wrap(err, "Jobs(0)")
	}
	finishedJobs, err := b.store.Jobs(r.Context(),
		JobsFilter{NotState: api.JobStateReady},
		JobsFilter{NotState: api.JobStateStarting},
		JobsFilter{NotState: api.JobStateRunning},
	)
	if err != nil {
		return errors.Wrap(err, "Jobs(1)")
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fmt.Fprintf(w, "<h2>Runners</h2>")
	{
		var values [][]string
		for _, runner := range runners {
			wrenchDate := runner.Env.WrenchDate
			wrenchDateTime, err := time.Parse(time.RFC3339, runner.Env.WrenchDate)
			if err == nil {
				wrenchDate = humanize.Time(wrenchDateTime)
			}
			values = append(values, []string{
				fmt.Sprintf(`<a href="/runners/%s">%s</a>`, runner.ID, runner.ID),
				runner.Arch,
				runner.RegisteredAt.UTC().Format(time.RFC3339),
				humanizeTimeRecent(runner.LastSeenAt),
				runner.Env.WrenchVersion,
				wrenchDate,
			})
		}
		tableStyle(w)
		table(w, []string{"id", "arch", "registered", "last seen", "version", "built"}, values)
	}

	fmt.Fprintf(w, "<h2>Jobs</h2>")
	{
		var values [][]string
		for _, job := range jobs {
			values = append(values, []string{
				fmt.Sprintf(`<a href="%v/logs/job-%v">%v</a>`, b.Config.ExternalURL, job.ID, job.ID),
				string(job.State),
				job.Title,
				job.TargetRunnerID,
				job.TargetRunnerArch,
				humanizeTimeMaybeZero(job.ScheduledStart),
				humanize.Time(job.Updated),
				job.Created.UTC().Format(time.RFC3339),
			})
		}
		tableStyle(w)
		table(w, []string{"id", "state", "title", "target runner ID", "target runner arch", "scheduled start", "last updated", "created"}, values)
	}
	fmt.Fprintf(w, "<h2>Finished jobs</h2>")
	{
		var values [][]string
		for _, job := range finishedJobs {
			values = append(values, []string{
				fmt.Sprintf(`<a href="%v/logs/job-%v">%v</a>`, b.Config.ExternalURL, job.ID, job.ID),
				string(job.State),
				job.Title,
				job.TargetRunnerID,
				job.TargetRunnerArch,
				humanizeTimeMaybeZero(job.ScheduledStart),
				humanize.Time(job.Updated),
				job.Created.UTC().Format(time.RFC3339),
			})
		}
		tableStyle(w)
		table(w, []string{"id", "state", "title", "target runner ID", "target runner arch", "scheduled start", "last updated", "created"}, values)
	}
	return nil
}

func humanizeTimeMaybeZero(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return humanize.Time(t)
}

func humanizeTimeRecent(t time.Time) string {
	if time.Since(t) < 10*time.Second {
		return "now"
	}
	return humanize.Time(t)
}

func tableStyle(w io.Writer) {
	fmt.Fprintf(w, `
<style>
table {
    border: solid 1px #DDEEEE;
    border-collapse: collapse;
    border-spacing: 0;
}
table thead th {
    border: solid 1px #DDEEEE;
    background-color: #DDEFEF;
    padding: 0.75rem;
    text-align: left;
}
table tbody td {
    border: solid 1px #DDEEEE;
    padding: 0.75rem;
}
</style>`)
}

func table(w io.Writer, rows []string, values [][]string) {
	fmt.Fprintf(w, `<table>`)
	fmt.Fprintf(w, `<thead><tr>`)
	for _, label := range rows {
		fmt.Fprintf(w, "<th>%s</th>", label)
	}
	fmt.Fprintf(w, `</tr></thead>`)
	fmt.Fprintf(w, `<tbody>`)
	for _, row := range values {
		fmt.Fprintf(w, `<tr>`)
		for _, value := range row {
			fmt.Fprintf(w, `<td>%s</td>`, value)
		}
		fmt.Fprintf(w, `</tr>`)
	}
	fmt.Fprintf(w, `</tbody></table>`)
}

func (b *Bot) httpBasicAuthMiddleware(handler handlerFunc) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		if b.Config.Secret == "" {
			return errors.New("API not enabled; Config.Secret not configured.")
		}

		_, pass, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(pass), []byte(b.Config.Secret)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="wrench"`)
			w.WriteHeader(401)
			w.Write([]byte("Unauthorised.\n"))
			return nil
		}

		return handler(w, r)
	}
}

func botHttpAPI[Request any, Response any](b *Bot, handler func(context.Context, *Request) (*Response, error)) handlerFunc {
	return b.httpBasicAuthMiddleware(func(w http.ResponseWriter, r *http.Request) error {
		if r.Method != "POST" {
			return errors.New("POST is required for this endpoint")
		}

		defer r.Body.Close()
		var req Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if err != io.EOF {
				return errors.Wrap(err, "Decode")
			}
		}
		resp, err := handler(r.Context(), &req)
		if err != nil {
			return err
		}
		return errors.Wrap(json.NewEncoder(w).Encode(resp), "Encode")
	})
}

func (b *Bot) httpServeRunnerPoll(ctx context.Context, r *api.RunnerPollRequest) (*api.RunnerPollResponse, error) {
	err := b.store.RunnerSeen(ctx, r.ID, r.Arch, r.Env)
	if err != nil {
		return nil, errors.Wrap(err, "RunnerSeen")
	}

	if r.Job != nil {
		// Update job state.
		job, err := b.store.JobByID(ctx, r.Job.ID)
		if err != nil {
			if err == ErrNotFound {
				return &api.RunnerPollResponse{NotFound: true}, nil
			}
			return nil, errors.Wrap(err, "JobsByID")
		}
		job.State = r.Job.State
		err = b.store.UpsertRunnerJob(ctx, job)
		if err != nil {
			return nil, errors.Wrap(err, "UpsertRunnerJob(0)")
		}

		// Log job messages.
		if r.Job.Log != "" {
			if b.Config.GitPushUsername != "" {
				r.Job.Log = strings.ReplaceAll(r.Job.Log, b.Config.GitPushUsername, "<redacted>")
			}
			if b.Config.GitPushPassword != "" {
				r.Job.Log = strings.ReplaceAll(r.Job.Log, b.Config.GitPushPassword, "<redacted>")
			}
			secrets, err := b.store.Secrets(ctx)
			if err != nil {
				return nil, errors.Wrap(err, "Secrets")
			}
			for _, secret := range secrets {
				r.Job.Log = strings.ReplaceAll(r.Job.Log, secret.Value, "<redacted>")
			}
			b.idLogf(r.Job.ID.LogID(), "%s", r.Job.Log)
		}
	}

	startNewJob := r.Job == nil ||
		r.Job.State == api.JobStateSuccess ||
		r.Job.State == api.JobStateError
	if startNewJob {
		b.jobAcquire.Lock()
		defer b.jobAcquire.Unlock()

		// If the runner is looking for new jobs, but we think it's currently starting or running
		// one, then that means that job died.
		deadJobs, err := b.store.Jobs(ctx,
			// Starting OR Running
			JobsFilter{NotState: api.JobStateSuccess},
			JobsFilter{NotState: api.JobStateError},
			JobsFilter{NotState: api.JobStateReady},
			// Assigned to this runner
			JobsFilter{TargetRunnerID: r.ID},
		)
		if err != nil {
			return nil, errors.Wrap(err, "Jobs(dead)")
		}
		for _, job := range deadJobs {
			if strings.Contains(job.Title, "script rebuild") {
				// `wrench script rebuild` is expected to not finish gracefully as the service will
				// restart itself before the job completes.
				job.State = api.JobStateSuccess
				b.idLogf(job.ID.LogID(), "runner restarted successfully")
			} else {
				b.idLogf(job.ID.LogID(), "runner stopped performing job unexpectedly: %v:%v", r.ID, r.Arch)
				job.State = api.JobStateError
			}
			err = b.store.UpsertRunnerJob(ctx, job)
			if err != nil {
				return nil, errors.Wrap(err, "UpsertRunnerJob(1)")
			}
		}

		// Identify if a new job is available.
		readyJobs, err := b.store.Jobs(ctx,
			JobsFilter{State: api.JobStateReady},
			JobsFilter{ScheduledStartGreaterEqualTo: time.Now()},
		)
		if err != nil {
			return nil, errors.Wrap(err, "Jobs(ready)")
		}
	jobSearch:
		for _, job := range readyJobs {
			archMatch := job.TargetRunnerArch == "" || job.TargetRunnerArch == r.Arch
			idMatch := job.TargetRunnerID == "" || job.TargetRunnerID == r.ID
			if archMatch && idMatch {
				needSecrets := job.Payload.SecretIDs
				secrets := map[string]string{}
				for _, secretID := range needSecrets {
					secret, err := b.store.Secret(ctx, secretID)
					if err != nil {
						b.idLogf(job.ID.LogID(), `could not find secret "%v": %v`, secretID, err)
						job.State = api.JobStateError
						err = b.store.UpsertRunnerJob(ctx, job)
						if err != nil {
							return nil, errors.Wrap(err, "UpsertRunnerJob(1)")
						}
						continue jobSearch
					}
					secrets[secretID] = secret.Value
				}

				job.State = api.JobStateStarting
				job.TargetRunnerID = r.ID // assign job to this runner
				err = b.store.UpsertRunnerJob(ctx, job)
				if err != nil {
					return nil, errors.Wrap(err, "UpsertRunnerJob(1)")
				}
				b.idLogf(job.ID.LogID(), "job assigned to runner: %v:%v", r.ID, r.Arch)
				return &api.RunnerPollResponse{Start: &api.RunnerJobStart{
					ID:                 job.ID,
					Title:              job.Title,
					Payload:            job.Payload,
					GitPushUsername:    b.Config.GitPushUsername,
					GitPushPassword:    b.Config.GitPushPassword,
					GitConfigUserName:  b.Config.GitConfigUserName,
					GitConfigUserEmail: b.Config.GitConfigUserEmail,
					Secrets:            secrets,
				}}, nil
			}
		}
	}
	return &api.RunnerPollResponse{}, nil
}

func (b *Bot) httpServeRunnerJobUpdate(ctx context.Context, r *api.RunnerJobUpdateRequest) (*api.RunnerJobUpdateResponse, error) {
	// Update job state.
	job, err := b.store.JobByID(ctx, r.Job.ID)
	if err != nil {
		if err == ErrNotFound {
			return &api.RunnerJobUpdateResponse{NotFound: true}, nil
		}
		return nil, errors.Wrap(err, "JobsByID")
	}
	job.State = r.Job.State
	err = b.store.UpsertRunnerJob(ctx, job)
	if err != nil {
		return nil, errors.Wrap(err, "UpsertRunnerJob(0)")
	}

	// Log job messages.
	if r.Job.Log != "" {
		if b.Config.GitPushUsername != "" {
			r.Job.Log = strings.ReplaceAll(r.Job.Log, b.Config.GitPushUsername, "<redacted>")
		}
		if b.Config.GitPushPassword != "" {
			r.Job.Log = strings.ReplaceAll(r.Job.Log, b.Config.GitPushPassword, "<redacted>")
		}
		secrets, err := b.store.Secrets(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "Secrets")
		}
		for _, secret := range secrets {
			r.Job.Log = strings.ReplaceAll(r.Job.Log, secret.Value, "<redacted>")
		}
		b.idLogf(r.Job.ID.LogID(), "%s", r.Job.Log)
	}
	return &api.RunnerJobUpdateResponse{}, nil
}

func (b *Bot) httpServeRunnerList(ctx context.Context, r *api.RunnerListRequest) (*api.RunnerListResponse, error) {
	runners, err := b.store.Runners(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "RunnerSeen")
	}
	return &api.RunnerListResponse{Runners: runners}, nil
}

func (b *Bot) httpServeSecretsList(ctx context.Context, r *api.SecretsListRequest) (*api.SecretsListResponse, error) {
	secrets, err := b.store.Secrets(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Secrets")
	}
	var ids []string
	for _, secret := range secrets {
		ids = append(ids, secret.ID)
	}
	return &api.SecretsListResponse{IDs: ids}, nil
}

func (b *Bot) httpServeSecretsDelete(ctx context.Context, r *api.SecretsDeleteRequest) (*api.SecretsDeleteResponse, error) {
	err := b.store.DeleteSecret(ctx, r.ID)
	if err != nil {
		return nil, errors.Wrap(err, "DeleteSecret")
	}
	return &api.SecretsDeleteResponse{}, nil
}

func (b *Bot) httpServeSecretsUpsert(ctx context.Context, r *api.SecretsUpsertRequest) (*api.SecretsUpsertResponse, error) {
	err := b.store.UpsertSecret(ctx, r.ID, r.Value)
	if err != nil {
		return nil, errors.Wrap(err, "UpsertSecret")
	}
	return &api.SecretsUpsertResponse{}, nil
}
