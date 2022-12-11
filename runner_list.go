package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/hexops/cmder"
	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/wrench"
	"github.com/hexops/wrench/internal/wrench/api"
)

func init() {
	const usage = `
Examples:

  List registered runners:

    $ wrench runner list

`

	// Parse flags for our subcommand.
	flagSet := flag.NewFlagSet("runners", flag.ExitOnError)
	configFile := flagSet.String("config", "config.toml", "Path to TOML configuration file (see config.go)")

	// Handles calls to our subcommand.
	handler := func(args []string) error {
		_ = flagSet.Parse(args)
		ctx := context.Background()
		client, err := wrench.Client(*configFile)
		if err != nil {
			return errors.Wrap(err, "Client")
		}
		resp, err := client.RunnerList(ctx, &api.RunnerListRequest{})
		if err != nil {
			return errors.Wrap(err, "RunnerList")
		}
		if len(resp.Runners) == 0 {
			fmt.Println("no runners found")
		}
		for _, runner := range resp.Runners {
			fmt.Printf("'%v' (%v)\n", runner.ID, runner.Arch)
			fmt.Printf("    registered: %v ago\n", time.Since(runner.RegisteredAt).Round(time.Hour*24))
			fmt.Printf("    last seen: %v ago\n\n", time.Since(runner.LastSeenAt).Round(time.Second))
		}
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
			fmt.Printf("%s", usage)
		},
	})
}
