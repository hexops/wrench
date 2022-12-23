//go:build !windows

package scripts

import (
	"runtime"

	"github.com/hexops/wrench/internal/errors"
)

func setEnvPermanent(key, value string) error {
	switch runtime.GOOS {
	case "darwin":
		err := AppendToFile("/System/Volumes/Data/private/etc/zshrc", "export %s=%s", key, value)()
		if err != nil {
			return errors.Wrap(err, "appending to system zshrc")
		}
		return nil
	case "linux":
		err := AppendToFile("/etc/profile.d/wrench.sh", "%s=%s", key, value)()
		if err != nil {
			return errors.Wrap(err, "appending to /etc/profile.d/wrench.sh")
		}
		return nil
	}
	return errors.New("not implemented for this OS")
}

func appendEnvPermanent(key, value string) error {
	switch runtime.GOOS {
	case "darwin":
		err := AppendToFile("/System/Volumes/Data/private/etc/zshrc", "export %s=$%s:%s", key, key, value)()
		if err != nil {
			return errors.Wrap(err, "appending to system zshrc")
		}
		return nil
	case "linux":
		err := AppendToFile("/etc/profile.d/wrench.sh", "%s=$%s:%s", key, key, value)()
		if err != nil {
			return errors.Wrap(err, "appending to /etc/profile.d/wrench.sh")
		}
		return nil
	}
	return errors.New("not implemented for this OS")
}
