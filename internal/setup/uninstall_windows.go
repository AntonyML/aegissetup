//go:build windows

// © Antony Monge López — Costa Rica — Céd. 604700548

package setup

import (
	"errors"

	"golang.org/x/sys/windows/registry"
)

// Rutas del DSN de 32 bits. Son las MISMAS que escribe dsn_windows.go: si acá se cambiara
// una, la desinstalación dejaría el DSN vivo y el próximo WriteDSN lo actualizaría en vez
// de crearlo, que es justo el estado sucio que se quiere evitar.
const (
	raizODBC     = `SOFTWARE\WOW6432Node\ODBC\ODBC.INI\`
	subListaODBC = `SOFTWARE\WOW6432Node\ODBC\ODBC.INI\ODBC Data Sources`
)

func soportaDSN() bool { return true }

func claveDSN(dsn string) string { return raizODBC + dsn }

func claveDSNLista() string { return subListaODBC }

// borrarDSN borra la clave completa del DSN: se lleva todos sus valores de una.
func borrarDSN(clave string) error {
	err := registry.DeleteKey(registry.LOCAL_MACHINE, clave)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	return err
}

// borrarDSNLista borra solo el VALOR del DSN dentro de "ODBC Data Sources". Esa clave la
// comparten todos los DSN de la máquina, así que borrarla entera se llevaría los ajenos.
func borrarDSNLista(dsn string) error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, subListaODBC, registry.SET_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer k.Close()
	err = k.DeleteValue(dsn)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	return err
}
