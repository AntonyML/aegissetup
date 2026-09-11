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
	"aegis-setup/internal/setup"
)

// kitDeMentira es el kit embebido en chico: dos controles registrables y una
// dependencia que solo se copia. Con esto la prueba no toca los 22 archivos reales ni
// el SysWOW64 de la máquina.
func kitDeMentira() fstest.MapFS {
	m := fstest.MapFS{}
	for _, f := range setup.RequiredOCX[:2] {
		m[f] = &fstest.MapFile{Data: []byte("kit")}
	}
	m[setup.SupportFiles[0]] = &fstest.MapFile{Data: []byte("kit")}
	return m
}

// stubOCXCLI apunta el origen, el destino y el registro de los controles a un lugar de
// mentira y devuelve lo que se registró.
//
// El registro delega en el que estuviera puesto cuando la ruta no cae en su destino: los
// dos instaladores del paquete (Crystal y OCX) comparten el regsvr32 de 32 bits, así que
// cada uno reconoce lo suyo por la carpeta y el orden en que se llamen los stubs no
// cambia el resultado.
func stubOCXCLI(t *testing.T, origen fs.FS, destino string, orden ...*[]string) *[]string {
	t.Helper()
	var ordenPtr *[]string
	if len(orden) > 0 {
		ordenPtr = orden[0]
	}
	fsPrevio, dirPrevio, regPrevio := ocxFS, ocxDir, registrarCOM
	registrados := &[]string{}
	ocxFS, ocxDir = origen, destino
	registrarCOM = func(ruta string) error {
		if !strings.HasPrefix(ruta, destino) {
			if regPrevio != nil {
				return regPrevio(ruta)
			}
			return nil
		}
		*registrados = append(*registrados, ruta)
		if ordenPtr != nil {
			*ordenPtr = append(*ordenPtr, "ocx")
		}
		return nil
	}
	t.Cleanup(func() { ocxFS, ocxDir, registrarCOM = fsPrevio, dirPrevio, regPrevio })
	return registrados
}

// El kit tiene que venir del propio EXE: en una PC limpia no hay carpeta de la PC vieja,
// y sin estos archivos la app abre y falla con error 339 al entrar a una pantalla.
func TestElKitOCXPorDefectoEsElEmbebido(t *testing.T) {
	for _, f := range setup.RequiredOCX {
		if _, err := fs.Stat(ocxFS, f); err != nil {
			t.Errorf("el kit embebido no trae %s: %v", f, err)
		}
	}
	if ocxDir != setup.SysWOW64 {
		t.Errorf("ocxDir = %q, quiero %q (VB6 es de 32 bits)", ocxDir, setup.SysWOW64)
	}
}

func TestInstalarOCXDevuelveLoPendiente(t *testing.T) {
	destino := t.TempDir()
	registrados := stubOCXCLI(t, kitDeMentira(), destino)

	pendientes := InstalarOCX("", func(string) {})

	if _, err := os.Stat(filepath.Join(destino, setup.RequiredOCX[0])); err != nil {
		t.Fatalf("no se copió %s a %s: %v", setup.RequiredOCX[0], destino, err)
	}
	if len(*registrados) != 2 {
		t.Errorf("registró %v, quiero los 2 controles del kit de mentira", *registrados)
	}
	// Un kit incompleto tiene que devolver lo que falta, no una instalación completa.
	if len(pendientes) != len(setup.RequiredOCX)-2 {
		t.Errorf("pendientes = %v, quiero %d", pendientes, len(setup.RequiredOCX)-2)
	}
}

// Este es el requisito externo que F7 elimina: antes, una PC limpia exigía copiar 22
// archivos a mano desde la PC vieja antes de poder instalar.
func TestElKitNoDependeDeLaCarpetaDeLaPCVieja(t *testing.T) {
	destino := t.TempDir()
	registrados := stubOCXCLI(t, ocxFS, destino)

	if pendientes := InstalarOCX(t.TempDir(), func(string) {}); len(pendientes) != 0 {
		t.Fatalf("con la carpeta de la PC vieja vacía no puede faltar nada: %v", pendientes)
	}
	for _, f := range append(append([]string{}, setup.RequiredOCX...), setup.SupportFiles...) {
		if _, err := os.Stat(filepath.Join(destino, f)); err != nil {
			t.Errorf("no quedó %s en %s: %v", f, destino, err)
		}
	}
	if len(*registrados) != len(setup.RequiredOCX) {
		t.Errorf("registró %d controles, esperaba %d", len(*registrados), len(setup.RequiredOCX))
	}
}

// El paso 2 tiene que instalar los controles: es el camino desatendido, y sin esta
// prueba borrar la llamada no rompe nada.
func TestSetupAppInstalaLosControlesDelBinario(t *testing.T) {
	destino := t.TempDir()
	registrados := stubOCXCLI(t, ocxFS, destino)
	stubCrystalCLI(t, fstest.MapFS{"crviewer.dll": {Data: []byte("runtime")}}, t.TempDir())
	stubPaso2(t, nil)

	cfg := config.Default()
	cfg.UseWinAuth = true // prod: no hay parche _DOCKER
	cfg.AppDir = t.TempDir()
	// legacy_dir apunta a una carpeta vacía a propósito: es el caso de la PC limpia.
	cfg.LegacyDir = t.TempDir()

	var log []string
	if err := ejecutarSetupApp(cfg, setupAppOpts{}, func(s string) { log = append(log, s) }); err != nil {
		t.Fatalf("Setup App falló: %v", err)
	}

	if _, err := os.Stat(filepath.Join(destino, "MSCOMCTL.OCX")); err != nil {
		t.Fatalf("Setup App no instaló los controles en %s: %v", destino, err)
	}
	if len(*registrados) != len(setup.RequiredOCX) {
		t.Errorf("registró %d controles, esperaba %d", len(*registrados), len(setup.RequiredOCX))
	}
	if strings.Contains(strings.Join(log, "\n"), "OCX PENDIENTE:") {
		t.Errorf("no debería quedar ningún control pendiente:\n%s", strings.Join(log, "\n"))
	}
}
