package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/hexops/cmder"
	"github.com/hexops/wrench/internal/wrench"
	"github.com/kardianos/service"
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

		service, _ := newServiceBot(*configFile)
		return service.Run()
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

func newServiceBot(configFile string) (service.Service, *wrench.Bot) {
	bot := &wrench.Bot{
		ConfigFile: configFile,
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
