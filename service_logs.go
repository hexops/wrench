package main

import (
	"flag"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/hexops/cmder"
	"github.com/hexops/wrench/internal/errors"
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

		grep := map[string]struct{}{}
		var files []string
		if runtime.GOOS == "darwin" {
			files = []string{
				"/var/log/com.apple.xpc.launchd/launchd.log",
				"/var/log/wrench.out.log",
				"/var/log/wrench.err.log",
			}
			grep["/var/log/com.apple.xpc.launchd/launchd.log"] = struct{}{}
		} else {
			fmt.Println("'wrench svc logs' not supported on this OS.")
			return nil
		}

		tails := []*tail.Tail{}
		for _, path := range files {
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

		lastLine := time.Now()
		delayed := false
		for {
			for i, tail := range tails {
				select {
				case line, ok := <-tail.Lines:
					lastLine = time.Now()
					if !ok {
						tails = slices.Delete(tails, i, i)
						break
					}
					if _, ok := grep[tail.Filename]; ok {
						if !strings.Contains(line.Text, "wrench") {
							continue
						}
					}
					if !*showAllFlag {
						if !delayed {
							continue
						}
					}
					fmt.Println(tail.Filename + ": " + line.Text)
				default:
				}
			}
			if time.Since(lastLine) > 1*time.Second {
				delayed = true
				time.Sleep(100 * time.Millisecond)
			}
		}
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
