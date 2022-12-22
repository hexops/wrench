package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hexops/cmder"
	"github.com/hexops/wrench/internal/wrench"
	"github.com/kardianos/service"
)

// serviceCommands contains all registered 'wrench service' subcommands.
var serviceCommands cmder.Commander

var (
	serviceFlagSet    = flag.NewFlagSet("service", flag.ExitOnError)
	serviceConfigFile = serviceFlagSet.String("config", defaultConfigFilePath(), "Path to TOML configuration file (see config.go)")
)

func init() {
	const usage = `wrench service: manage wrench as a service

Usage:

	wrench service [-config=config.toml] <command> [arguments]

The commands are:

	run          run the server now
	status       get the status of the wrench system service
	logs         view service logs (stderr, stdout, and system service runner logs)
	start        start wrench as a system service
	stop         stop wrench as a system service
	install      install wrench as a system service
	uninstall    uninstall wrench as a system service

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

func defaultConfigFilePath() string {
	u, err := user.Current()
	if err == nil {
		return filepath.Join(u.HomeDir, "wrench/config.toml")
	}
	return "config.toml"
}

func newServiceBot() (service.Service, *wrench.Bot) {
	return newServiceBotWithConfig(&ServiceConfig{
		ConfigFile: *serviceConfigFile,
		Executable: "",
	})
}

type ServiceConfig struct {
	ConfigFile string
	Executable string
}

func newServiceBotWithConfig(config *ServiceConfig) (service.Service, *wrench.Bot) {
	bot := &wrench.Bot{
		ConfigFile: config.ConfigFile,
	}

	var env map[string]string
	var options service.KeyValue
	if runtime.GOOS == "linux" {
		options = make(service.KeyValue)
		options["RestartSec"] = 1 // default is 120

		env = make(map[string]string)
		for _, kv := range os.Environ() {
			split := strings.SplitN(kv, "=", 1)
			if len(split) == 2 {
				env[split[0]] = split[1]
			} else {
				env[split[0]] = ""
			}
		}
	}

	// TODO: should perhaps allow setting Arguments, Executable, and EnvVars via config.toml
	svcConfig := &service.Config{
		Name:        "wrench",
		DisplayName: "Wrench",
		Description: "Let's fix this!",
		Arguments:   []string{"service", "-config=" + config.ConfigFile, "run"},
		Executable:  config.Executable,
		EnvVars:     env,
		Option:      options,
	}
	s, err := service.New(bot, svcConfig)
	if err != nil {
		log.Fatal("creating service", err)
	}
	return s, bot
}
