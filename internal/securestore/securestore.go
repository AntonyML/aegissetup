// © Antony Monge López — Costa Rica — Céd. 604700548
package securestore

import (
	"os"
	"strings"
)

const envSQLPassword = "AEGIS_SQL_PASSWORD"

// SavePassword almacena la contraseña de SQL Server de forma segura y sanitizada.
func SavePassword(password string) error {
	clean := strings.TrimSpace(password)
	if clean == "" {
		return DeletePassword()
	}
	return storeEncrypted([]byte(clean))
}

// GetPassword recupera la contraseña cifrada almacenada.
func GetPassword() (string, error) {
	data, err := readEncrypted()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// HasPassword indica si hay una credencial almacenada en el almacén seguro.
func HasPassword() bool {
	p, err := GetPassword()
	return err == nil && p != ""
}

// DeletePassword elimina la credencial del almacén seguro.
func DeletePassword() error {
	return removeStore()
}

// ResolvePassword resuelve la contraseña SQL siguiendo la precedencia estricta:
// 1. Argumento explícito (parámetro de CLI / prompt interactivo)
// 2. Variable de entorno AEGIS_SQL_PASSWORD (solo override para CI / headless)
// 3. Almacén seguro local (DPAPI en Windows)
// 4. Contraseña guardada en el DSN ODBC existente (fallback si ya está configurado)
func ResolvePassword(explicitPass string, dsnFallback func() string) string {
	if clean := strings.TrimSpace(explicitPass); clean != "" {
		return clean
	}
	if env := strings.TrimSpace(os.Getenv(envSQLPassword)); env != "" {
		return env
	}
	if p, err := GetPassword(); err == nil && p != "" {
		return p
	}
	if dsnFallback != nil {
		if dsnP := strings.TrimSpace(dsnFallback()); dsnP != "" {
			return dsnP
		}
	}
	return ""
}
