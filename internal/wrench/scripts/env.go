package scripts

import (
	"path/filepath"

	"github.com/hexops/wrench/internal/errors"
)

// Sets the given env var key=value permanently.
//
// Windows: the registry is modified
// Linux: /etc/environment.d/wrench.sh is appended to if an entry does not exist
// macOS: /System/Volumes/Data/private/etc/zshrc is appended to if an entry does not exist
func SetEnvPermanent(key, value string) error {
	return setEnvPermanent(key, value)
}

// Ensures dir is on the system PATH persistently.
func EnsureOnPathPermanent(dir string) error {
	return EnsureInEnvListPermanent("PATH", dir, filepath.Abs)
}

// Ensures the given value is in an environment list (like PATH) permanently.
// no-op if already in this process env.
//
// normalize(value) is called on every value in the list (including the input value)
// in order to compare env list values. If it is not present, then value is appended
// to the original list.
func EnsureInEnvListPermanent(key, value string, normalize func(value string) (string, error)) error {
	value, err := normalize(value)
	if err != nil {
		return errors.Wrap(err, "normalize")
	}

	err = appendEnvPermanent(key, value)
	return errors.Wrap(err, "appendEnvPermanent")
}
