package main

import (
	"flag"
	"fmt"

	"github.com/hexops/cmder"
)

// gitCommands contains all registered 'wrench git' subcommands.
var gitCommands cmder.Commander

var gitFlagSet = flag.NewFlagSet("git", flag.ExitOnError)

func init() {
	const usage = `wrench git: manage git repositories

Usage:

	wrench git <command> [arguments]

The commands are:

	clone        clone all repositories
	status       status all repositories
	commitpush   commit and push all repositories
	resethard    git reset --hard all repositories

Use "wrench git <command> -h" for more information about a command.
`

	usageFunc := func() {
		fmt.Printf("%s", usage)
	}
	gitFlagSet.Usage = usageFunc

	// Handles calls to our subcommand.
	handler := func(args []string) error {
		_ = gitFlagSet.Parse(args)
		gitCommands.Run(gitFlagSet, "wrench git", usage, args)
		return nil
	}

	// Register the command.
	commands = append(commands, &cmder.Command{
		FlagSet:   gitFlagSet,
		Aliases:   []string{},
		Handler:   handler,
		UsageFunc: usageFunc,
	})
}
