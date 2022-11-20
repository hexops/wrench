package main

import (
	"flag"
	"fmt"

	"github.com/hexops/cmder"
	"github.com/hexops/wrench/internal/wrench"
)

func init() {
	const usage = `
Examples:

  Start performing work:

    $ wrench start

`

	// Parse flags for our subcommand.
	flagSet := flag.NewFlagSet("start", flag.ExitOnError)
	configFile := flagSet.String("config", "config.toml", "Path to TOML configuration file (see config.go)")

	// Handles calls to our subcommand.
	handler := func(args []string) error {
		_ = flagSet.Parse(args)
		bot := wrench.Bot{
			ConfigFile: *configFile,
		}
		return bot.Start()
	}

	// Register the command.
	commands = append(commands, &cmder.Command{
		FlagSet: flagSet,
		Aliases: []string{},
		Handler: handler,
		UsageFunc: func() {
			fmt.Fprintf(flag.CommandLine.Output(), "Usage of 'wrench %s':\n", flagSet.Name())
			flagSet.PrintDefaults()
			fmt.Printf("%s", usage)
		},
	})
}
