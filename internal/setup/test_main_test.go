// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain mantiene los archivos y las credenciales de las pruebas fuera de la
// instalación del operador, incluso cuando se ejecuta go test sin overrides.
func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "aegis-setup-tests-")
	if err != nil {
		panic("no se pudo crear el entorno aislado de pruebas")
	}
	for name, value := range map[string]string{
		"AEGIS_APPDATA":      filepath.Join(root, "appdata"),
		"AEGIS_PROGRAMDATA":  filepath.Join(root, "programdata"),
		"AEGIS_SQL_PASSWORD": "",
	} {
		if err := os.Setenv(name, value); err != nil {
			_ = os.RemoveAll(root)
			panic("no se pudo aislar la configuración de pruebas")
		}
	}
	code := m.Run()
	_ = os.RemoveAll(root)
	os.Exit(code)
}
