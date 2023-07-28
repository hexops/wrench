package wrench

import (
	"bytes"
	"context"
	"crypto/subtle"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	"github.com/google/go-github/v48/github"
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

	var mux http.Handler
	if b.Config.PkgProxy {
		b.logf("http: PkgProxy mode enabled")
		mux = b.httpMuxPkgProxy(handler)
	} else {
		mux = b.httpMuxDefault(handler)
	}

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

		go func() {
			err := http.ListenAndServe(":http", certManager.HTTPHandler(nil))
			if err != nil {
				log.Fatal("ListenAndServe:", err)
			}
		}()

		// Key and cert are provided by LetsEncrypt
		go func() {
			err := server.ListenAndServeTLS("", "")
			if err != nil {
				log.Fatal("ListenAndServeTLS:", err)
			}
		}()
		return nil
	}
	go func() {
		err := http.ListenAndServe(b.Config.Address, mux)
		if err != nil {
			log.Fatal("ListenAndServe(addr):", err)
		}
	}()
	return nil
}

func (b *Bot) httpMuxDefault(handler func(prefix string, handle handlerFunc) http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		logo := "https://raw.githubusercontent.com/hexops/media/b71e82ae9ea20c22a2eb3ab95d8ba48684635620/mach/wrench_rocket.svg"
		fmt.Fprintf(w, `<div style="padding: 1rem;">`)
		fmt.Fprintf(w, `<h1>[bot] wrench: let's fix this!</h1>`)
		fmt.Fprintf(w, `<div style="display: flex; align-items: center; width: 50rem;">`)
		{
			fmt.Fprintf(w, `<img width="250px" align="left" src="%s">`, logo)
			fmt.Fprintf(w, `<div style="padding-left: 2rem;"><em><strong>Wrench</strong> here!</em> I'm the mascot of <a href="https://machengine.org">Mach engine</a>, and I help automate and maintain our projects. You can read about me in <em><a href="https://devlog.hexops.com/2023/how-wrench-helps-build-mach/">"Wrench helps automate and maintain Mach (the Zig game engine)"</a></em> or view my code <a href="https://github.com/hexops/wrench">on GitHub</a>!</div>`)
		}
		fmt.Fprintf(w, `</div>`)

		fmt.Fprintf(w, `<h1>Where you can find me</h1>`)
		fmt.Fprintf(w, `<ul>`)
		{
			fmt.Fprintf(w, `<li>On this website</li>`)
			fmt.Fprintf(w, `<li>In the <a href="https://discord.gg/XNG3NZgCqp">Mach discord</a></li>`)
			fmt.Fprintf(w, `<li>Making contributions <a href="https://github.com/wrench-bot">on GitHub</a> such as <a href="https://github.com/hexops/mach/pull/760">updating the version of Zig we use</a>, <a href="https://github.com/hexops/mach/pull/697">keeping our version of Dawn/WebGPU up-to-date</a>, and more.</li>`)
		}
		fmt.Fprintf(w, `</ul>`)

		fmt.Fprintf(w, `<h1>Explore</h1>`)
		fmt.Fprintf(w, `<ul>`)
		{
			fmt.Fprintf(w, `<li><a href="%s/projects">Projects overview</a></li>`, b.Config.ExternalURL)
			fmt.Fprintf(w, `<li><a href="%s/pull-requests">Pull requests</a></li>`, b.Config.ExternalURL)
			fmt.Fprintf(w, `<li><a href="%s/runners">Runners & jobs</a></li>`, b.Config.ExternalURL)
			fmt.Fprintf(w, `<li><a href="%s/logs">Job logs</a></li>`, b.Config.ExternalURL)
			fmt.Fprintf(w, `<li><a href="%s/rebuild">Trigger a rebuild of wrench.machengine.org (admin-only)</a></li>`, b.Config.ExternalURL)
		}
		fmt.Fprintf(w, `</ul>`)

		fmt.Fprintf(w, `<h2>Discord integration</h2>`)
		fmt.Fprintf(w, `<p>In the <a href="https://discord.gg/XNG3NZgCqp">Mach discord</a> join <code>#wrench</code> to see what I'm up to!</p>`)
		fmt.Fprintf(w, `<p>Type <code>!wrench</code> in the <code>#spam</code> channel or when direct messaging me for help.</p>`)
		fmt.Fprintf(w, `</div>`)
	})
	mux.Handle("/webhook/github", handler("webhook", b.httpServeWebHookGitHub))
	mux.Handle("/rebuild", handler("rebuild", b.httpBasicAuthMiddleware(b.httpServeRebuild)))
	mux.Handle("/logs/", handler("logs", b.httpServeLogs))
	mux.Handle("/runners/", handler("runners", b.httpServeRunners))
	mux.Handle("/pull-requests/", handler("pull-requests", b.httpServePullRequests))
	mux.Handle("/projects/", handler("projects", b.httpServeProjects))
	mux.Handle("/api/runner/poll", handler("api-runner-poll", botHttpAPI(b, b.httpServeRunnerPoll)))
	mux.Handle("/api/runner/job-update", handler("api-runner-job-update", botHttpAPI(b, b.httpServeRunnerJobUpdate)))
	mux.Handle("/api/runner/list", handler("api-runner-list", botHttpAPI(b, b.httpServeRunnerList)))
	mux.Handle("/api/secrets/list", handler("api-secrets-list", botHttpAPI(b, b.httpServeSecretsList)))
	mux.Handle("/api/secrets/delete", handler("api-secrets-delete", botHttpAPI(b, b.httpServeSecretsDelete)))
	mux.Handle("/api/secrets/upsert", handler("api-secrets-upsert", botHttpAPI(b, b.httpServeSecretsUpsert)))
	return mux
}

