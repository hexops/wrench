package wrench

import "fmt"

type Script struct {
	Command     string
	Args        []string
	Description string
	Execute     func(args ...string) error
}

var Scripts = []Script{
	{
		Command:     "rebuild",
		Args:        nil,
		Description: "wrench rebuilds and restarts itself",
		Execute: func(args ...string) error {
			fmt.Println("Not implemented yet.")
			return nil
		},
	},
}
