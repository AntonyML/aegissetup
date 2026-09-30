// © Antony Monge López — Costa Rica — Céd. 604700548
package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var checkTestDSN string

// TestMain aísla credenciales y directorios antes de que Run resuelva defaults.
// La integración con EXE sigue requiriendo AEGIS_VERIFY_EXE explícito.
func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "aegis-check-tests-")
	if err != nil {
		panic("no se pudo crear el entorno aislado de pruebas de check")
	}
	checkTestDSN = "AEGIS_CHECK_TEST_" + strings.TrimPrefix(filepath.Base(root), "aegis-check-tests-")
	for name, value := range map[string]string{
		"AEGIS_APPDATA":      filepath.Join(root, "appdata"),
		"AEGIS_PROGRAMDATA":  filepath.Join(root, "programdata"),
		"AEGIS_SQL_PASSWORD": "",
		"AEGIS_SA_PASSWORD":  "",
	} {
		if err := os.Setenv(name, value); err != nil {
			_ = os.RemoveAll(root)
			panic("no se pudo aislar la configuración de pruebas de check")
		}
	}
	code := m.Run()
	_ = os.RemoveAll(root)
	os.Exit(code)
}
