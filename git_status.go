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

  Git status all repositories:

    $ wrench git status

`

	// Parse flags for our subcommand.
	flagSet := flag.NewFlagSet("status", flag.ExitOnError)

	// Handles calls to our subcommand.
	handler := func(args []string) error {
		_ = flagSet.Parse(args)

		for _, repo := range scripts.AllRepos {
			repoName := strings.Split(repo.Name, "/")[1]
			if _, err := os.Stat(repoName); err != nil {
				continue
			}
			err := scripts.ExecArgs("git", []string{"status", "-s"}, scripts.WorkDir(repoName))(os.Stderr)
			if err != nil {
				return err
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
