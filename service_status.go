package main

import (
	"flag"
	"fmt"

	"github.com/hexops/cmder"
	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/wrench"
)

func init() {
	const usage = `
Examples:

  Check if wrench is running on the system:

    $ wrench service status

`

	// Parse flags for our subcommand.
	flagSet := flag.NewFlagSet("status", flag.ExitOnError)

	// Handles calls to our subcommand.
	handler := func(args []string) error {
		_ = flagSet.Parse(args)

		svc, _ := newServiceBot()
		status, err := wrench.ServiceStatus(svc)
		if err != nil {
			return errors.Wrap(err, "ServiceStatus")
		}
		fmt.Printf("%s registered in %s\n", svc.String(), svc.Platform())
		fmt.Println(status)
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
