package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/hexops/cmder"
	"github.com/hexops/wrench/internal/wrench"
	"github.com/kardianos/service"
)

// serviceCommands contains all registered 'wrench service' subcommands.
var serviceCommands cmder.Commander

var (
	serviceFlagSet    = flag.NewFlagSet("service", flag.ExitOnError)
	serviceConfigFile = serviceFlagSet.String("config", "config.toml", "Path to TOML configuration file (see config.go)")
)

func init() {
	const usage = `wrench service: manage wrench as a service

Usage:

	wrench service [-config=config.toml] <command> [arguments]

The commands are:

	start    run the server now

Use "wrench service <command> -h" for more information about a command.
`

	usageFunc := func() {
		fmt.Printf("%s", usage)
	}
	serviceFlagSet.Usage = usageFunc

	// Handles calls to our subcommand.
	handler := func(args []string) error {
		_ = serviceFlagSet.Parse(args)
		serviceCommands.Run(serviceFlagSet, "wrench service", usage, args)
		return nil
	}

	// Register the command.
	commands = append(commands, &cmder.Command{
		FlagSet:   serviceFlagSet,
		Aliases:   []string{"svc"},
		Handler:   handler,
		UsageFunc: usageFunc,
	})
}

func newServiceBot() (service.Service, *wrench.Bot) {
	bot := &wrench.Bot{
		ConfigFile: *serviceConfigFile,
	}

	// TODO: should perhaps allow setting Arguments, Executable, and EnvVars via config.toml
	svcConfig := &service.Config{
		Name:        "wrench",
		DisplayName: "Wrench",
		Description: "Let's fix this!",
		Arguments:   []string{"start"},
	}
	s, err := service.New(bot, svcConfig)
	if err != nil {
		log.Fatal("creating service", err)
	}
	return s, bot
}
