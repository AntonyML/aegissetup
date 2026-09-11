//go:build !windows
// © Antony Monge López — Costa Rica — Céd. 604700548

package setup

import (
	"errors"

	"aegis-setup/internal/config"
)

func WriteDSN(cfg config.Config, appPass string, savePWD bool, out func(string)) error {
	return errors.New("DSN solo se escribe en Windows (usa odbcad32 o docker en Linux)")
}

func ReadDSN(dsn string) (map[string]string, error) {
	return nil, errors.New("DSN solo se lee en Windows")
}
