package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hexops/cmder"
	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/wrench"
	"github.com/nxadm/tail"
	"golang.org/x/exp/slices"
)

func init() {
	const usage = `
Examples:

  Tail the logs of the wrench service:

    $ wrench service logs

`

	// Parse flags for our subcommand.
	flagSet := flag.NewFlagSet("logs", flag.ExitOnError)
	followFlag := flagSet.Bool("f", true, "follow log files")
	showAllFlag := flagSet.Bool("a", false, "show all logs, not just new ones")

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

		var cfg wrench.Config
		err = wrench.LoadConfig(*serviceConfigFile, &cfg)
		if err != nil {
			return errors.Wrap(err, "LoadConfig")
		}

		files := []string{cfg.LogFilePath()}

		tails := []*tail.Tail{}
		for _, path := range files {
			_, err := os.Stat(path)
			if err != nil {
				fmt.Println("warning:", err)
			}
			tf, err := tail.TailFile(path, tail.Config{
				Follow: *followFlag,
				ReOpen: *followFlag,
			})
			if err != nil {
				fmt.Println("wrench: failed to tail file", path, "error", err)
				continue
			}
			tails = append(tails, tf)
		}
		if len(tails) == 0 {
			return nil
		}

		start := time.Now()
		lastLine := time.Now()
		for {
		sliceUpdated:
			for i, tail := range tails {
				select {
				case line, ok := <-tail.Lines:
					lastLine = time.Now()
					if !ok {
						tails = slices.Delete(tails, i, i)
						goto sliceUpdated
					}
					if !*showAllFlag {
						split := strings.Split(line.Text, " ")
						t, err := time.Parse(time.RFC3339, split[0])
						if err == nil && !t.After(start) {
							continue
						}
					}
					fmt.Println(line.Text)
				default:
				}
			}
			if time.Since(lastLine) > 1*time.Second {
				time.Sleep(100 * time.Millisecond)
			}
		}
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
