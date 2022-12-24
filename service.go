package main

import (
	"flag"
	"fmt"
	"log"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"

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

	var options service.KeyValue
	var envVars map[string]string
	if runtime.GOOS == "linux" {
		options = make(service.KeyValue)
		options["RestartSec"] = 1 // default is 120
		u, err := user.Current()
		if err != nil {
			log.Fatal("user.Current", err)
		}
		envVars = map[string]string{"HOME": u.HomeDir}
	}

	var executable string
	var arguments []string
	wrenchCmd := fmt.Sprintf(`%s service -config=%s run`, config.Executable, config.ConfigFile)
	switch runtime.GOOS {
	case "linux":
		var err error
		executable, err = exec.LookPath("sh")
		if err != nil {
			log.Fatal("LookPath", err)
		}
		arguments = []string{"-lc", wrenchCmd}
	case "darwin":
		var err error
		executable, err = exec.LookPath("zsh")
		if err != nil {
			log.Fatal("LookPath", err)
		}
		arguments = []string{"-c", wrenchCmd}
	case "windows":
		executable = config.Executable
		arguments = []string{"service", "-config=" + config.ConfigFile, "run"}
	}

	// TODO: should perhaps allow setting Arguments, Executable, and EnvVars via config.toml
	svcConfig := &service.Config{
		Name:        "wrench",
		DisplayName: "Wrench",
		Description: "Let's fix this!",
		Arguments:   arguments,
		Executable:  executable,
		EnvVars:     envVars,
		Option:      options,
	}
	s, err := service.New(bot, svcConfig)
	if err != nil {
		log.Fatal("creating service", err)
	}
	return s, bot
}
