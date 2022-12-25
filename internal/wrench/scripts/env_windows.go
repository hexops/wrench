package scripts

import (
	"os"
	"strings"

	"github.com/hexops/wrench/internal/errors"
	"golang.org/x/sys/windows/registry"
)

func setEnvPermanent(key, value string) error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\ControlSet001\Control\Session Manager\Environment`, registry.ALL_ACCESS)
	if err != nil {
		return errors.Wrap(err, "OpenKey "+key)
	}
	defer k.Close()

	err = k.SetStringValue(key, value)
	if err != nil {
		return errors.Wrap(err, "SetStringValue "+key)
	}
	return nil
}

func appendEnvPermanent(key, value string) error {
	current, _ := os.LookupEnv(key)
	currentList := strings.Split(current, string(os.PathListSeparator))

	// Confirm it's not already in the list.
	for _, existing := range currentList {
		if existing == value {
			// already in list
			return nil
		}
	}

	// Add value to list and update env var.
	currentList = append(currentList, value)
	newValue := strings.Join(currentList, string(os.PathListSeparator))
	err := setEnvPermanent(key, newValue)
	return errors.Wrap(err, "setEnvPermanent")
}
