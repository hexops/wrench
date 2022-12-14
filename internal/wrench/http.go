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
	"os"
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

	b.idLogf("restart-self", "👀 I see new changes")
	err := b.runScript("restart-self", `
#!/usr/bin/env bash
set -exuo pipefail

git clone https://github.com/hexops/wrench || true
cd wrench/
git fetch
git reset --hard origin/main

DATE=$(date)
GOVERSION=$(go version)
VERSION=$(git describe --tags --abbrev=8 --dirty --always --long)
COMMIT_TITLE=$(git log --pretty=format:%s HEAD^1..HEAD)
PREFIX="github.com/hexops/wrench/internal/wrench"
LDFLAGS="-X '$PREFIX.Version=$VERSION'"
LDFLAGS="$LDFLAGS -X '$PREFIX.CommitTitle=$COMMIT_TITLE'"
LDFLAGS="$LDFLAGS -X '$PREFIX.Date=$DATE'"
LDFLAGS="$LDFLAGS -X '$PREFIX.GoVersion=$GOVERSION'"
GOARCH="amd64" GOOS="linux" go build -ldflags "$LDFLAGS" -o bin/wrench .

sudo mv bin/wrench /usr/local/bin/wrench
`)
	if err != nil {
		b.discord("Oops, looks like I can't build myself? Logs: " + b.Config.ExternalURL + "/logs/restart-self")
		b.idLogf("restart-self", "build failure!")
		return nil
	}

	b.idLogf("restart-self", "build success! restarting..")

	return b.runScript("restart-self", `
#!/usr/bin/env bash
set -exuo pipefail

sudo wrench svc restart
`)
}

func (b *Bot) runScript(id string, script string) error {
	file, err := os.CreateTemp("", "script-"+id)
	if err != nil {
		return errors.Wrap(err, "CreateTemp")
	}
	defer os.Remove(file.Name())

	err = os.WriteFile(file.Name(), []byte(script), 0744)
	if err != nil {
		return errors.Wrap(err, "WriteFile")
	}

	w := b.idWriter(id)
	cmd := exec.Command("bash", file.Name())
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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
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
	fmt.Fprintf(w, `<table>`)
	fmt.Fprintf(w, `<thead><tr>`)
	fmt.Fprintf(w, `<th>id</th><th>arch</th><th>registered</th><th>last seen</th>`)
	fmt.Fprintf(w, `</tr></thead>`)
	fmt.Fprintf(w, `<tbody>`)
	for _, runner := range runners {
		fmt.Fprintf(w, `<tr>`)
		fmt.Fprintf(w, `<td>%s</td><td>%s</td><td>%s</td><td>%s</td>`,
			runner.ID,
			runner.Arch,
			runner.RegisteredAt.UTC().Format(time.RFC3339),
			runner.LastSeenAt.UTC().Format(time.RFC3339),
		)
		fmt.Fprintf(w, `</tr>`)
	}
	fmt.Fprintf(w, `</tbody></table>`)
	return nil
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
	return &api.RunnerPollResponse{}, nil
}

func (b *Bot) httpServeRunnerList(ctx context.Context, r *api.RunnerListRequest) (*api.RunnerListResponse, error) {
	runners, err := b.store.Runners(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "RunnerSeen")
	}
	return &api.RunnerListResponse{Runners: runners}, nil
}
