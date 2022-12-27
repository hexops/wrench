package scripts

import (
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
	return doAppendEnvPermanent(key, value, setEnvPermanent)
}
