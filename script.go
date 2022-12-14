package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/hexops/cmder"
	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/wrench/scripts"
)

func init() {
	usage := `wrench script: run a script built-in to wrench

Usage:

	wrench script <command> [arguments]

The scripts are:

`

	maxCmdStrLen := 0
	for _, script := range scripts.Scripts {
		cmdStr := script.Command
		if len(script.Args) > 0 {
			cmdStr = fmt.Sprintf("%s [%s]", cmdStr, strings.Join(script.Args, "] ["))
		}
		if len(cmdStr) > maxCmdStrLen {
			maxCmdStrLen = len(cmdStr)
		}
	}
	for _, script := range scripts.Scripts {
		cmdStr := script.Command
		if len(script.Args) > 0 {
			cmdStr = fmt.Sprintf("%s [%s]", cmdStr, strings.Join(script.Args, "] ["))
		}
		usage = fmt.Sprintf("%s\n	%-"+fmt.Sprint(maxCmdStrLen+2)+"s%s", usage, cmdStr, script.Description)
	}
	usage += "\n\n"

	// Parse flags for our subcommand.
	flagSet := flag.NewFlagSet("script", flag.ExitOnError)

	// Handles calls to our subcommand.
	handler := func(args []string) error {
		_ = flagSet.Parse(args)
		if len(args) < 1 {
			return &cmder.UsageError{Err: errors.New("expected command")}
		}
		for _, script := range scripts.Scripts {
			if args[0] != script.Command {
				continue
			}
			resp, err := script.Run(args[1:]...)
			if err != nil {
				return err
			}
			if resp != nil {
				data, err := json.MarshalIndent(resp, "", "  ")
				if err != nil {
					return errors.Wrap(err, "Marshal")
				}
				fmt.Fprintf(os.Stdout, "%s\n", data)
			}
			return nil
		}
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
