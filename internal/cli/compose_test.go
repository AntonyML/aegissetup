// © Antony Monge López — Costa Rica — Céd. 604700548
package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"aegis-setup/internal/config"
)

// kitDeDockerCLI es el kit de mentira del embed: raíz + init/.
func kitDeDockerCLI() fstest.MapFS {
	return fstest.MapFS{
		"docker-compose.yml": &fstest.MapFile{Data: []byte("services:\n  sqlsidc: {}\n")},
		"init/01-sidc.sql":   &fstest.MapFile{Data: []byte("SELECT 1;\n")},
	}
}

// stubComposeCLI deja el kit y su destino bajo control del test.
func stubComposeCLI(t *testing.T, origen fs.FS, destino string) {
	t.Helper()
	prevFS, prevDir := composeFS, composeDir
	t.Cleanup(func() { composeFS, composeDir = prevFS, prevDir })
	composeFS, composeDir = origen, destino
}

// El paso 1 del CLI (setup-db) es el que tiene que dejar el compose en la PC: es el comando
// que el operador corre antes de levantar el motor, y en la PC destino el repo no existe.
func TestSetupDBDejaElComposeAntesDeRestaurar(t *testing.T) {
	destino := t.TempDir()
	stubComposeCLI(t, kitDeDockerCLI(), destino)

	cfg := config.Default()
	cfg.BackupDir = t.TempDir() // vacío: no hay .bak y el restore se corta ahí
	cfg.Server = "localhost,1"  // puerto muerto: este test no toca ninguna base real

	var log []string
	err := ejecutarSetupDB(cfg, "", "", "", func(s string) { log = append(log, s) })

	// El kit se instala antes de buscar el respaldo, así que los archivos tienen que estar
	// aunque el paso haya terminado en error por no encontrar .bak.
	for _, f := range []string{"docker-compose.yml", "init/01-sidc.sql"} {
		if _, statErr := os.Stat(filepath.Join(destino, filepath.FromSlash(f))); statErr != nil {
			t.Fatalf("no quedó %s en %s (err del paso: %v): %v", f, destino, err, statErr)
		}
	}
	if !strings.Contains(strings.Join(log, "\n"), "DOCKER OK: docker-compose.yml") {
		t.Errorf("no dijo que dejó el compose: %v", log)
	}
}

// En prod local el compose no aplica: el motor es el SQL Server de la máquina.
func TestSetupDBEnProdLocalNoDejaCompose(t *testing.T) {
	destino := t.TempDir()
	stubComposeCLI(t, kitDeDockerCLI(), destino)

	cfg := config.Default()
	cfg.Env, cfg.DbMode, cfg.UseWinAuth = "prod", config.DbLocal, true
	cfg.BackupDir = t.TempDir()
	cfg.Server = "localhost,1"

	_ = ejecutarSetupDB(cfg, "", "", "", func(string) {})

	if ents, _ := os.ReadDir(destino); len(ents) != 0 {
		t.Errorf("dejó el compose en prod local: %v", ents)
	}
}

// configure es el momento en que el operador dice "esta PC es de pruebas", así que el kit
// tiene que quedar ya en su carpeta: el checklist da el comando para levantar el motor y ese
// comando apunta justo ahí. Si el kit apareciera recién al correr setup-db, el operador
// pegaría un comando que falla porque el archivo todavía no existe.
func TestConfigureDejaElKitDeDocker(t *testing.T) {
	destino := t.TempDir()
	stubComposeCLI(t, kitDeDockerCLI(), destino)

	var salida strings.Builder
	cmd := newConfigureCmd("")
	cmd.SetOut(&salida)
	cmd.SetArgs([]string{"--env", "dev", "--db-mode", "docker", "--out", filepath.Join(t.TempDir(), "config.json")})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("configure: %v", err)
	}

	if _, err := os.Stat(filepath.Join(destino, "docker-compose.yml")); err != nil {
		t.Errorf("configure no dejó el kit en %s: %v\nsalida: %s", destino, err, salida.String())
	}
	if !strings.Contains(salida.String(), "DOCKER OK: docker-compose.yml") {
		t.Errorf("configure no dijo que dejó el kit: %s", salida.String())
	}
}

// Y en prod no lo deja: una carpeta con un docker-compose.yml al lado de una instalación de
// producción es algo que después alguien levanta creyendo que es parte del sistema.
func TestConfigureEnProdNoDejaElKitDeDocker(t *testing.T) {
	destino := t.TempDir()
	stubComposeCLI(t, kitDeDockerCLI(), destino)

	cmd := newConfigureCmd("")
	cmd.SetOut(&strings.Builder{})
	cmd.SetArgs([]string{"--env", "prod", "--db-mode", "local", "--out", filepath.Join(t.TempDir(), "config.json")})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("configure: %v", err)
	}

	if ents, _ := os.ReadDir(destino); len(ents) != 0 {
		t.Errorf("configure dejó el kit en prod local: %v", ents)
	}
}
