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

	start      begin working
	runners    list registered runners

Use "wrench <command> -h" for more information about a command.
`

func main() {
	// Configure logging if desired.
	log.SetFlags(0)
	log.SetPrefix("")

	commands.Run(flag.CommandLine, "wrench", usageText, os.Args[1:])
}
