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
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/wrench/api"
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
	mux.Handle("/runners", handler("runners", b.httpServeRunners))
	mux.Handle("/api/runner/poll", handler("api-runner-poll", botHttpAPI(b, b.httpServeRunnerPoll)))
	mux.Handle("/api/runner/list", handler("api-runner-list", botHttpAPI(b, b.httpServeRunnerList)))

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

	b.idLogf(logID, "👀 I see new changes")
	err := b.runWrench(logID, "script", "rebuild")
	if err != nil {
		b.discord("Oops, looks like I can't build myself? Logs: " + b.Config.ExternalURL + "/logs/restart-self")
		b.idLogf(logID, "build failure!")
		return nil
	}
	b.idLogf(logID, "build success! restarting..")

	return b.runWrench(logID, "svc", "restart")
}

func (b *Bot) runWrench(id string, args ...string) error {
	w := b.idWriter(id)
	cmd := exec.Command("wrench", args...)
	cmd.Dir = b.Config.WrenchDir
	cmd.Stderr = w
	cmd.Stdout = w
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			b.idLogf(id, "process finished: error: exit code: %v", exitError.ExitCode())
			return nil
		}
	}
	b.idLogf(id, "process finished")
	return nil
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
	for _, log := range logs {
		fmt.Fprintf(w, "%v %v\n", log.Time.UTC().Format(time.RFC3339), log.Message)
	}
	return nil
}

func (b *Bot) httpServeRunners(w http.ResponseWriter, r *http.Request) error {
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
			values = append(values, []string{
				runner.ID,
				runner.Arch,
				runner.RegisteredAt.UTC().Format(time.RFC3339),
				runner.LastSeenAt.UTC().Format(time.RFC3339),
			})
		}
		tableStyle(w)
		table(w, []string{"id", "arch", "registered", "last seen"}, values)
	}

	fmt.Fprintf(w, "<h2>Jobs</h2>")
	{
		var values [][]string
		for _, job := range jobs {
			values = append(values, []string{
				string(job.ID),
				string(job.State),
				job.Title,
				job.TargetRunnerID,
				job.TargetRunnerArch,
				job.Updated.UTC().Format(time.RFC3339),
				job.Created.UTC().Format(time.RFC3339),
			})
		}
		tableStyle(w)
		table(w, []string{"id", "state", "title", "target runner ID", "target runner arch", "last updated", "created"}, values)
	}
	fmt.Fprintf(w, "<h2>Finished jobs</h2>")
	{
		var values [][]string
		for _, job := range finishedJobs {
			values = append(values, []string{
				string(job.ID),
				string(job.State),
				job.Title,
				job.TargetRunnerID,
				job.TargetRunnerArch,
				job.Updated.UTC().Format(time.RFC3339),
				job.Created.UTC().Format(time.RFC3339),
			})
		}
		tableStyle(w)
		table(w, []string{"id", "state", "title", "target runner ID", "target runner arch", "last updated", "created"}, values)
	}
	return nil
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
	err := b.store.RunnerSeen(ctx, r.ID, r.Arch)
	if err != nil {
		return nil, errors.Wrap(err, "RunnerSeen")
	}

	if r.Job != nil {
		// Update job state.
		job, err := b.store.JobByID(ctx, r.Job.ID)
		if err != nil {
			return nil, errors.Wrap(err, "JobsByID")
		}
		job.State = r.Job.State
		err = b.store.UpsertRunnerJob(ctx, *job)
		if err != nil {
			return nil, errors.Wrap(err, "UpsertRunnerJob(0)")
		}

		// Log job messages.
		if r.Job.Log != "" {
			b.idLogf(r.Job.ID.LogID(), "%s", r.Job.Log)
		}
	}

	startNewJob := r.Job == nil ||
		r.Job.State == api.JobStateSuccess ||
		r.Job.State == api.JobStateError
	if startNewJob {
		b.jobAcquire.Lock()
		defer b.jobAcquire.Unlock()

		readyJobs, err := b.store.Jobs(ctx, JobsFilter{State: api.JobStateReady})
		if err != nil {
			return nil, errors.Wrap(err, "Jobs")
		}
		for _, job := range readyJobs {
			archMatch := job.TargetRunnerArch == "" || job.TargetRunnerArch == r.Arch
			idMatch := job.TargetRunnerID == "" || job.TargetRunnerID == r.ID
			if archMatch && idMatch {
				job.State = api.JobStateStarting
				err = b.store.UpsertRunnerJob(ctx, job)
				if err != nil {
					return nil, errors.Wrap(err, "UpsertRunnerJob(1)")
				}
				return &api.RunnerPollResponse{Start: &api.RunnerJobStart{
					ID:                 job.ID,
					Title:              job.Title,
					Payload:            job.Payload,
					GitPushUsername:    b.Config.GitPushUsername,
					GitPushPassword:    b.Config.GitPushPassword,
					GitConfigUserName:  b.Config.GitConfigUserName,
					GitConfigUserEmail: b.Config.GitConfigUserEmail,
				}}, nil
			}
		}
	}
	return &api.RunnerPollResponse{}, nil
}

func (b *Bot) httpServeRunnerList(ctx context.Context, r *api.RunnerListRequest) (*api.RunnerListResponse, error) {
	runners, err := b.store.Runners(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "RunnerSeen")
	}
	return &api.RunnerListResponse{Runners: runners}, nil
}
