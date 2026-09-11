//go:build !windows

// © Antony Monge López — Costa Rica — Céd. 604700548

package setup

import "errors"

// Fuera de Windows no hay registro de ODBC que limpiar, así que el DSN no entra en el
// plan. Devolver false (y no un paso que falla siempre) mantiene honesto el plan: no
// anuncia algo que no puede hacer.
func soportaDSN() bool { return false }

func claveDSN(string) string { return "" }

func claveDSNLista() string { return "" }

func borrarDSN(string) error {
	return errors.New("el DSN solo existe en Windows")
}

func borrarDSNLista(string) error {
	return errors.New("el DSN solo existe en Windows")
}
