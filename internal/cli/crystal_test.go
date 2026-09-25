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

// El runtime tiene que venir del propio EXE: si crystalFS quedara apuntando a otra
// cosa, la PC limpia se quedaría sin reportes y nadie se enteraría hasta imprimir.
func TestElRuntimeDeCrystalPorDefectoEsElEmbebido(t *testing.T) {
	for _, f := range setup.CrystalCOM {
		if _, err := fs.Stat(crystalFS, f); err != nil {
			t.Errorf("el runtime embebido no trae %s: %v", f, err)
		}
	}
	if crystalDir != setup.SysWOW64 {
		t.Errorf("crystalDir = %q, quiero %q", crystalDir, setup.SysWOW64)
	}
}

// InstalarCrystal devuelve lo que quedó pendiente en vez de abortar: mismo criterio
// que los OCX, porque una dependencia que no se pudo poner no invalida el resto.
func TestInstalarCrystalDevuelveLoPendiente(t *testing.T) {
	destino := t.TempDir()
	registrados := stubCrystalCLI(t, fstest.MapFS{
		"crviewer.dll": {Data: []byte("runtime")},
		"crpe32.dll":   {Data: []byte("puente odbc")},
	}, destino)

	var log []string
	pendientes := InstalarCrystal(func(s string) { log = append(log, s) })

	if _, err := fs.Stat(os.DirFS(destino), "crpe32.dll"); err != nil {
		t.Fatalf("no se copió crpe32.dll a %s: %v", destino, err)
	}
	if len(*registrados) != 1 || filepath.Base((*registrados)[0]) != "crviewer.dll" {
		t.Errorf("registró %v, quiero solo crviewer.dll", *registrados)
	}
	// Los otros 3 componentes COM no están en el runtime de mentira: tienen que
	// volver como pendientes y no desaparecer.
	if len(pendientes) != 3 {
		t.Errorf("pendientes = %v, quiero 3", pendientes)
	}
	if len(log) == 0 {
		t.Error("no avisó nada de lo que copió")
	}
}

// Un binario compilado sin el runtime (o un destino sin permisos) no puede tirar un
// panic en la cara del operador: es un pendiente más.
func TestInstalarCrystalConRuntimeVacioEsUnPendiente(t *testing.T) {
	stubCrystalCLI(t, fstest.MapFS{}, t.TempDir())

	pendientes := InstalarCrystal(func(string) {})
	if len(pendientes) != 1 {
		t.Fatalf("pendientes = %v, quiero un error de runtime vacío", pendientes)
	}
}

// stubCrystalCLI apunta el origen, el destino y el registro a un lugar de mentira y
// devuelve la lista de lo registrado.
func stubCrystalCLI(t *testing.T, origen fs.FS, destino string, orden ...*[]string) *[]string {
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

// El paso 2 tiene que instalar Crystal: es la razón de existir de F6, y el CLI es el
// camino desatendido. Sin esta prueba, borrar la llamada no rompe nada.
func TestSetupAppInstalaElRuntimeDeCrystal(t *testing.T) {
	destino := t.TempDir()
	registrados := stubCrystalCLI(t, fstest.MapFS{
		"crviewer.dll": {Data: []byte("runtime")},
		"crpe32.dll":   {Data: []byte("puente odbc")},
	}, destino)
	stubOCXCLI(t, kitDeMentira(), t.TempDir())
	stubPaso2(t, nil)

	cfg := config.Default()
	cfg.UseWinAuth = true // prod: no hay parche _DOCKER
	cfg.AppDir, cfg.LegacyDir = t.TempDir(), t.TempDir()

	var log []string
	if err := ejecutarSetupApp(cfg, setupAppOpts{}, func(s string) { log = append(log, s) }); err != nil {
		t.Fatalf("Setup App falló: %v", err)
	}
	if _, err := fs.Stat(os.DirFS(destino), "crviewer.dll"); err != nil {
		t.Fatalf("Setup App no instaló el runtime en %s: %v", destino, err)
	}
	if len(*registrados) == 0 {
		t.Error("Setup App copió el runtime pero no registró ningún componente COM")
	}
}

// El orden del paso 2 es DSN -> OCX -> Crystal -> parche _DOCKER. El parche reescribe
// el .exe de la app, así que tiene que ver el entorno ya preparado.
func TestElRuntimeSeInstalaAntesDelParcheDocker(t *testing.T) {
	var orden []string
	stubOCXCLI(t, kitDeMentira(), t.TempDir(), &orden)
	stubCrystalCLI(t, fstest.MapFS{"crviewer.dll": {Data: []byte("runtime")}}, t.TempDir(), &orden)
	stubPaso2(t, &orden)

	cfg := config.Default() // dev: UseWinAuth=false, así el parche aplica
	cfg.AppDir, cfg.LegacyDir = t.TempDir(), t.TempDir()

	if err := ejecutarSetupApp(cfg, setupAppOpts{appPass: "clave", patch: true}, func(string) {}); err != nil {
		t.Fatalf("Setup App falló: %v", err)
	}
	if got := sinRepetir(orden); !mismoOrden(got, []string{"dsn", "ocx", "crystal", "parche"}) {
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

func mismoOrden(a, b []string) bool {
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

// Parchear con clave vacía deja un _DOCKER.exe que arranca y falla al conectar: peor
// que no generarlo, porque parece instalado.
func TestElParcheDockerSinClaveNoSeHace(t *testing.T) {
	stubCrystalCLI(t, fstest.MapFS{"crviewer.dll": {Data: []byte("runtime")}}, t.TempDir())
	stubOCXCLI(t, kitDeMentira(), t.TempDir())
	var orden []string
	stubPaso2(t, &orden)

	cfg := config.Default()
	cfg.AppDir, cfg.LegacyDir = t.TempDir(), t.TempDir()

	err := ejecutarSetupApp(cfg, setupAppOpts{patch: true}, func(string) {})
	if err == nil {
		t.Fatal("parcheó _DOCKER sin clave")
	}
	for _, paso := range orden {
		if paso == "parche" {
			t.Fatal("llamó al parche aunque no había clave")
		}
	}
}

// stubPaso2 evita que la prueba escriba el DSN en el registro o reescriba el .exe de
// la app, y anota en qué orden pasaron las cosas.
func stubPaso2(t *testing.T, orden *[]string) {
	t.Helper()
	dsnPrevio, parchePrevio, pdfPrevio := escribirDSN, parcheDocker, setupPrintToPDF
	escribirDSN = func(config.Config, string, bool, func(string)) error {
		if orden != nil {
			*orden = append(*orden, "dsn")
		}
		return nil
	}
	parcheDocker = func(string, string, string, func(string)) error {
		if orden != nil {
			*orden = append(*orden, "parche")
		}
		return nil
	}
	setupPrintToPDF = func(func(string)) error {
		return nil
	}
	t.Cleanup(func() { escribirDSN, parcheDocker, setupPrintToPDF = dsnPrevio, parchePrevio, pdfPrevio })
}
