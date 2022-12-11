package main

import (
	"flag"
	"fmt"

	"github.com/hexops/cmder"
)

func init() {
	const usage = `
Examples:

  Run wrench:

    $ wrench svc run

`

	// Parse flags for our subcommand.
	flagSet := flag.NewFlagSet("run", flag.ExitOnError)

	// Handles calls to our subcommand.
	handler := func(args []string) error {
		_ = flagSet.Parse(args)

		service, _ := newServiceBot()
		return service.Run()
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
