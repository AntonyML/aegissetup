// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// kitOCX arma un origen con los 22 archivos del kit (11 requeridos + 11 de soporte),
// como el que viaja dentro del ejecutable.
func kitOCX() fstest.MapFS {
	m := fstest.MapFS{}
	for _, f := range append(append([]string{}, RequiredOCX...), SupportFiles...) {
		m[f] = &fstest.MapFile{Data: []byte("kit:" + f)}
	}
	return m
}

// origenBinario es el origen normal: todo sale del binario.
func origenBinario(m fs.FS) OrigenOCX { return OrigenOCX{Binario: m} }

// registrarFalso anota los registros sin llamar a regsvr32.
func registrarFalso(t *testing.T, registrados *[]string) func(string) error {
	t.Helper()
	return func(ruta string) error {
		*registrados = append(*registrados, ruta)
		return nil
	}
}

func TestInstallOCXCopiaEInstalaTodoElKit(t *testing.T) {
	dir := t.TempDir()
	var registrados []string
	fallos := InstallOCX(origenBinario(kitOCX()), dir, registrarFalso(t, &registrados), func(string) {})

	if len(fallos) != 0 {
		t.Fatalf("el kit completo no debería fallar: %v", fallos)
	}
	for _, f := range append(append([]string{}, RequiredOCX...), SupportFiles...) {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("no quedó %s en %s: %v", f, dir, err)
		}
	}
}

// Los 11 requeridos son los que dan error 339: van registrados. Los 11 de soporte son
// dependencias: copiarlos alcanza, y registrarlos de más ensucia el registro.
func TestInstallOCXSoloRegistraLosRequeridos(t *testing.T) {
	var registrados []string
	InstallOCX(origenBinario(kitOCX()), t.TempDir(), registrarFalso(t, &registrados), func(string) {})

	if len(registrados) != len(RequiredOCX) {
		t.Fatalf("registró %d archivos, esperaba %d: %v", len(registrados), len(RequiredOCX), registrados)
	}
	for _, r := range registrados {
		if contiene(SupportFiles, filepath.Base(r)) {
			t.Errorf("registró un archivo de soporte, que solo se copia: %s", r)
		}
	}
}

func TestInstallOCXNoTocaLoQueYaEstaIgual(t *testing.T) {
	dir := t.TempDir()
	if fallos := InstallOCX(origenBinario(kitOCX()), dir, func(string) error { return nil }, func(string) {}); len(fallos) != 0 {
		t.Fatalf("primera corrida con fallos: %v", fallos)
	}

	// Segunda corrida: no se copia ni un archivo, pero se registra igual. Que el
	// archivo esté no prueba que esté registrado.
	var salida []string
	InstallOCX(origenBinario(kitOCX()), dir, func(string) error { return nil }, func(s string) {
		salida = append(salida, s)
	})
	log := strings.Join(salida, "\n")
	for _, malo := range []string{"OCX OK:", "SOPORTE OK:"} {
		if strings.Contains(log, malo) {
			t.Errorf("volvió a copiar en la segunda corrida:\n%s", log)
		}
	}
	if n := strings.Count(log, "OCX YA ESTABA:"); n != len(RequiredOCX) {
		t.Errorf("la segunda corrida tiene que reportar %d controles ya instalados, reportó %d:\n%s",
			len(RequiredOCX), n, log)
	}
}

// Mismo caso que en Crystal: dos versiones distintas de un OCX suelen pesar lo mismo,
// y el síntoma (error 339 en una pantalla puntual) aparece lejos de la instalación.
func TestInstallOCXReemplazaUnoDistintoDelMismoTamano(t *testing.T) {
	dir := t.TempDir()
	Nombre := RequiredOCX[0]
	viejo := []byte("old:" + Nombre)
	if len(viejo) != len("kit:"+Nombre) {
		t.Fatalf("el texto de prueba tiene que medir lo mismo que el del kit (%d)", len(viejo))
	}
	if err := os.WriteFile(filepath.Join(dir, Nombre), viejo, 0o644); err != nil {
		t.Fatal(err)
	}

	InstallOCX(origenBinario(kitOCX()), dir, func(string) error { return nil }, func(string) {})

	got, err := os.ReadFile(filepath.Join(dir, Nombre))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "kit:"+Nombre {
		t.Errorf("no reemplazó el OCX viejo: quedó %q", got)
	}
}

