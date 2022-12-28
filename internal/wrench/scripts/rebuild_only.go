package scripts

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/hexops/wrench/internal/errors"
)

func init() {
	Scripts = append(Scripts, Script{
		Command:     "rebuild-only",
		Args:        nil,
		Description: "wrench rebuilds and reinstalls itself",
		Execute: func(args ...string) error {
			if err := Sequence(
				Exec("git clone https://github.com/hexops/wrench").IgnoreError(),
				Exec("git fetch", WorkDir("wrench")),
				Exec("git reset --hard origin/main", WorkDir("wrench")),
			)(os.Stderr); err != nil {
				return err
			}

			date := time.Now().Format(time.RFC3339)
			goVersion, err := Output("go version")
			if err != nil {
				return err
			}
			version, err := Output("git describe --abbrev=8 --dirty --always --long", WorkDir("wrench"))
			if err != nil {
				return err
			}
			commitTitle, err := Output("git log -1 --pretty=format:%s", WorkDir("wrench"))
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
				func(w io.Writer) error {
					exePath, err := os.Executable()
					if err != nil {
						return errors.Wrap(err, "Executable")
					}
					newBinary := "wrench/bin/wrench"

					if runtime.GOOS == "windows" {
						// On Windows you can't delete the binary of a running program as it is in
						// use, but you can rename it. So we do this:
						//
						// Delete wrench-old.exe if it exists
						// Rename wrench.exe -> wrench-old.exe
						// Rename new build -> wrench.exe
						exe2Path := strings.TrimSuffix(exePath, ".exe") + "-old.exe"
						fmt.Fprintf(w, "$ rm -f %s\n", exe2Path)
						_ = os.Remove(exe2Path)
						fmt.Fprintf(w, "$ mv %s %s\n", exePath, exe2Path)
						err = os.Rename(exePath, exe2Path)
						if err != nil {
							return errors.Wrap(err, "Rename")
						}
					} else {
						if err := os.Remove(exePath); err != nil {
							return errors.Wrap(err, "Remove")
						}
					}
					fmt.Fprintf(w, "$ mv %s %s\n", newBinary, exePath)
					err = os.Rename("wrench/bin/wrench", exePath)
					if err != nil {
						return errors.Wrap(err, "Rename")
					}
					return nil
				},
			)(os.Stderr)
		},
	})
}
