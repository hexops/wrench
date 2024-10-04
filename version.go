package main

import (
	"flag"
	"fmt"

	"github.com/hexops/cmder"
	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/wrench"
)

func init() {
	usage := `wrench version: print the wrench version

Usage:

	wrench version

`

	// Parse flags for our subcommand.
	flagSet := flag.NewFlagSet("version", flag.ExitOnError)

	// Handles calls to our subcommand.
	handler := func(args []string) error {
		_ = flagSet.Parse(args)
		if len(args) != 0 {
			return &cmder.UsageError{Err: errors.New("expected no arguments")}
		}

		fmt.Println("wrench version", wrench.Version, "built using", wrench.GoVersion)

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
