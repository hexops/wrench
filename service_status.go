package main

import (
	"flag"
	"fmt"
	"runtime"

	"github.com/hexops/cmder"
	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/wrench"
)

func init() {
	const usage = `
Examples:

  Check if wrench is running on the system:

    $ wrench service status

`

	// Parse flags for our subcommand.
	flagSet := flag.NewFlagSet("status", flag.ExitOnError)

	// Handles calls to our subcommand.
	handler := func(args []string) error {
		_ = flagSet.Parse(args)

		ok, err := isRoot()
		if err != nil {
			return errors.Wrap(err, "isRoot")
		}
		if !ok {
			fmt.Println("error: please run as root")
			return nil
		}

		svc, _ := newServiceBot()
		status, err := wrench.ServiceStatus(svc)
		if err != nil {
			return errors.Wrap(err, "ServiceStatus")
		}
		fmt.Printf("%s registered in %s\n", svc.String(), svc.Platform())
		if runtime.GOOS == "darwin" {
			fmt.Println("LaunchDaemon: /Library/LaunchDaemons/wrench.plist")
			fmt.Println("stdout: /var/log/wrench.out.log")
			fmt.Println("stderr: /var/log/wrench.err.log")
		}
		if runtime.GOOS == "linux" && svc.Platform() == "linux-systemd" {
			fmt.Println("systemd: /etc/systemd/system/wrench.service")
			fmt.Println("logs: sudo journalctl -u wrench -e")
		}
		fmt.Println("")
		fmt.Println(status)
		return nil
	}

	// Register the command.
	serviceCommands = append(serviceCommands, &cmder.Command{
		FlagSet: flagSet,
		Aliases: []string{},
		Handler: handler,
		UsageFunc: func() {
			fmt.Fprintf(flag.CommandLine.Output(), "Usage of 'wrench service %s':\n", flagSet.Name())
			flagSet.PrintDefaults()
			fmt.Printf("%s", usage)
		},
	})
}
