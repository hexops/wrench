package scripts

import (
	"fmt"
	"os"
	"time"

	"github.com/hexops/wrench/internal/errors"
)

func init() {
	Scripts = append(Scripts, Script{
		Command:     "rebuild",
		Args:        nil,
		Description: "wrench rebuilds and reinstalls itself",
		Execute: func(args ...string) error {
			if err := Sequence(
				Exec("git clone https://github.com/hexops/wrench").IgnoreError(),
				Exec("git fetch", WorkDir("wrench")),
				Exec("git reset --hard origin/main", WorkDir("wrench")),
			)(); err != nil {
				return err
			}

			date := time.Now().Format(time.RFC3339)
			goVersion, err := Output("go version")
			if err != nil {
				return err
			}
			version, err := Output("git describe --tags --abbrev=8 --dirty --always --long", WorkDir("wrench"))
			if err != nil {
				return err
			}
			commitTitle, err := Output("git log --pretty=format:%s HEAD^1..HEAD", WorkDir("wrench"))
			if err != nil {
				return err
			}
			prefix := "github.com/hexops/wrench/internal/wrench"
			ldFlags := fmt.Sprintf("-X '%s.Version=%s'", prefix, version)
			ldFlags += fmt.Sprintf(" -X '%s.CommitTitle=%s'", prefix, commitTitle)
			ldFlags += fmt.Sprintf(" -X '%s.Date=%s'", prefix, date)
			ldFlags += fmt.Sprintf(" -X '%s.GoVersion=%s'", prefix, goVersion)

			return Sequence(
				ExecArgs("go", []string{"build", "-ldflags", ldFlags, "-o", "bin/wrench", "."}, WorkDir("wrench")),
				func() error {
					exePath, err := os.Executable()
					if err != nil {
						return errors.Wrap(err, "Executable")
					}
					newBinary := "wrench/bin/wrench"
					fmt.Printf("$ mv %s %s\n", newBinary, exePath)
					if err := os.Remove(exePath); err != nil {
						return errors.Wrap(err, "Remove")
					}
					err = os.Rename("wrench/bin/wrench", exePath)
					if err != nil {
						return errors.Wrap(err, "Rename")
					}
					return nil
				},
			)()
		},
	})
}
