package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/hexops/cmder"
	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/wrench"
	"github.com/hexops/wrench/internal/wrench/api"
)

func init() {
	const usage = `
Examples:

  List secrets:

    $ wrench secret list

`

	// Parse flags for our subcommand.
	flagSet := flag.NewFlagSet("list", flag.ExitOnError)

	// Handles calls to our subcommand.
	handler := func(args []string) error {
		_ = flagSet.Parse(args)

		ctx := context.Background()
		client, err := wrench.Client(*secretConfigFile)
		if err != nil {
			return errors.Wrap(err, "Client")
		}
		resp, err := client.SecretsList(ctx, &api.SecretsListRequest{})
		if err != nil {
			return errors.Wrap(err, "SecretsList")
		}
		for _, secretID := range resp.IDs {
			fmt.Println(secretID)
		}
		return nil
	}

	// Register the command.
	secretCommands = append(secretCommands, &cmder.Command{
		FlagSet: flagSet,
		Handler: handler,
		UsageFunc: func() {
			fmt.Fprintf(flag.CommandLine.Output(), "Usage of 'wrench secret %s':\n", flagSet.Name())
			flagSet.PrintDefaults()
			fmt.Printf("%s", usage)
		},
	})
}
