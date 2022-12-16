package main

import (
	"flag"
	"log"
	"os"

	"github.com/hexops/cmder"
)

// commands contains all registered subcommands.
var commands cmder.Commander

var usageText = `wrench: let's fix this!

Usage:

	wrench <command> [arguments]

The commands are:

	service    manage the wrench service (also 'wrench svc')
	runners    list registered runners
	script     execute a script built-in to wrench

Use "wrench <command> -h" for more information about a command.
`

func main() {
	// Configure logging.
	log.SetFlags(0)
	log.SetPrefix("")

	commands.Run(flag.CommandLine, "wrench", usageText, os.Args[1:])
}
