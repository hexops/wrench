package wrench

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"github.com/hexops/wrench/internal/errors"
	"golang.org/x/crypto/acme/autocert"
)

func (b *Bot) httpStart() error {
	if b.Config.Address == "" {
		b.logf("http: disabled (Config.Address not configured)")
		return nil
	}

	handler := func(prefix string, handle func(w http.ResponseWriter, r *http.Request) error) http.Handler {
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
	mux.Handle("/logs/", handler("logs", b.httpServeLogs))

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
		return server.ListenAndServeTLS("", "")
	}
	return http.ListenAndServe(b.Config.Address, mux)
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

	b.webHookGitHubSelf.Lock()
	defer b.webHookGitHubSelf.Unlock()

	b.idLogf("restart-self", "👀 I see new changes")
	b.runScript("restart-self", `
#!/usr/bin/env bash
set -exuo pipefail
	
git clone https://github.com/hexops/wrench
cd wrench/
go build -o wrench .
sudo mv wrench /usr/local/bin/wrench
`)

	b.idLogf("restart-self", "build success! restarting..")

	return b.runScript("restart-self", `
#!/usr/bin/env bash
set -exuo pipefail
	
sudo systemctl restart wrench
`)
}

func (b *Bot) runScript(id string, script string) error {
	file, err := os.CreateTemp("wrench", "script-"+id)
	if err != nil {
		return errors.Wrap(err, "CreateTemp")
	}
	defer os.Remove(file.Name())

	err = os.WriteFile(file.Name(), []byte(script), 0744)
	if err != nil {
		return errors.Wrap(err, "WriteFile")
	}

	w := b.idWriter(id)
	cmd := exec.Command("/usr/bin/env", "bash", file.Name())
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
		id = "general"
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
