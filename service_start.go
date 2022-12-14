package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/hexops/cmder"
)

func init() {
	const usage = `
Examples:

  Start wrench:

    $ wrench svc start

`

	// Parse flags for our subcommand.
	flagSet := flag.NewFlagSet("start", flag.ExitOnError)

	// Handles calls to our subcommand.
	handler := func(args []string) error {
		_ = flagSet.Parse(args)

		service, _ := newServiceBot()
		err := service.Start()
		if err != nil && !strings.Contains(err.Error(), "Warning: Expecting a LaunchAgents path") {
			return err
		}
		return nil
	}

	// Register the command.
	serviceCommands = append(serviceCommands, &cmder.Command{
		FlagSet: flagSet,
		Aliases: []string{},
		Handler: handler,
		UsageFunc: func() {
			fmt.Fprintf(flag.CommandLine.Output(), "Usage of 'wrench service %s':\n", flagSet.Name())
			flagSet.PrintDefaults()
			fmt.Printf("%s", usage)
		},
	})
}
