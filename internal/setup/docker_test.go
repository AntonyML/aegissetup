// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// cero es una marca de tiempo inconfundible: si un archivo la conserva, nadie lo reescribió.
var cero = time.Unix(1000000, 0)

// composeDeMentira es un kit mínimo con un archivo en la raíz y otro en init/, que es la
// forma real del embed (el init/ tiene que salir como carpeta, no aplanado).
func composeDeMentira() fstest.MapFS {
	return fstest.MapFS{
		"docker-compose.yml": &fstest.MapFile{Data: []byte("services:\n  sqlsidc: {}\n")},
		"GUIA-DOCKER.txt":    &fstest.MapFile{Data: []byte("guia\n")},
		".env.example":       &fstest.MapFile{Data: []byte("SA_PASSWORD=xxxx\n")},
		"init/01-sidc.sql":   &fstest.MapFile{Data: []byte("SELECT 1;\n")},
		"init/02-extra.sql":  &fstest.MapFile{Data: []byte("SELECT 2;\n")},
	}
}

// Instalar el kit deja el compose utilizable: los archivos en su lugar y la carpeta init/
// respetada. `docker compose -f <dir>/docker-compose.yml up -d` es el paso que sigue, y
// ese comando lee init/ por ruta relativa al compose.
func TestInstalaElKitDeDocker(t *testing.T) {
	dir := t.TempDir()
	var dicho []string

	fallos := InstallDockerAssets(composeDeMentira(), dir, func(s string) { dicho = append(dicho, s) })
	if len(fallos) != 0 {
		t.Fatalf("fallos = %v, quiero ninguno", fallos)
	}
	for f, quiero := range map[string]string{
		"docker-compose.yml": "services:\n  sqlsidc: {}\n",
		"GUIA-DOCKER.txt":    "guia\n",
		".env.example":       "SA_PASSWORD=xxxx\n",
		"init/01-sidc.sql":   "SELECT 1;\n",
		"init/02-extra.sql":  "SELECT 2;\n",
	} {
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f)))
		if err != nil {
			t.Errorf("%s no quedó instalado: %v", f, err)
			continue
		}
		if string(got) != quiero {
			t.Errorf("%s = %q, quiero %q", f, got, quiero)
		}
	}
	if len(dicho) == 0 {
		t.Error("no dijo qué instaló: el operador no ve que pasó algo")
	}
}

// Repetir la instalación no reescribe lo que ya está igual. La razón no es la velocidad:
// un archivo reescrito en cada corrida es un archivo que nadie puede editar (un operador
// que ajusta el compose a mano pierde el ajuste sin enterarse).
func TestLoQueYaEstaIgualNoSeReescribe(t *testing.T) {
	dir := t.TempDir()
	origen := composeDeMentira()
	InstallDockerAssets(origen, dir, nil)

	// Una marca de tiempo vieja en un archivo que NO debería tocarse.
	guia := filepath.Join(dir, "GUIA-DOCKER.txt")
	if err := os.Chtimes(guia, cero, cero); err != nil {
		t.Fatal(err)
	}
	var dicho []string
	if fallos := InstallDockerAssets(origen, dir, func(s string) { dicho = append(dicho, s) }); len(fallos) != 0 {
		t.Fatalf("fallos = %v, quiero ninguno", fallos)
	}
	fi, err := os.Stat(guia)
	if err != nil {
		t.Fatal(err)
	}
	if !fi.ModTime().Equal(cero) {
		t.Error("reescribió un archivo que ya estaba igual: cualquier ajuste a mano se pierde en silencio")
	}
	if len(dicho) != 0 {
		t.Errorf("dijo %v en una segunda corrida sin cambios: quiero silencio", dicho)
	}
}

// Un compose viejo en la carpeta destino es peor que no tenerlo: el motor levanta con la
// config anterior y el operador mide una máquina que no es la que cree. Se compara
// contenido, no fecha ni tamaño.
func TestUnComposeViejoSeReemplaza(t *testing.T) {
	dir := t.TempDir()
	InstallDockerAssets(composeDeMentira(), dir, nil)

	viejo := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(viejo, []byte("services:\n  sql2014: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var dicho []string
	InstallDockerAssets(composeDeMentira(), dir, func(s string) { dicho = append(dicho, s) })

	got, err := os.ReadFile(viejo)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "sqlsidc") {
		t.Errorf("el compose quedó viejo: %q", got)
	}
	if !strings.Contains(strings.Join(dicho, "\n"), "docker-compose.yml") {
		t.Errorf("no dijo qué reemplazó: %v", dicho)
	}
}

// Sin permiso de escritura (o con la carpeta tomada) el fallo dice qué archivo y dónde, y
// no aborta: el resto del kit se instala igual y el operador ve todo lo que falta de una.
func TestLosFallosSeAcumulanYNombranElArchivo(t *testing.T) {
	dir := t.TempDir()
	// Un archivo en lugar de la carpeta init/: mkdir falla.
	if err := os.WriteFile(filepath.Join(dir, "init"), []byte("no soy una carpeta"), 0o644); err != nil {
		t.Fatal(err)
	}

	fallos := InstallDockerAssets(composeDeMentira(), dir, nil)
	if len(fallos) == 0 {
		t.Fatal("no reportó el fallo de init/")
	}
	unido := strings.Join(fallos, "\n")
	for _, quiero := range []string{"init/01-sidc.sql", dir} {
		if !strings.Contains(unido, quiero) {
			t.Errorf("el fallo no nombra %q: %v", quiero, fallos)
		}
	}
	// El compose de la raíz no depende de init/: tiene que haberse instalado igual.
	if _, err := os.Stat(filepath.Join(dir, "docker-compose.yml")); err != nil {
		t.Errorf("un fallo en init/ abortó el resto del kit: %v", err)
	}
}

// Un binario mal armado (sin el kit embebido) no puede hacer creer que instaló algo.
func TestSinKitNoSeDiceQueInstalo(t *testing.T) {
	fallos := InstallDockerAssets(fstest.MapFS{}, t.TempDir(), nil)
	if len(fallos) == 0 {
		t.Fatal("con el kit vacío dijo que instaló: el motor después no arranca y el operador no sabe por qué")
	}
}
