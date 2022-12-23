package scripts

import (
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"

	"github.com/hexops/wrench/internal/errors"
)

func init() {
	Scripts = append(Scripts, Script{
		Command:     "test",
		Args:        []string{"gist"},
		Description: "download and execute a gist",
		Execute: func(args ...string) error {
			if len(args) != 1 {
				return errors.New("expected argument: [gist URL]")
			}
			// Fetch the gist file.
			resp, err := http.Get(args[0])
			if err != nil {
				return errors.Wrap(err, "Get")
			}
			defer resp.Body.Close()

			// Write gist to a tmp file
			tmpFile, err := ioutil.TempFile("", "wrench-test")
			if err != nil {
				return errors.Wrap(err, "TempFile")
			}
			defer os.Remove(tmpFile.Name())
			if _, err := io.Copy(tmpFile, resp.Body); err != nil {
				return errors.Wrap(err, "Copy")
			}

			// Mark tmp file as executable.
			if err := os.Chmod(tmpFile.Name(), 0o700); err != nil {
				return errors.Wrap(err, "Chmod")
			}

			if runtime.GOOS == "windows" {
				return ExecArgs("powershell.exe", []string{tmpFile.Name()})()
			}
			return ExecArgs("sh", []string{tmpFile.Name()})()
		},
	})
}
