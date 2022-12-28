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

  Upsert a secrets:

    $ wrench secret upsert [id] [value]

`

	// Parse flags for our subcommand.
	flagSet := flag.NewFlagSet("upsert", flag.ExitOnError)

	// Handles calls to our subcommand.
	handler := func(args []string) error {
		_ = flagSet.Parse(args)
		if len(args) != 2 {
			return &cmder.UsageError{Err: errors.New("expected [id] [value] arguments")}
		}

		ctx := context.Background()
		client, err := wrench.Client(*secretConfigFile)
		if err != nil {
			return errors.Wrap(err, "Client")
		}
		_, err = client.SecretsUpsert(ctx, &api.SecretsUpsertRequest{
			ID:    args[0],
			Value: args[1],
		})
		if err != nil {
			return errors.Wrap(err, "SecretsUpsert")
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
