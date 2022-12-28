package main

import (
	"flag"
	"fmt"

	"github.com/hexops/cmder"
)

// secretCommands contains all registered 'wrench secret' subcommands.
var secretCommands cmder.Commander

var (
	secretFlagSet    = flag.NewFlagSet("secret", flag.ExitOnError)
	secretConfigFile = secretFlagSet.String("config", defaultConfigFilePath(), "Path to TOML configuration file (see config.go)")
)

func init() {
	const usage = `wrench secret: manage wrench as a secret

Usage:

	wrench secret [-config=config.toml] <command> [arguments]

The commands are:

	list         list all secrets
	delete       delete a secret
	upsert       create or update a secret

Use "wrench secret <command> -h" for more information about a command.
`

	usageFunc := func() {
		fmt.Printf("%s", usage)
	}
	secretFlagSet.Usage = usageFunc

	// Handles calls to our subcommand.
	handler := func(args []string) error {
		_ = secretFlagSet.Parse(args)
		secretCommands.Run(secretFlagSet, "wrench secret", usage, args)
		return nil
	}

	// Register the command.
	commands = append(commands, &cmder.Command{
		FlagSet:   secretFlagSet,
		Handler:   handler,
		UsageFunc: usageFunc,
	})
}