func (b *Bot) httpStop() error {
	if b.discordSession == nil {
		return nil
	}
	return b.discordSession.Close()
}

func (b *Bot) httpServeWebHookGitHub(w http.ResponseWriter, r *http.Request) error {
	if b.Config.GitHubWebHookSecret == "" {
		b.logf("http: webhook: ignored: /webhook/github (config.GitHubWebHookSecret not set)")
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

	switch ev := event.(type) {
	case *github.PushEvent:
		if scripts.IsPrivateRepo(ev.Repo.GetFullName()) {
			return nil
		}
		if ev.Repo.GetFullName() == "hexops/dawn" {
			// Do not post Discord messages for hexops/dawn commits (which occur as part of automated
			// updates)
			return nil
		}
		if err := b.discordGitHubPushEvent(ev); err != nil {
			b.logf("http: discordGitHubPushEvent: %v", err)
		}
		if ev.Repo.GetFullName() == "hexops/wrench" {
			return b.runRebuild()
		}
		return nil
	case *github.PullRequestEvent:
		if scripts.IsPrivateRepo(ev.Repo.GetFullName()) {
			return nil
		}
		if err := b.discordGitHubPullRequestEvent(ev); err != nil {
			b.logf("http: discordGitHubPullRequestEvent: %v", err)
		}
		b.githubUpdatePRNow(r.Context(), ev.Repo.GetFullName(), ev.PullRequest)
		return nil
	case *github.IssuesEvent:
		if scripts.IsPrivateRepo(ev.Repo.GetFullName()) {
			return nil
		}
		if err := b.discordGitHubIssuesEvent(ev); err != nil {
			b.logf("http: discordGitHubIssuesEvent: %v", err)
		}
		b.githubUpdateIssueNow(r.Context(), ev.Repo.GetFullName(), ev.Issue)
		return nil
	default:
		return nil
	}
}

func commitTitle(msg string) string {
	split := strings.Split(msg, "\n")
	if len(split) == 0 {
		return ""
	}
	return split[0]
}

func ellipsis(s string, max int) string {
	if len(s) > max-1 {
		return s[:max-1] + "…"
	}
	return s
}

func (b *Bot) discordGitHubPushEvent(ev *github.PushEvent) error {
	if *ev.Ref != "refs/heads/main" && *ev.Ref != "refs/heads/master" {
		return nil
	}
	var out bytes.Buffer
	numCommits := 0
	for _, commit := range ev.Commits {
		if commit.Author.GetLogin() == "wrench-bot" {
			continue
		}
		numCommits++
		fmt.Fprintf(&out, "[[%s](<%s>)] [%s](<%s>): %s (@%s)\n",
			*ev.Repo.FullName,
			*ev.Repo.HTMLURL,
			commit.GetID()[:7],
			*commit.URL,
			ellipsis(commitTitle(commit.GetMessage()), 60),
			commit.Author.GetLogin(),
		)
	}
	if numCommits == 0 {
		return nil
	}
	return b.discordSendMessageToChannel("github", out.String())
}

func (b *Bot) discordGitHubPullRequestEvent(ev *github.PullRequestEvent) error {
	if ev.GetAction() != "opened" {
		return nil
	}
	author := ev.PullRequest.User.GetLogin()
	if author == "wrench-bot" {
		return nil
	}

	embed := &discordgo.MessageEmbed{
		Color:       3134534,
		Description: *ev.PullRequest.Title,
		Author: &discordgo.MessageEmbedAuthor{
			Name:    fmt.Sprintf("[%s] @%s created PR #%v", *ev.Repo.FullName, author, *ev.PullRequest.Number),
			URL:     *ev.PullRequest.HTMLURL,
			IconURL: *ev.PullRequest.User.AvatarURL,
		},
	}
	return b.discordSendMessageToChannelEmbeds("github", []*discordgo.MessageEmbed{
		embed,
	})
}

func (b *Bot) discordGitHubIssuesEvent(ev *github.IssuesEvent) error {
	if ev.GetAction() != "opened" {
		return nil
	}
	author := ev.Issue.User.GetLogin()
	if author == "wrench-bot" {
		return nil
	}

	embed := &discordgo.MessageEmbed{
		Color:       3134534,
		Description: *ev.Issue.Title,
		Author: &discordgo.MessageEmbedAuthor{
			Name:    fmt.Sprintf("[%s] @%s created issue #%v", *ev.Repo.FullName, author, *ev.Issue.Number),
			URL:     *ev.Issue.HTMLURL,
			IconURL: *ev.Issue.User.AvatarURL,
		},
	}
	return b.discordSendMessageToChannelEmbeds("github", []*discordgo.MessageEmbed{
		embed,
	})
}

func (b *Bot) httpServeRebuild(w http.ResponseWriter, r *http.Request) error {
	return b.runRebuild()
}

func (b *Bot) runRebuild() error {
	b.rebuildSelfMu.Lock()
	defer b.rebuildSelfMu.Unlock()

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
		JobsFilter{Limit: 50},
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

func (b *Bot) httpServePullRequests(w http.ResponseWriter, r *http.Request) error {
	prList := func(label, state string, draft, filterDraft bool) error {
		fmt.Fprintf(w, "<h2>Pull requests (%s)</h2>", label)
		var values [][]string
		renderPRs := func(wrench bool) error {
			for _, repo := range scripts.AllRepos {
				repoPair := repo.Name
				pullRequests, err := b.githubPullRequests(r.Context(), repoPair)
				if err != nil {
					return err
				}
				for _, pr := range pullRequests {
					if wrench != (*pr.User.Login == "wrench-bot") {
						continue
					}
					if *pr.State != state {
						continue
					}
					if filterDraft && draft != *pr.Draft {
						continue
					}
					values = append(values, []string{
						fmt.Sprintf(`<a href="https://github.com/%s/pulls">%s</a>`, repoPair, strings.TrimPrefix(repoPair, "hexops/")),
						fmt.Sprintf(`<a href="%s">%s</a>`, *pr.HTMLURL, html.EscapeString(*pr.Title)),
						fmt.Sprintf(`<a href="%s">%s</a>`, *pr.User.HTMLURL, html.EscapeString(*pr.User.Login)),
						humanizeTimeRecent(*pr.CreatedAt),
					})
				}
			}
			return nil
		}
		if err := renderPRs(false); err != nil {
			return err
		}
		if err := renderPRs(true); err != nil {
			return err
		}
		tableStyle(w)
		table(w, []string{"repository", "title", "author", "created"}, values)
		return nil
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := prList("open", "open", false, true); err != nil {
		return err
	}
	if err := prList("draft", "open", true, true); err != nil {
		return err
	}
	if err := prList("closed", "closed", true, false); err != nil {
		return err
	}
	return nil
}

func (b *Bot) httpServeProjects(w http.ResponseWriter, r *http.Request) error {
	countPRs := func(repoPair, state string, draft, filterDraft bool) (int, error) {
		count := 0
		pullRequests, err := b.githubPullRequests(r.Context(), repoPair)
		if err != nil {
			return 0, err
		}
		for _, pr := range pullRequests {
			if *pr.State != state {
				continue
			}
			if filterDraft && draft != *pr.Draft {
				continue
			}
			count++
		}
		return count, nil
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fmt.Fprintf(w, `
<style>
.row {
	display: flex;
	justify-content: center;
	flex-wrap: wrap;
}
.row>span {
	font-weight: bold;
	font-size: 20px;
}
.row>div {
	display: flex;
	flex-direction: column;
	justify-content: space-around;
	align-items: center;
	width: 7rem;
	height: 7rem;
	outline: 1px solid black;
}
.row>div>span {
	padding: .25rem;
	font-weight: bold;
	word-break: break-word;
}
.row>div>ul {
	margin: 0;
	padding: 0;
}
</style>
`)

	detail := r.URL.Query().Has("detail")
	if !detail {
		fmt.Fprintf(w, `<div class="row">`)
	}
	for _, category := range scripts.AllReposByCategory {
		if detail {
			fmt.Fprintf(w, `<div class="row"><span>%s</span></div>`, category.Name)
			fmt.Fprintf(w, `<div class="row">`)
		}
		for _, repo := range category.Repos {
			repoPair := repo.Name
			if scripts.IsPrivateRepo(repoPair) {
				continue
			}
			numOpenPRs, err := countPRs(repoPair, "open", false, true)
			if err != nil {
				return err
			}
			numDraftPRs, err := countPRs(repoPair, "open", true, true)
			if err != nil {
				return err
			}
			numClosedPRs, err := countPRs(repoPair, "closed", true, false)
			if err != nil {
				return err
			}

			// Determine CI status
			checkRuns, err := b.githubCheckRunsHEAD(r.Context(), repoPair)
			if err != nil {
				return err
			}
			completed := 0
			pending := 0
			failure := false
			headSHA := ""
			for _, run := range checkRuns.CheckRuns {
				headSHA = *run.HeadSHA
				if *run.Status == "completed" {
					completed++
				}
				if *run.Status == "pending" || *run.Status == "in_progress" {
					pending++
				}
				if run.Conclusion != nil && *run.Conclusion == "failure" {
					failure = true
				}
			}

			reportNoCIAsGreen := repo.CI == scripts.None || repoPair == "hexops/machengine.org"
			statusColor := "#fff"
			status := "↻"
			if pending > 0 {
				status = "↻"
			} else if failure {
				status = "✖️"
				statusColor = "#ff5757"
			} else if *checkRuns.Total == 0 && !reportNoCIAsGreen {
				status = "∅"
				statusColor = "#d2d2d2"
			} else if completed > 0 || reportNoCIAsGreen {
				statusColor = "#0d0"
				status = "✓"
			}

			repoShortName := strings.TrimPrefix(repoPair, "hexops/")
			fontSize := "14px"
			if len(repoShortName) >= len("mach-freetype") {
				fontSize = "12px"
			}

			displayHidden := `display: none;`
			if detail {
				displayHidden = ""
			}
			fmt.Fprintf(w, `<div style="background: %s;">
	<span style="font-size: `+fontSize+`;">%s %s</span>
	<ul style="%s">
		<li>%s open</li>
		<li>%s drafts</li>
		<li>%s closed</li>
	</ul>
</div>`,
				statusColor,
				fmt.Sprintf(`<a href="https://github.com/%s">%s</a>`, repoPair, repoShortName),
				fmt.Sprintf(`<a href="https://github.com/%s/commit/%s">%v</a>`, repoPair, headSHA, status),
				displayHidden,
				fmt.Sprintf(`<a href="https://github.com/%s/pulls">%v</a>`, repoPair, numOpenPRs),
				fmt.Sprintf(`<a href="https://github.com/%s/pulls">%v</a>`, repoPair, numDraftPRs),
				fmt.Sprintf(`<a href="https://github.com/%s/pulls">%v</a>`, repoPair, numClosedPRs),
			)

		}
		if detail {
			fmt.Fprintf(w, `</div>`)
		}
	}
	if !detail {
		fmt.Fprintf(w, `</div>`)
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
			_, err := w.Write([]byte("Unauthorised.\n"))
			return err
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

	b.jobAcquire.Lock()
	defer b.jobAcquire.Unlock()

	runningSet := map[api.JobID]struct{}{}
	for _, running := range r.Running {
		runningSet[running] = struct{}{}
	}

	// Look for starting or running jobs that are assigned to our runner. We will check that the
	// runner reports they are running. If it does not, then they are dead jobs.
	maybeDeadJobs, err := b.store.Jobs(ctx,
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
	runningForegroundJobs := 0
	for _, job := range maybeDeadJobs {
		if _, isRunning := runningSet[job.ID]; isRunning {
			if !job.Payload.Background {
				runningForegroundJobs++
			}
			continue // job is running
		}
		// job is dead
		if len(job.Payload.Cmd) >= 2 && job.Payload.Cmd[0] == "script" && job.Payload.Cmd[1] == "rebuild" {
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
		JobsFilter{ScheduledStartLessOrEqualTo: time.Now()},
	)
	if err != nil {
		return nil, errors.Wrap(err, "Jobs(ready)")
	}
jobSearch:
	for _, job := range readyJobs {
		archMatch := job.TargetRunnerArch == "" || job.TargetRunnerArch == r.Arch
		idMatch := job.TargetRunnerID == "" || job.TargetRunnerID == r.ID
		wantForegroundJobs := runningForegroundJobs < 1
		foregroundMatch := job.Payload.Background || wantForegroundJobs

		if archMatch && idMatch && foregroundMatch {
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
				secrets[strings.TrimPrefix(secretID, r.ID+"/")] = secret.Value
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
				GitPushUsername:    stringIf(b.Config.GitPushUsername, job.Payload.GitPushBranchName != ""),
				GitPushPassword:    stringIf(b.Config.GitPushPassword, job.Payload.GitPushBranchName != ""),
				GitConfigUserName:  stringIf(b.Config.GitConfigUserName, job.Payload.GitPushBranchName != ""),
				GitConfigUserEmail: stringIf(b.Config.GitConfigUserEmail, job.Payload.GitPushBranchName != ""),
				Secrets:            secrets,
			}}, nil
		}
	}
	return &api.RunnerPollResponse{}, nil
}

func stringIf(s string, conditional bool) string {
	if conditional {
		return s
	}
	return ""
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

	if r.Job.State == api.JobStateSuccess && len(r.Job.Response.PushedRepos) > 0 {
		// Ensure pull requests exist.
		for _, repoRemoteURL := range r.Job.Response.PushedRepos {
			repoPair := repoPairFromURL(repoRemoteURL)
			prTemplate := job.Payload.PRTemplate.ToGitHub()

			if *prTemplate.Base == "main" {
				// Adjust to the actual default branch of the repository
				for _, repo := range scripts.AllRepos {
					if repoPair == repo.Name {
						if repo.Main != "" {
							prTemplate.Base = &repo.Main
						}
						break
					}
				}
			}
			replacements := map[string]string{
				"JOB_LOGS_URL": fmt.Sprintf("%s/logs/%s", b.Config.ExternalURL, r.Job.ID.LogID()),
			}
			for logName, logValue := range r.Job.Response.CustomLogs {
				b.idLogf(r.Job.ID.LogID()+"-"+logName, "%s", logValue)
				replacements["CUSTOM_LOG_"+uppercaseUnderscore(logName)] = fmt.Sprintf("%s/logs/%s-%s", b.Config.ExternalURL, r.Job.ID.LogID(), logName)
			}
			for metaName, metaValue := range r.Job.Response.Metadata {
				b.idLogf(r.Job.ID.LogID(), "metadata: %s = %s", metaName, metaValue)
				replacements["METADATA_"+uppercaseUnderscore(metaName)] = metaValue
			}
			for key, value := range replacements {
				*prTemplate.Body = strings.ReplaceAll(*prTemplate.Body, "${"+key+"}", value)
			}

			pr, isNew, err := b.githubUpsertPullRequest(ctx, repoPair, prTemplate)
			if err != nil {
				b.idLogf(r.Job.ID.LogID(), "error creating pull request: %v", err)
				if isGitHubRateLimit(err) {
					b.idLogf(r.Job.ID.LogID(), "GitHub rate limit encountered, waiting for 5 minutes")
					b.logf("GitHub rate limit encountered, waiting for 5 minutes")
					time.Sleep(5 * time.Minute)
				}
				return nil, errors.Wrap(err, "githubUpsertPullRequest")
			}
			b.idLogf(r.Job.ID.LogID(), "pull request: %s", *pr.HTMLURL)
			_ = isNew
			// if isNew {
			// 	b.discord("I sent a PR just now: %s", *pr.HTMLURL)
			// }
		}
	}

	if r.Job.State == api.JobStateSuccess && len(r.Job.Response.UpsertIssues) > 0 {
		// Ensure pull requests exist.
		for _, upsertIssue := range r.Job.Response.UpsertIssues {
			issueRequest := &github.IssueRequest{
				Title: &upsertIssue.Title,
				Body:  &upsertIssue.Body,
			}

			issue, isNew, err := b.githubUpsertIssue(ctx, upsertIssue.RepoPair, issueRequest)
			if err != nil {
				b.idLogf(r.Job.ID.LogID(), "error creating issue: %v", err)
				if isGitHubRateLimit(err) {
					b.idLogf(r.Job.ID.LogID(), "GitHub rate limit encountered, waiting for 5 minutes")
					b.logf("GitHub rate limit encountered, waiting for 5 minutes")
					time.Sleep(5 * time.Minute)
				}
				return nil, errors.Wrap(err, "githubUpsertIssue")
			}
			b.idLogf(r.Job.ID.LogID(), "issue: %s", *issue.HTMLURL)
			_ = isNew
			// if isNew {
			// 	b.discord("I created an issue just now: %s", *issue.HTMLURL)
			// }
		}
	}

	// Log job messages.
	if r.Job.Log != "" {
		if b.Config.GitPushUsername != "" {
			r.Job.Log = strings.ReplaceAll(r.Job.Log, b.Config.GitPushUsername, "<redacted>")
		}
		if b.Config.GitPushPassword != "" {
			r.Job.Log = strings.ReplaceAll(r.Job.Log, b.Config.GitPushPassword, "<redacted>")
		}
		if b.Config.GitConfigUserName != "" {
			r.Job.Log = strings.ReplaceAll(r.Job.Log, b.Config.GitConfigUserName, "<redacted>")
		}
		if b.Config.GitConfigUserEmail != "" {
			r.Job.Log = strings.ReplaceAll(r.Job.Log, b.Config.GitConfigUserEmail, "<redacted>")
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
