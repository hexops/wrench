package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/hexops/cmder"
	"github.com/hexops/wrench/internal/wrench/scripts"
)

func init() {
	const usage = `
Examples:

  Git commit & push all repositories:

    $ wrench git commitpush

`

	// Parse flags for our subcommand.
	flagSet := flag.NewFlagSet("commitpush", flag.ExitOnError)
	excludeSet := flagSet.String("exclude", "", "comma separated list of repositories to exclude")
	accept := flagSet.Bool("accept", false, "Actually run the commands (do not run in dry-run mode)")

	// Handles calls to our subcommand.
	handler := func(args []string) error {
		_ = flagSet.Parse(args)

		commitMessage := flagSet.Arg(0)

		excludedRepos := map[string]struct{}{}
		for _, excluded := range strings.FieldsFunc(*excludeSet, func(r rune) bool {
			return r == ','
		}) {
			excludedRepos[excluded] = struct{}{}
		}

		for _, repo := range scripts.AllRepos {
			repoName := strings.Split(repo.Name, "/")[1]
			if _, err := os.Stat(repoName); err != nil {
				continue
			}
			if _, excluded := excludedRepos[repoName]; excluded {
				continue
			}
			if !*accept {
				fmt.Printf("$ cd %s/ && git add . && git commit -s -m '%s' && git push\n", repoName, commitMessage)
			} else {
				_ = scripts.ExecArgs("git", []string{"pull"}, scripts.WorkDir(repoName))(os.Stderr)
				_ = scripts.ExecArgs("git", []string{"add", "."}, scripts.WorkDir(repoName))(os.Stderr)
				_ = scripts.ExecArgs("git", []string{"commit", "-s", "-m", commitMessage}, scripts.WorkDir(repoName))(os.Stderr)
				_ = scripts.ExecArgs("git", []string{"push"}, scripts.WorkDir(repoName))(os.Stderr)
			}
		}
		return nil
	}

	// Register the command.
	gitCommands = append(gitCommands, &cmder.Command{
		FlagSet: flagSet,
		Aliases: []string{},
		Handler: handler,
		UsageFunc: func() {
			fmt.Fprintf(flag.CommandLine.Output(), "Usage of 'wrench git status %s':\n", flagSet.Name())
			flagSet.PrintDefaults()
			fmt.Printf("%s", usage)
		},
	})
}
