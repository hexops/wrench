//go:build !windows

package scripts

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/hexops/wrench/internal/errors"
)

func setEnvPermanent(key, value string) error {
	switch runtime.GOOS {
	case "darwin":
		return envFileEnsureLine(fmt.Sprintf("export %s=%s", key, value))
	case "linux":
		return envFileEnsureLine(fmt.Sprintf("%s=%s", key, value))
	default:
		return errors.New("not implemented for this OS")
	}
}

func appendEnvPermanent(key, value string) error {
	switch runtime.GOOS {
	case "darwin":
		return envFileEnsureLine(fmt.Sprintf("export %s=$%s:%s", key, key, value))
	case "linux":
		return envFileEnsureLine(fmt.Sprintf("%s=$%s:%s", key, key, value))
	default:
		return errors.New("not implemented for this OS")
	}
}

func envFileEnsureLine(line string) error {
	var file string
	switch runtime.GOOS {
	case "darwin":
		file = "/System/Volumes/Data/private/etc/zprofile"
	case "linux":
		file = "/etc/profile.d/wrench.sh"
	default:
		return errors.New("not implemented for this OS")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return errors.Wrap(err, "ReadFile")
	}
	for _, existingLine := range strings.Split(string(data), "\n") {
		if existingLine == line {
			// already exists
			return nil
		}
	}

	err = AppendToFile(file, "\n"+line+"\n")()
	return errors.Wrap(err, "appending to "+file)
}
