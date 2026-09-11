// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aegis-setup/internal/config"
)

const (
	testWinBak    = `C:\DEV\SIDC\AegisSetup\assets\backups\sqlserver2014\SIDC_2014_COPYONLY_20260911.bak`
	testEngineBak = "/var/opt/mssql/backup/SIDC_2014_COPYONLY_20260911.bak"
)

// TestBakPathForEngine fija la ruta del .bak que se manda a FILELISTONLY y
// RESTORE. En Docker el motor corre en un contenedor Linux que no tiene C:\:
// el respaldo se monta :ro en /var/opt/mssql/backup (docker-compose.yml), así
// que pasarle la ruta Windows hace fallar el restore antes de empezar.
func TestBakPathForEngine(t *testing.T) {
	cases := []struct {
		name string
		mode config.DbMode
		in   string
		want string
	}{
		{"docker traduce la ruta Windows al mount del contenedor", config.DbDocker, testWinBak, testEngineBak},
		{"docker traduce igual si la ruta viene con barras normales", config.DbDocker, "C:/DEV/SIDC/assets/backups/sqlserver2014/SIDC_2014_COPYONLY_20260911.bak", testEngineBak},
		{"docker traduce una ruta UNC sin depender del nombre del respaldo", config.DbDocker, `\\srv\backups\SIDC_2014_COPYONLY_20260911.bak`, testEngineBak},
		{"docker deja intacta una ruta que ya es del motor", config.DbDocker, testEngineBak, testEngineBak},
		{"local deja la ruta Windows intacta", config.DbLocal, testWinBak, testWinBak},
		{"server deja la ruta Windows intacta", config.DbServer, testWinBak, testWinBak},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.Default()
			cfg.DbMode = tc.mode
			if got := bakPathForEngine(cfg, tc.in); got != tc.want {
				t.Errorf("bakPathForEngine(mode=%s, %q) = %q, quiero %q", tc.mode, tc.in, got, tc.want)
			}
		})
	}
}

// TestLocalStatNeeded cubre el guard que valida el .bak contra el sistema de
// archivos local. En Docker el operador puede pasar la ruta del motor, que por
// definición no existe en Windows: si se le pide os.Stat, --bak queda
// inutilizable y no hay forma de rodear el defecto desde la CLI.
func TestLocalStatNeeded(t *testing.T) {
	cases := []struct {
		name string
		mode config.DbMode
		in   string
		want bool
	}{
		{"docker con ruta del motor no se valida localmente", config.DbDocker, testEngineBak, false},
		{"docker con ruta Windows sí se valida localmente", config.DbDocker, testWinBak, true},
		{"local con ruta Windows se valida localmente", config.DbLocal, testWinBak, true},
		{"server con ruta Windows se valida localmente", config.DbServer, testWinBak, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.Default()
			cfg.DbMode = tc.mode
			if got := localStatNeeded(cfg, tc.in); got != tc.want {
				t.Errorf("localStatNeeded(mode=%s, %q) = %v, quiero %v", tc.mode, tc.in, got, tc.want)
			}
		})
	}
}

func TestFindNewestBak(t *testing.T) {
	t.Run("elige el .bak más nuevo e ignora el resto", func(t *testing.T) {
		dir := t.TempDir()
		viejo := writeFile(t, dir, "viejo.bak")
		nuevo := writeFile(t, dir, "nuevo.BAK") // la extensión se compara sin distinguir mayúsculas
		_ = writeFile(t, dir, "LEEME.txt")
		_ = writeFile(t, dir, "respaldo.bak.txt")
		if err := os.Mkdir(filepath.Join(dir, "directorio.bak"), 0o755); err != nil {
			t.Fatalf("preparando un directorio con nombre .bak: %v", err)
		}
		setModTime(t, viejo, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		setModTime(t, nuevo, time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC))

		got, err := FindNewestBak(dir)
		if err != nil {
			t.Fatalf("FindNewestBak(%s) devolvió error: %v", dir, err)
		}
		if got != nuevo {
			t.Errorf("FindNewestBak = %q, quiero el más nuevo: %q", got, nuevo)
		}
	})

	t.Run("sin .bak el error explica qué dejar en el directorio", func(t *testing.T) {
		dir := t.TempDir()
		_ = writeFile(t, dir, "LEEME.txt")

		if _, err := FindNewestBak(dir); err == nil {
			t.Fatal("quiero error cuando no hay ningún .bak, obtuve nil")
		}
	})

	t.Run("directorio inexistente propaga el error", func(t *testing.T) {
		if _, err := FindNewestBak(filepath.Join(t.TempDir(), "no-existe")); err == nil {
			t.Fatal("quiero error cuando el backup_dir no existe, obtuve nil")
		}
	})
}

func writeFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("creando %s: %v", path, err)
	}
	return path
}

func setModTime(t *testing.T, path string, when time.Time) {
	t.Helper()
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatalf("fijando mtime de %s: %v", path, err)
	}
}

// F7d: el respaldo se publica como asset del Release, así que el error no puede seguir
// mandando a copiarlo "de la PC vieja" a mano: tiene que decir cómo conseguirlo.
func TestElErrorDeRespaldoMandaAlRelease(t *testing.T) {
	_, err := FindNewestBak(t.TempDir())
	if err == nil {
		t.Fatal("una carpeta vacía tiene que dar error")
	}
	if !strings.Contains(err.Error(), "aegis bak") {
		t.Errorf("el error no dice cómo conseguir el respaldo: %v", err)
	}
}
