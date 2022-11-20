package main

import (
	"flag"
	"fmt"

	"github.com/hexops/cmder"
)

func init() {
	const usage = `
Examples:

  Start performing work:

    $ wrench start

`

	// Parse flags for our subcommand.
	flagSet := flag.NewFlagSet("start", flag.ExitOnError)

	// Handles calls to our subcommand.
	handler := func(args []string) error {
		_ = flagSet.Parse(args)
		fmt.Println("sorry, I can't do anything right now. Maybe some day.")
		return nil
	}

	// Register the command.
	commands = append(commands, &cmder.Command{
		FlagSet: flagSet,
		Aliases: []string{},
		Handler: handler,
		UsageFunc: func() {
			fmt.Fprintf(flag.CommandLine.Output(), "Usage of 'wrench %s':\n", flagSet.Name())
			flagSet.PrintDefaults()
			fmt.Println(usage)
		},
	})
}
