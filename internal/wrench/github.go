package wrench

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/go-github/v48/github"
	"github.com/hexops/wrench/internal/errors"
	"golang.org/x/oauth2"
)

func (b *Bot) githubStart() error {
	if b.Config.GitHubAccessToken == "" {
		return nil
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: b.Config.GitHubAccessToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	b.github = github.NewClient(tc)

	go func() {
		b.sync(ctx)
		time.Sleep(5 * time.Minute)
	}()
	return nil
}

// TODO: move to config?
var githubRepoNames = []string{
	"hexops/wrench",
	"hexops/mach",
	"hexops/mach-examples",
	"hexops/sdk-linux-aarch64",
	"hexops/sdk-linux-x86_64",
	"hexops/sdk-windows-x86_64",
	"hexops/sdk-macos-12.0",
	"hexops/sdk-macos-11.3",
	"hexops/mach-glfw-opengl-example",
	"hexops/mach-glfw-vulkan-example",
	"hexops/basisu",
	// "soundio",
	"hexops/freetype",
	"hexops/glfw",
	"hexops/dawn",
	"hexops/hexops.com",
	"hexops/zigmonthly.org",
	"hexops/devlog",
	"hexops/machengine.org",
	"hexops/media",
}

func (b *Bot) sync(ctx context.Context) {
	logID := "github-sync"

	b.idLogf(logID, "github sync: starting")
	defer b.idLogf(logID, "github sync: finished")
	for _, repoPair := range githubRepoNames {
		org, repo := splitRepoPair(repoPair)
		page := 0
		retry := 0
		var pullRequests []*github.PullRequest
		for {
			b.idLogf(logID, "synchronizing: %s/%s", org, repo)
			pagePRs, resp, err := b.github.PullRequests.List(ctx, org, repo, &github.PullRequestListOptions{
				State: "all",
				ListOptions: github.ListOptions{
					Page:    page,
					PerPage: 1000,
				},
			})
			if err != nil {
				retry++
				b.idLogf(logID, "error: %v (retry %v of 5)", err, retry)
				if retry >= 5 {
					break
				}
				time.Sleep(5 * time.Second)
				continue
			}
			pullRequests = append(pullRequests, pagePRs...)
			b.idLogf(logID, "progress: queried %v pull requests total", len(pullRequests))
			b.idLogf(logID, "progress: rate limit: %v", resp.Rate)

			page = resp.NextPage
			if resp.NextPage == 0 {
				break
			}
		}
		cacheKey := repoPair + "-PullRequests"
		cacheValue, err := json.Marshal(pullRequests)
		if err != nil {
			b.idLogf(logID, "error: Marshal: %v", err)
			continue
		}
		b.store.CacheSet(ctx, githubAPICacheName, cacheKey, string(cacheValue), nil)
	}
}

func (b *Bot) githubPullRequests(ctx context.Context, repoPair string) (v []*github.PullRequest, err error) {
	cacheKey := repoPair + "-PullRequests"
	entry, err := b.store.CacheKey(ctx, githubAPICacheName, cacheKey)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(entry.Value), &v); err != nil {
		return nil, errors.Wrap(err, "Unmarshal")
	}
	return v, nil
}

func (b *Bot) githubUpsertPullRequest(ctx context.Context, repoPair string, pr *github.NewPullRequest) error {
	pullRequests, err := b.githubPullRequests(ctx, repoPair)
	if err != nil {
		return errors.Wrap(err, "githubPullRequests")
	}
	var exists *github.PullRequest
	for _, existing := range pullRequests {
		// TODO: don't hard-code wrench user here
		wrenchGitHubUsername := "wrench-bot"
		if *existing.State == "open" && *existing.Title == *pr.Title && *existing.User.Login == wrenchGitHubUsername {
			exists = existing
		}
	}

	org, repo := splitRepoPair(repoPair)
	if exists != nil {
		// Update the existing PR.
		*exists.Title = *pr.Title
		*exists.Head.Ref = *pr.Head
		*exists.Base.Ref = *pr.Base
		*exists.Body = *pr.Body
		*exists.Draft = *pr.Draft
		_, _, err := b.github.PullRequests.Edit(ctx, org, repo, *exists.Number, exists)
		return errors.Wrap(err, "PullRequests.Edit")
	}

	// Create a new PR.
	_, _, err = b.github.PullRequests.Create(ctx, org, repo, pr)
	return errors.Wrap(err, "PullRequests.Create")
}

func (b *Bot) githubStop() error {
	return nil
}

func splitRepoPair(repoPair string) (owner, name string) {
	split := strings.Split(repoPair, "/")
	return split[0], split[1]
}

const githubAPICacheName = "github-api"
