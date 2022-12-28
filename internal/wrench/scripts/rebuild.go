package scripts

import "os"

func init() {
	Scripts = append(Scripts, Script{
		Command:     "rebuild",
		Args:        nil,
		Description: "wrench installs prerequisites (Go), rebuilds itself, and restarts the service",
		Execute: func(args ...string) error {
			return Sequence(
				Exec("wrench script install-go"),
				Exec("wrench script rebuild-only"),
				Exec("wrench svc restart").IgnoreError(),
			)(os.Stderr)
		},
	})
}
