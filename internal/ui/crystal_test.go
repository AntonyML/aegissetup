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
	"aegis-setup/internal/setup"
)

// runtimeDeMentira es un runtime chico: un componente COM y una dependencia que solo
// se copia. Con esto la prueba no toca los 43 archivos reales ni el SysWOW64.
func runtimeDeMentira() fstest.MapFS {
	return fstest.MapFS{
		"crviewer.dll": {Data: []byte("runtime")},
		"crpe32.dll":   {Data: []byte("puente odbc")},
	}
}

// El runtime tiene que venir del propio EXE: si crystalFS quedara apuntando a otra
// cosa, la PC limpia se quedaría sin reportes y nadie se enteraría hasta imprimir.
func TestElRuntimePorDefectoEsElEmbebido(t *testing.T) {
	for _, f := range setup.CrystalCOM {
		if _, err := fs.Stat(crystalFS, f); err != nil {
			t.Errorf("el runtime embebido no trae %s: %v", f, err)
		}
	}
	if crystalDir != setup.SysWOW64 {
		t.Errorf("crystalDir = %q, quiero %q", crystalDir, setup.SysWOW64)
	}
	if registrarCOM == nil {
		t.Error("registrarCOM quedó en nil: no se registraría ningún componente")
	}
}

// El paso de Setup App tiene que copiar el runtime y REGISTRAR los componentes: un
// DLL copiado sin registrar deja el reporte tirando el error de COM, que es
// exactamente el síntoma que este paso existe para matar.
func TestInstalaElRuntimeEnElDestinoYRegistra(t *testing.T) {
	destino := t.TempDir()
	registrados := stubCrystal(t, runtimeDeMentira(), destino)

	var log []string
	instalarCrystal(func(s string) { log = append(log, s) })

	if _, err := fs.Stat(os.DirFS(destino), "crviewer.dll"); err != nil {
		t.Fatalf("no se copió crviewer.dll a %s: %v", destino, err)
	}
	if len(*registrados) != 1 || filepath.Base((*registrados)[0]) != "crviewer.dll" {
		t.Errorf("registró %v, quiero solo crviewer.dll (el único componente COM del runtime)", *registrados)
	}
	linea := strings.Join(log, "\n")
	if !strings.Contains(linea, "CRYSTAL: 2 copiados") {
		t.Errorf("el log no resume lo copiado:\n%s", linea)
	}
	// crpe32.dll solo se copia, pero los otros 3 componentes COM no están en el
	// runtime de mentira: tienen que quedar como pendientes, no en silencio.
	if !strings.Contains(linea, "3 pendientes") {
		t.Errorf("el log no avisa de los componentes que faltan:\n%s", linea)
	}
}

// Un runtime vacío o un destino sin permiso no puede tumbar Setup App: la app de SIDC
// ya quedó instalada y abortar acá obligaría a repetir todo por un problema de
// permisos que se arregla reabriendo Aegis como administrador.
func TestElRuntimeVacioEsPendienteYNoAborta(t *testing.T) {
	stubCrystal(t, fstest.MapFS{}, t.TempDir())

	var log []string
	instalarCrystal(func(s string) { log = append(log, s) })

	linea := strings.Join(log, "\n")
	if !strings.Contains(linea, "CRYSTAL PENDIENTE:") {
		t.Errorf("un runtime vacío no se reportó como pendiente:\n%s", linea)
	}
}

// stubCrystal apunta el origen, el destino y el registro a un lugar de mentira y
// devuelve la lista de lo registrado.
func stubCrystal(t *testing.T, origen fs.FS, destino string, orden ...*[]string) *[]string {
	t.Helper()
	var ordenPtr *[]string
	if len(orden) > 0 {
		ordenPtr = orden[0]
	}
	fsPrevio, dirPrevio, regPrevio := crystalFS, crystalDir, registrarCOM
	registrados := &[]string{}
	crystalFS, crystalDir = origen, destino
	// Igual que el stub de los OCX: el regsvr32 de 32 bits es el mismo para los dos
	// instaladores, así que cada uno reconoce lo suyo por la carpeta de destino y el
	// orden en que se llamen los stubs no cambia el resultado.
	registrarCOM = func(ruta string) error {
		if !strings.HasPrefix(ruta, destino) {
			if regPrevio != nil {
				return regPrevio(ruta)
			}
			return nil
		}
		*registrados = append(*registrados, ruta)
		if ordenPtr != nil {
			*ordenPtr = append(*ordenPtr, "crystal")
		}
		return nil
	}
	t.Cleanup(func() { crystalFS, crystalDir, registrarCOM = fsPrevio, dirPrevio, regPrevio })
	return registrados
}

