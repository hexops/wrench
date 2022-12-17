package wrench

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/go-github/v48/github"
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

func (b *Bot) sync(ctx context.Context) {
	// TODO: move to config?
	repos := []string{
		"wrench",
		"mach",
		"mach-examples",
		"sdk-linux-aarch64",
		"sdk-linux-x86_64",
		"sdk-windows-x86_64",
		"sdk-macos-12.0",
		"sdk-macos-11.3",
		"mach-glfw-opengl-example",
		"mach-glfw-vulkan-example",
		"basisu",
		// "soundio",
		"freetype",
		"glfw",
		"dawn",
		"hexops.com",
		"zigmonthly.org",
		"devlog",
		"machengine.org",
		"media",
	}
	org := "hexops"

	logID := "github-sync"
	cacheName := "github-api"

	b.idLogf(logID, "github sync: starting")
	defer b.idLogf(logID, "github sync: finished")
	for _, repo := range repos {
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

			page = resp.NextPage
			if resp.NextPage == 0 {
				break
			}
		}
		cacheKey := fmt.Sprintf("%s/%s-PullRequests", org, repo)
		cacheValue, err := json.Marshal(pullRequests)
		if err != nil {
			b.idLogf(logID, "error: Marshal: %v", err)
		}
		b.store.CacheSet(ctx, cacheName, cacheKey, string(cacheValue), nil)
	}
}

func (b *Bot) githubStop() error {
	return nil
}
