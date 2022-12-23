package scripts

func init() {
	Scripts = append(Scripts, Script{
		Command:     "rebuild",
		Args:        nil,
		Description: "wrench rebuilds and reinstalls itself, installing prerequisites (Go) if needed",
		Execute: func(args ...string) error {
			return Sequence(
				Exec("wrench script install-go"),
				Exec("wrench script rebuild"),
			)()
		},
	})
}
