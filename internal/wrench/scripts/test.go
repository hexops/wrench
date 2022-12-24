package scripts

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
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
			gist, err := transformGistURL(args[0])
			if err != nil {
				return errors.Wrap(err, "transformGistURL")
			}

			// Fetch the gist file.
			resp, err := http.Get(gist.String())
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

func transformGistURL(urlString string) (*url.URL, error) {
	u, err := url.Parse(urlString)
	if err != nil {
		return nil, errors.Wrap(err, "gist is not a valid gist URL")
	}
	if u.Host == "gist.github.com" {
		// transform URL:
		// https://gist.github.com/slimsag/ac2f04a101680631ba3b2c99f8180d2d
		// ->
		// https://gist.githubusercontent.com/slimsag/ac2f04a101680631ba3b2c99f8180d2d/raw
		u.Host = "gist.githubusercontent.com"
		u.Path += "/raw"
		return u, nil
	}
	return nil, fmt.Errorf("%q is not an accepted gist host", u.Host)
}
