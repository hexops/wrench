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

  Delete a secrets:

    $ wrench secret delete [id]

`

	// Parse flags for our subcommand.
	flagSet := flag.NewFlagSet("delete", flag.ExitOnError)

	// Handles calls to our subcommand.
	handler := func(args []string) error {
		_ = flagSet.Parse(args)
		if len(args) != 1 {
			return &cmder.UsageError{Err: errors.New("expected [id] argument")}
		}

		ctx := context.Background()
		client, err := wrench.Client(*secretConfigFile)
		if err != nil {
			return errors.Wrap(err, "Client")
		}
		_, err = client.SecretsDelete(ctx, &api.SecretsDeleteRequest{ID: args[0]})
		if err != nil {
			return errors.Wrap(err, "SecretsDelete")
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