// El paso de Setup App tiene que instalar Crystal: es la razón de existir de F6, y sin
// esta prueba borrar la llamada no rompe nada.
func TestSetupAppInstalaElRuntimeDeCrystal(t *testing.T) {
	destino := t.TempDir()
	registrados := stubCrystal(t, runtimeDeMentira(), destino)
	stubOCX(t, kitDeMentira(), t.TempDir())
	stubDSN(t, nil)

	cfg := prodCfg()
	cfg.AppDir, cfg.LegacyDir = t.TempDir(), t.TempDir()

	var log []string
	if err := runStep(context.Background(), cfg, taskSetupApp, func(string) string { return "" },
		func(s string) { log = append(log, s) }); err != nil {
		t.Fatalf("Setup App falló: %v", err)
	}
	if _, err := fs.Stat(os.DirFS(destino), "crviewer.dll"); err != nil {
		t.Fatalf("Setup App no instaló el runtime en %s: %v", destino, err)
	}
	if len(*registrados) == 0 {
		t.Error("Setup App copió el runtime pero no registró ningún componente COM")
	}
}

// El DSN, los OCX, Crystal y el parche _DOCKER van en ese orden. El orden importa:
// es el log que lee el operador cuando algo falla a mitad de camino, y el parche de
// _DOCKER tiene que ver un DSN ya escrito.
func TestElRuntimeSeInstalaDespuesDeLosOCXYAntesDelParche(t *testing.T) {
	var orden []string
	stubOCX(t, kitDeMentira(), t.TempDir(), &orden)
	stubCrystal(t, runtimeDeMentira(), t.TempDir(), &orden)
	stubDSN(t, &orden)
	stubParche(t, &orden)

	cfg := devCfg()
	cfg.AppDir, cfg.LegacyDir = t.TempDir(), t.TempDir()

	var log []string
	err := runStep(context.Background(), cfg, taskSetupApp, func(string) string { return "clave" },
		func(s string) { log = append(log, s) })
	if err != nil {
		t.Fatalf("Setup App falló: %v", err)
	}
	if got := sinRepetir(orden); !iguales(got, []string{"dsn", "ocx", "crystal", "parche"}) {
		t.Errorf("orden de los pasos = %v, quiero [dsn ocx crystal parche]", got)
	}
}

// sinRepetir deja el primer paso de cada tramo: los 11 controles y los 4 componentes se
// registran de a uno, así que el orden real llega con el mismo paso repetido.
func sinRepetir(xs []string) []string {
	var out []string
	for i, x := range xs {
		if i == 0 || xs[i-1] != x {
			out = append(out, x)
		}
	}
	return out
}

func iguales(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// stubDSN evita que la prueba escriba el DSN en el registro de la máquina.
func stubDSN(t *testing.T, orden *[]string) {
	t.Helper()
	previo := escribirDSN
	escribirDSN = func(config.Config, string, bool, func(string)) error {
		if orden != nil {
			*orden = append(*orden, "dsn")
		}
		return nil
	}
	t.Cleanup(func() { escribirDSN = previo })
}

// stubParche evita que la prueba reescriba el .exe de la app de verdad.
func stubParche(t *testing.T, orden *[]string) {
	t.Helper()
	previo := parcheDocker
	parcheDocker = func(string, string, string, func(string)) error {
		*orden = append(*orden, "parche")
		return nil
	}
	t.Cleanup(func() { parcheDocker = previo })
}