// legacy_dir sobrevive para el control que no venga en el binario. Es el caso de un
// kit incompleto o de un control que la PC vieja tenía más nuevo.
func TestInstallOCXBuscaEnLaCarpetaLegacyLoQueNoVieneEnElBinario(t *testing.T) {
	falta := RequiredOCX[0]
	parcial := kitOCX()
	delete(parcial, falta)

	carpeta := t.TempDir()
	if err := os.WriteFile(filepath.Join(carpeta, falta), []byte("de la pc vieja"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var registrados []string

	fallos := InstallOCX(OrigenOCX{Binario: parcial, Carpeta: carpeta}, dir, registrarFalso(t, &registrados), func(string) {})

	if len(fallos) != 0 {
		t.Fatalf("la carpeta legacy debería cubrir el faltante: %v", fallos)
	}
	got, err := os.ReadFile(filepath.Join(dir, falta))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "de la pc vieja" {
		t.Errorf("%s quedó con %q, esperaba el de la carpeta legacy", falta, got)
	}
}

// El binario gana sobre el disco: un archivo suelto y viejo en la carpeta de la PC
// vieja no puede degradar una instalación que ya es autocontenida.
func TestInstallOCXPrefiereElBinarioAntesQueLaCarpeta(t *testing.T) {
	Nombre := RequiredOCX[0]
	carpeta := t.TempDir()
	if err := os.WriteFile(filepath.Join(carpeta, Nombre), []byte("de la pc vieja"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()

	InstallOCX(OrigenOCX{Binario: kitOCX(), Carpeta: carpeta}, dir, func(string) error { return nil }, func(string) {})

	got, err := os.ReadFile(filepath.Join(dir, Nombre))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "kit:"+Nombre {
		t.Errorf("%s quedó con %q y tenía que venir del binario", Nombre, got)
	}
}

// Un binario compilado sin el kit (el linker descarta lo que nadie referencia) no
// puede reportar una instalación completa: hay que decir cuáles faltan.
func TestInstallOCXSinOrigenReportaLosRequeridos(t *testing.T) {
	var salida []string
	fallos := InstallOCX(OrigenOCX{}, t.TempDir(), func(string) error { return nil }, func(s string) {
		salida = append(salida, s)
	})

	if len(fallos) != len(RequiredOCX) {
		t.Fatalf("reportó %d fallos, esperaba %d: %v", len(fallos), len(RequiredOCX), fallos)
	}
	for _, f := range RequiredOCX {
		if !strings.Contains(strings.Join(fallos, " "), f) {
			t.Errorf("%s no aparece en los fallos: %v", f, fallos)
		}
	}
	// Los de soporte no son fallo: sin ellos la app abre igual.
	for _, f := range SupportFiles {
		if strings.Contains(strings.Join(fallos, " "), f) {
			t.Errorf("%s es opcional y no puede ser un fallo: %v", f, fallos)
		}
	}
	if !strings.Contains(strings.Join(salida, "\n"), SupportFiles[0]) {
		t.Error("los de soporte faltantes tienen que seguir avisándose")
	}
}

func TestInstallOCXNoAbortaSiUnRegistroFalla(t *testing.T) {
	roto := RequiredOCX[2]
	var registrados []string
	fallos := InstallOCX(origenBinario(kitOCX()), t.TempDir(), func(ruta string) error {
		if filepath.Base(ruta) == roto {
			return os.ErrPermission
		}
		registrados = append(registrados, ruta)
		return nil
	}, func(string) {})

	if len(fallos) != 1 || !strings.Contains(fallos[0], roto) {
		t.Fatalf("esperaba un solo fallo de %s: %v", roto, fallos)
	}
	if len(registrados) != len(RequiredOCX)-1 {
		t.Errorf("siguió con los demás: registró %d de %d", len(registrados), len(RequiredOCX)-1)
	}
}

func TestFaltanOCXEnConElBinarioCompletoEsVacio(t *testing.T) {
	if faltan := FaltanOCXEn(origenBinario(kitOCX())); len(faltan) != 0 {
		t.Errorf("el kit completo no tiene faltantes: %v", faltan)
	}
}

func TestFaltanOCXEnSinOrigenLosListaTodos(t *testing.T) {
	faltan := FaltanOCXEn(OrigenOCX{})
	if len(faltan) != len(RequiredOCX) {
		t.Fatalf("esperaba %d faltantes: %v", len(RequiredOCX), faltan)
	}
}

func contiene(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
