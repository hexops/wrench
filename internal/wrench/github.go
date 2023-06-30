package wrench

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v48/github"
	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/wrench/scripts"
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
		for {
			b.sync(ctx)
			time.Sleep(5 * time.Minute)
		}
	}()
	return nil
}

func (b *Bot) sync(ctx context.Context) {
	logID := "github-sync"

	b.idLogf(logID, "github sync: starting")
	defer b.idLogf(logID, "github sync: finished")
	var wg sync.WaitGroup
	for _, repo := range scripts.AllRepos {
		repoPair := repo.Name
		wg.Add(1)
		go func() {
			defer wg.Done()
			org, repo := splitRepoPair(repoPair)
			page := 0
			retry := 0

			var pullRequests []*github.PullRequest
			for {
				pagePRs, resp, err := b.github.PullRequests.List(ctx, org, repo, &github.PullRequestListOptions{
					State: "all",
					ListOptions: github.ListOptions{
						Page:    page,
						PerPage: 1000,
					},
				})
				if err != nil {
					retry++
					b.idLogf(logID, "%s/%s: error: %v (retry %v of 5)", org, repo, err, retry)
					if retry >= 5 {
						break
					}
					time.Sleep(5 * time.Second)
					continue
				}
				pullRequests = append(pullRequests, pagePRs...)
				b.idLogf(logID, "%s/%s: progress: queried %v pull requests total (rate limit %v/%v)", org, repo, len(pullRequests), resp.Rate.Remaining, resp.Rate.Limit)

				page = resp.NextPage
				if resp.NextPage == 0 {
					break
				}
			}
			if err := b.githubUpdatePullRequestsCache(ctx, repoPair, pullRequests); err != nil {
				b.idLogf(logID, "error: githubUpdatePullRequests: %v", err)
				return
			}

			// Cache combined repository status
			combinedStatus, _, err := b.github.Repositories.GetCombinedStatus(ctx, org, repo, "HEAD", nil)
			if err != nil {
				b.idLogf(logID, "%s/%s: error: %v (fetching combined status)", org, repo, err)
				return
			}
			cacheKey := repoPair + "-Repositories-GetCombinedStatus-HEAD"
			cacheValue, err := json.Marshal(combinedStatus)
			if err != nil {
				b.idLogf(logID, "error: Marshal: %v", err)
				return
			}
			err = b.store.CacheSet(ctx, githubAPICacheName, cacheKey, string(cacheValue), nil)
			if err != nil {
				b.idLogf(logID, "error: CacheSet: %v", err)
				return
			}

			// Cache check runs for HEAD (CI status check)
			checkRuns, _, err := b.github.Checks.ListCheckRunsForRef(ctx, org, repo, "HEAD", nil)
			if err != nil {
				b.idLogf(logID, "%s/%s: error: %v (fetching check runs)", org, repo, err)
				return
			}
			cacheKey = repoPair + "-Checks-ListCheckRunsForRef-HEAD"
			cacheValue, err = json.Marshal(checkRuns)
			if err != nil {
				b.idLogf(logID, "error: Marshal: %v", err)
				return
			}
			err = b.store.CacheSet(ctx, githubAPICacheName, cacheKey, string(cacheValue), nil)
			if err != nil {
				b.idLogf(logID, "error: CacheSet: %v", err)
				return
			}
		}()
	}
	wg.Wait()
}

func (b *Bot) githubUpdatePRNow(ctx context.Context, repoPair string, updated *github.PullRequest) {
	if err := b.githubUpdatePRNowFallible(ctx, repoPair, updated); err != nil {
		b.idLogf("github", "githubUpdatePRNow: %v", err)
	}
}

func (b *Bot) githubUpdatePRNowFallible(ctx context.Context, repoPair string, updated *github.PullRequest) error {
	pullRequests, err := b.githubPullRequests(ctx, repoPair)
	if err != nil {
		return errors.Wrap(err, "githubPullRequests")
	}
	found := false
	for i, pr := range pullRequests {
		if *pr.Number != *updated.Number {
			continue
		}
		pullRequests[i] = updated
		found = true
		break
	}
	if !found {
		pullRequests = append(pullRequests, updated)
	}
	return b.githubUpdatePullRequestsCache(ctx, repoPair, pullRequests)
}

func isGitHubRateLimit(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "rate limit exceeded")
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

func (b *Bot) githubUpdatePullRequestsCache(ctx context.Context, repoPair string, pullRequests []*github.PullRequest) error {
	cacheKey := repoPair + "-PullRequests"
	cacheValue, err := json.Marshal(pullRequests)
	if err != nil {
		return errors.Wrap(err, "Marshal")
	}
	err = b.store.CacheSet(ctx, githubAPICacheName, cacheKey, string(cacheValue), nil)
	if err != nil {
		return errors.Wrap(err, "CacheSet")
	}
	return nil
}

//nolint:unused
func (b *Bot) githubCombinedStatusHEAD(ctx context.Context, repoPair string) (v *github.CombinedStatus, err error) {
	cacheKey := repoPair + "-Repositories-GetCombinedStatus-HEAD"
	entry, err := b.store.CacheKey(ctx, githubAPICacheName, cacheKey)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(entry.Value), &v); err != nil {
		return nil, errors.Wrap(err, "Unmarshal")
	}
	return v, nil
}

func (b *Bot) githubCheckRunsHEAD(ctx context.Context, repoPair string) (v *github.ListCheckRunsResults, err error) {
	cacheKey := repoPair + "-Checks-ListCheckRunsForRef-HEAD"
	entry, err := b.store.CacheKey(ctx, githubAPICacheName, cacheKey)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(entry.Value), &v); err != nil {
		return nil, errors.Wrap(err, "Unmarshal")
	}
	return v, nil
}

func (b *Bot) githubUpsertPullRequest(ctx context.Context, repoPair string, pr *github.NewPullRequest) (*github.PullRequest, bool, error) {
	pullRequests, err := b.githubPullRequests(ctx, repoPair)
	if err != nil {
		return nil, false, errors.Wrap(err, "githubPullRequests")
	}
	var exists *github.PullRequest
	for _, existing := range pullRequests {
		// TODO: don't hard-code wrench user here
		wrenchGitHubUsername := "wrench-bot"
		if *existing.State == "open" &&
			*existing.Title == *pr.Title &&
			*existing.Head.Ref == *pr.Head &&
			*existing.User.Login == wrenchGitHubUsername {
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
		if err != nil {
			errClosed := strings.Contains(err.Error(), "Cannot change the base branch of a closed pull request")
			if errClosed {
				goto createNewPR
			}
		}
		return exists, false, errors.Wrap(err, "PullRequests.Edit")
	}

	// Create a new PR.
createNewPR:
	newPR, _, err := b.github.PullRequests.Create(ctx, org, repo, pr)
	return newPR, true, errors.Wrap(err, "PullRequests.Create")
}

func (b *Bot) githubStop() error {
	return nil
}

func splitRepoPair(repoPair string) (owner, name string) {
	split := strings.Split(repoPair, "/")
	return split[0], split[1]
}

func repoPairFromURL(remoteURL string) string {
	remoteURL = strings.TrimPrefix(remoteURL, "https://")
	remoteURL = strings.TrimPrefix(remoteURL, "http://")
	remoteURL = strings.TrimPrefix(remoteURL, "github.com/")
	return remoteURL
}

const githubAPICacheName = "github-api"
