package wrench

import (
	"context"
	"fmt"

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

	// list all repositories for the authenticated user
	repos, _, err := b.github.Repositories.List(ctx, "", nil)
	if err != nil {
		return errors.Wrap(err, "Repositories.List")
	}
	for _, repo := range repos {
		fmt.Println("found repo", repo.Name)
	}
	return nil
}

func (b *Bot) githubStop() error {
	return nil
}
