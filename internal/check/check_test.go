// © Antony Monge López — Costa Rica — Céd. 604700548
package check

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aegis-setup/internal/config"
)

func TestCheckNamedInstanceTCP(t *testing.T) {
	cfg := config.Default()
	cfg.Server = `192.168.2.145\SQLEXPRESS`
	cfg.AppDir = t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel right away so SQL dial doesn't wait

	rs := Run(ctx, cfg, "")
	for _, r := range rs {
		if strings.HasPrefix(r.Name, "TCP") {
			if !r.OK {
				t.Errorf("TCP check falló para instancia con nombre: %s", r.Info)
			}
			if !strings.Contains(r.Info, "SQL Browser") {
				t.Errorf("TCP check debe mencionar SQL Browser para instancias con nombre: %s", r.Info)
			}
		}
	}
}

func TestCheckSIDCAppMissingExe(t *testing.T) {
	cfg := config.Default()
	cfg.AppDir = t.TempDir() // empty dir, no exe

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	rs := Run(ctx, cfg, "")
	found := false
	for _, r := range rs {
		if r.Name == "SIDC app" {
			found = true
			if r.OK {
				t.Errorf("SIDC app debió fallar por falta de ejecutable")
			}
			if !strings.Contains(r.Info, "no encontrado") {
				t.Errorf("mensaje inesperado: %s", r.Info)
			}
		}
	}
	if !found {
		t.Errorf("no se encontró el resultado 'SIDC app'")
	}
}

func TestCheckSIDCAppUnpatchedExe(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.AppDir = dir
	cfg.UseWinAuth = false

	// Crear exe con cadena sin credenciales
	unpatched := append([]byte("MZ..."), setupUtf16le("Provider=MSDASQL.1;Persist Security Info=False;Data Source=SIDC_SQL;Initial Catalog=SIDC")...)
	exePath := filepath.Join(dir, "Sistema Intergrado de Controles y Presupuesto.exe")
	if err := os.WriteFile(exePath, unpatched, 0644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	rs := Run(ctx, cfg, "")
	found := false
	for _, r := range rs {
		if r.Name == "SIDC app" {
			found = true
			if r.OK {
				t.Errorf("SIDC app debió fallar por exe sin credenciales")
			}
			if !strings.Contains(r.Info, "sin credenciales") {
				t.Errorf("mensaje inesperado: %s", r.Info)
			}
		}
	}
	if !found {
		t.Errorf("no se encontró el resultado 'SIDC app'")
	}
}

func setupUtf16le(s string) []byte {
	b := make([]byte, 0, len(s)*2)
	for _, r := range s {
		b = append(b, byte(r), byte(r>>8))
	}
	return b
}

