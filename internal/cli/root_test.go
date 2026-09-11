// © Antony Monge López — Costa Rica — Céd. 604700548
package cli

import (
	"os"
	"path/filepath"
	"testing"

	"aegis-setup/internal/config"
)

// TestConfigPathToUseOrden fija la migración del config.json: el flag manda,
// después el canónico de %APPDATA%\AegisSetup y, si todavía no existe, el viejo
// junto al binario. Un config nuevo se escribe siempre en el canónico.
//
// Cada subtest arma su propio %APPDATA%: si compartieran el temporal, el
// canónico escrito en uno contaminaría al siguiente.
func TestConfigPathToUseOrden(t *testing.T) {
	escribir := func(t *testing.T, p string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	canonico := func(appData string) string {
		return filepath.Join(appData, "AegisSetup", "config.json")
	}

	t.Run("el flag manda sobre todo", func(t *testing.T) {
		appData := t.TempDir()
		t.Setenv("AEGIS_APPDATA", appData)
		exeDir := t.TempDir()

		escribir(t, canonico(appData))
		escribir(t, filepath.Join(exeDir, "config.json"))

		quiero := filepath.Join(t.TempDir(), "otro.json")
		if got := configPathToUse(exeDir, quiero); got != quiero {
			t.Errorf("got %q, quiero el flag %q", got, quiero)
		}
	})

	t.Run("sólo legado existe: se sigue usando", func(t *testing.T) {
		t.Setenv("AEGIS_APPDATA", t.TempDir())
		exeDir := t.TempDir()
		legado := filepath.Join(exeDir, "config.json")
		escribir(t, legado)

		if got := configPathToUse(exeDir, ""); got != legado {
			t.Errorf("got %q, quiero el legado %q", got, legado)
		}
	})

	t.Run("ninguno existe: se apunta al canónico", func(t *testing.T) {
		appData := t.TempDir()
		t.Setenv("AEGIS_APPDATA", appData)

		if got := configPathToUse(t.TempDir(), ""); got != canonico(appData) {
			t.Errorf("got %q, quiero el canónico %q", got, canonico(appData))
		}
	})

	t.Run("ambos existen: gana el canónico", func(t *testing.T) {
		appData := t.TempDir()
		t.Setenv("AEGIS_APPDATA", appData)
		exeDir := t.TempDir()

		escribir(t, canonico(appData))
		escribir(t, filepath.Join(exeDir, "config.json"))

		if got := configPathToUse(exeDir, ""); got != canonico(appData) {
			t.Errorf("got %q, quiero el canónico %q", got, canonico(appData))
		}
	})
}

// TestResolveCfgCargaElPathElegido verifica que resolveCfg realmente le pasa al
// loader el path resuelto (y no otro), que es donde se rompería en silencio.
func TestResolveCfgCargaElPathElegido(t *testing.T) {
	t.Setenv("AEGIS_APPDATA", t.TempDir())

	var visto string
	loader := func(p string) (config.Config, error) {
		visto = p
		return config.Default(), nil
	}

	_, path, err := resolveCfg(t.TempDir(), "", loader)
	if err != nil {
		t.Fatalf("resolveCfg: %v", err)
	}
	if visto != path {
		t.Errorf("el loader recibió %q pero resolveCfg devolvió %q", visto, path)
	}
}

// El perfil se pregunta solo cuando no hay config. Con config ya está dicho en qué
// PC estamos y volver a preguntar (o peor: reescribirla) sería perder lo que el
// operador eligió.
func TestPrimeraVezSoloSinConfig(t *testing.T) {
	dir := t.TempDir()
	falta := filepath.Join(dir, "config.json")
	if !primeraVez(falta) {
		t.Error("sin config en disco no dijo primera vez: el TUI abriría el menú con el perfil dev por defecto")
	}

	if err := os.WriteFile(falta, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if primeraVez(falta) {
		t.Error("con config en disco volvió a decir primera vez")
	}
}
