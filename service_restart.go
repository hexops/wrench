package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/hexops/cmder"
	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/wrench/scripts"
)

func init() {
	const usage = `
Examples:

  Restart wrench:

    $ wrench svc restart

`

	// Parse flags for our subcommand.
	flagSet := flag.NewFlagSet("restart", flag.ExitOnError)

	// Handles calls to our subcommand.
	handler := func(args []string) error {
		_ = flagSet.Parse(args)

		service, _ := newServiceBot()
		if runtime.GOOS == "darwin" {
			// kickstart -k is a better / safer approacher to restarting on macOS.
			// https://github.com/kardianos/service/issues/358
			if err := scripts.Exec(`launchctl kickstart -k system/wrench`)(os.Stderr); err != nil {
				return errors.Wrap(err, "restart")
			}
			return nil
		}
		return service.Restart()
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
