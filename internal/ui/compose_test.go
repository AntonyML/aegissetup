// © Antony Monge López — Costa Rica — Céd. 604700548
package ui

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"aegis-setup/internal/config"
)

// kitDeDocker es el kit de mentira: un archivo en la raíz y otro en init/, que es la forma
// real del embed (el init/ tiene que salir como carpeta).
func kitDeDocker() fstest.MapFS {
	return fstest.MapFS{
		"docker-compose.yml": &fstest.MapFile{Data: []byte("services:\n  sqlsidc: {}\n")},
		"init/01-sidc.sql":   &fstest.MapFile{Data: []byte("SELECT 1;\n")},
	}
}

// stubCompose deja el kit y su destino bajo control del test, y devuelve el log.
func stubCompose(t *testing.T, origen fs.FS, destino string) *[]string {
	t.Helper()
	prevFS, prevDir := composeFS, composeDir
	t.Cleanup(func() { composeFS, composeDir = prevFS, prevDir })
	composeFS, composeDir = origen, destino
	return &[]string{}
}

// En dev la base vive en Docker, así que el compose lo levanta el operador en esta PC: los
// archivos tienen que quedar en la carpeta del producto. Antes solo existían en el repo, y
// la PC destino —justo la que no tiene el repo— se quedaba sin forma de levantar el motor.
func TestElPasoDeDBInstalaElComposeEnDocker(t *testing.T) {
	destino := t.TempDir()
	stubCompose(t, kitDeDocker(), destino)

	cfg := devCfg()
	cfg.BackupDir = t.TempDir() // vacío: el .bak no está y el paso se corta ahí

	var log []string
	err := runStep(context.Background(), cfg, taskSetupDB, func(string) string { return "" },
		func(s string) { log = append(log, s) })

	// El kit se instala ANTES de buscar el .bak, así que tiene que haber quedado aunque el
	// paso se corte después por no encontrar respaldo.
	for _, f := range []string{"docker-compose.yml", "init/01-sidc.sql"} {
		if _, statErr := os.Stat(filepath.Join(destino, filepath.FromSlash(f))); statErr != nil {
			t.Fatalf("no quedó %s en %s (err del paso: %v): %v", f, destino, err, statErr)
		}
	}
	if !strings.Contains(strings.Join(log, "\n"), "DOCKER OK: docker-compose.yml") {
		t.Errorf("no dijo que instaló el compose: %v", log)
	}
}

// En prod con el SQL instalado en la misma PC (o en un servidor de la red) el compose no
// aplica: dejarle una carpeta con un docker-compose.yml al lado es ruido que después alguien
// levanta creyendo que es parte de la instalación.
func TestEnProdLocalNoSeInstalaElCompose(t *testing.T) {
	destino := t.TempDir()
	stubCompose(t, kitDeDocker(), destino)

	cfg := prodCfg()
	cfg.BackupDir = t.TempDir()

	var log []string
	_ = runStep(context.Background(), cfg, taskSetupDB, func(string) string { return "" },
		func(s string) { log = append(log, s) })

	ents, err := os.ReadDir(destino)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 0 {
		t.Errorf("instaló el compose en prod local: %v", ents)
	}
	if strings.Contains(strings.Join(log, "\n"), "DOCKER") {
		t.Errorf("habló de Docker en prod local: %v", log)
	}
}

// Un fallo al escribir el kit se nombra en la salida, igual que Crystal y los OCX: el
// operador tiene que ver qué archivo no se pudo dejar antes de que el motor no arranque.
func TestElFalloDelComposeSeNombraEnElPaso(t *testing.T) {
	destino := t.TempDir()
	stubCompose(t, kitDeDocker(), destino)
	// Un archivo donde va la carpeta init/: crear el directorio falla.
	if err := os.WriteFile(filepath.Join(destino, "init"), []byte("no soy una carpeta"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := devCfg()
	cfg.BackupDir = t.TempDir()

	var log []string
	_ = runStep(context.Background(), cfg, taskSetupDB, func(string) string { return "" },
		func(s string) { log = append(log, s) })

	if !strings.Contains(strings.Join(log, "\n"), "DOCKER PENDIENTE: init/01-sidc.sql") {
		t.Errorf("el fallo no se nombró en la salida del paso: %v", log)
	}
}

// instalarCompose es la regla sola, sin el paso: en docker instala y en local no.
func TestInstalarComposeSoloAplicaADocker(t *testing.T) {
	destino := t.TempDir()
	stubCompose(t, kitDeDocker(), destino)

	if fallos := instalarCompose(config.DbLocal, nil); len(fallos) != 0 {
		t.Errorf("fallos en local = %v, quiero ninguno", fallos)
	}
	if ents, _ := os.ReadDir(destino); len(ents) != 0 {
		t.Errorf("instaló en local: %v", ents)
	}
	if fallos := instalarCompose(config.DbDocker, nil); len(fallos) != 0 {
		t.Errorf("fallos en docker = %v, quiero ninguno", fallos)
	}
	if _, err := os.Stat(filepath.Join(destino, "docker-compose.yml")); err != nil {
		t.Errorf("no instaló en docker: %v", err)
	}
}
