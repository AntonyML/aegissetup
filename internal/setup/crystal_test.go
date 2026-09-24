// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
)

// runtimeFalso arma un runtime mínimo con la forma real del embed: archivos planos
// en la raíz, los 4 componentes COM y un par de DLL que solo se copian.
func runtimeFalso() fstest.MapFS {
	return fstest.MapFS{
		"crpe32.dll":   {Data: []byte("motor de impresión")},
		"crviewer.dll": {Data: []byte("visor")},
		"craxdrt.dll":  {Data: []byte("rdc")},
		"craxddrt.dll": {Data: []byte("rdc directo")},
		"Crystl32.OCX": {Data: []byte("control")},
		"u2fxls.dll":   {Data: []byte("export a excel")},
	}
}

// registro cuenta qué se pidió registrar y puede fingir una falla.
type registro struct {
	paths []string
	err   error
}

func (r *registro) fn(p string) error {
	r.paths = append(r.paths, p)
	return r.err
}

func sinRegistrar(string) error { return nil }

func soloNombres(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		out = append(out, filepath.Base(p))
	}
	sort.Strings(out)
	return out
}

func leerDe(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no se pudo leer %s: %v", path, err)
	}
	return string(b)
}

// El punto de F6: en una PC limpia el runtime sale del propio binario, no de una
// carpeta con archivos sueltos ni del Setup original de Crystal.
func TestInstallCrystalCopiaTodoElRuntime(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "syswow64")

	res, err := InstallCrystal(runtimeFalso(), dst, sinRegistrar, func(string) {})
	if err != nil {
		t.Fatalf("InstallCrystal: %v", err)
	}
	if len(res.Copiados) != 6 {
		t.Errorf("copiados = %v, quiero los 6 del runtime", res.Copiados)
	}
	if len(res.YaEstaban) != 0 {
		t.Errorf("ya estaban = %v, con el destino vacío no había ninguno", res.YaEstaban)
	}
	if len(res.Fallos) != 0 {
		t.Errorf("fallos = %v, quiero ninguno", res.Fallos)
	}
	for name, f := range runtimeFalso() {
		if got := leerDe(t, filepath.Join(dst, name)); got != string(f.Data) {
			t.Errorf("%s quedó con %q, quiero %q", name, got, f.Data)
		}
	}
}

// La carpeta destino puede no existir: en una PC recién instalada no existe nada.
func TestInstallCrystalCreaElDestino(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "no", "existe", "todavía")

	if _, err := InstallCrystal(runtimeFalso(), dst, sinRegistrar, func(string) {}); err != nil {
		t.Fatalf("InstallCrystal: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "crpe32.dll")); err != nil {
		t.Errorf("no se creó el destino: %v", err)
	}
}

// Instalación completa = 24 MB. Si correr Setup App dos veces los reescribiera, el
// segundo intento sería lento al pedo y además rompería DLL en uso por la app.
func TestInstallCrystalNoReCopiaLoQueYaEsta(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "syswow64")
	src := runtimeFalso()

	if _, err := InstallCrystal(src, dst, sinRegistrar, func(string) {}); err != nil {
		t.Fatalf("primera pasada: %v", err)
	}
	res, err := InstallCrystal(src, dst, sinRegistrar, func(string) {})
	if err != nil {
		t.Fatalf("segunda pasada: %v", err)
	}
	if len(res.Copiados) != 0 {
		t.Errorf("segunda pasada copió %v, quiero nada", res.Copiados)
	}
	if len(res.YaEstaban) != 6 {
		t.Errorf("ya estaban = %v, quiero los 6", res.YaEstaban)
	}
}

// Un archivo del mismo tamaño pero distinto contenido es el caso que un chequeo por
// tamaño deja pasar: la PC queda con una DLL de otra versión y el síntoma aparece
// mucho después, al generar un reporte.
func TestInstallCrystalReemplazaUnoDistintoDelMismoTamano(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "syswow64")
	src := runtimeFalso()

	// Instalación completa y después el archivo viejo, cambiado, en su lugar.
	if _, err := InstallCrystal(src, dst, sinRegistrar, func(string) {}); err != nil {
		t.Fatal(err)
	}
	// Mismo largo que "motor de impresión", contenido distinto.
	if err := os.WriteFile(filepath.Join(dst, "crpe32.dll"), []byte("motor de impresion"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := InstallCrystal(src, dst, sinRegistrar, func(string) {})
	if err != nil {
		t.Fatalf("InstallCrystal: %v", err)
	}
	if len(res.Copiados) != 1 || res.Copiados[0] != "crpe32.dll" {
		t.Fatalf("copiados = %v, quiero solo crpe32.dll", res.Copiados)
	}
	if got := leerDe(t, filepath.Join(dst, "crpe32.dll")); got != "motor de impresión" {
		t.Errorf("quedó %q, quiero el del runtime", got)
	}
}

// Solo los 4 del LEEME llevan registro: el resto son dependencias que con existir
// en SysWOW64 ya sirven. Registrar de más es tan malo como de menos: algunos de
// estos DLL no tienen DllRegisterServer y regsvr32 devolvería error.
func TestInstallCrystalRegistraLosCuatroComponentesCOM(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "syswow64")
	reg := &registro{}

	res, err := InstallCrystal(runtimeFalso(), dst, reg.fn, func(string) {})
	if err != nil {
		t.Fatalf("InstallCrystal: %v", err)
	}
	quiero := []string{"Crystl32.OCX", "craxddrt.dll", "craxdrt.dll", "crviewer.dll"}
	if got := soloNombres(reg.paths); strings.Join(got, ",") != strings.Join(quiero, ",") {
		t.Errorf("registró %v, quiero %v", got, quiero)
	}
	for _, p := range reg.paths {
		if filepath.Dir(p) != dst {
			t.Errorf("registró %q, quiero el archivo del destino (%s)", p, dst)
		}
	}
	if len(res.Registrados) != 4 {
		t.Errorf("registrados = %v, quiero 4", res.Registrados)
	}
}

// Registrar se repite en cada corrida a propósito: que el archivo exista no prueba
// que esté registrado (lo pudo copiar alguien a mano), y el registro COM es
// idempotente. Es la única forma de no dejar un "presente pero sin registrar".
func TestInstallCrystalRegistraAunqueYaEstuvieran(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "syswow64")
	src := runtimeFalso()
	reg := &registro{}

	if _, err := InstallCrystal(src, dst, reg.fn, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallCrystal(src, dst, reg.fn, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if len(reg.paths) != 8 {
		t.Errorf("registró %d veces, quiero 8 (4 por corrida)", len(reg.paths))
	}
}

// Si un archivo no se puede escribir (típico sin elevación), el resto se copia
// igual y el que falló se reporta: abortar en el primero dejaría al operador sin
// saber cuántos más van a fallar.
func TestInstallCrystalSigueSiUnArchivoFalla(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "syswow64")
	if err := os.MkdirAll(filepath.Join(dst, "u2fxls.dll"), 0o755); err != nil {
		t.Fatal(err)
	}

	res, err := InstallCrystal(runtimeFalso(), dst, sinRegistrar, func(string) {})
	if err != nil {
		t.Fatalf("InstallCrystal devolvió error de programa: %v", err)
	}
	if len(res.Fallos) != 1 || !strings.Contains(res.Fallos[0], "u2fxls.dll") {
		t.Errorf("fallos = %v, quiero solo u2fxls.dll", res.Fallos)
	}
	if len(res.Copiados) != 5 {
		t.Errorf("copiados = %v, quiero los otros 5", res.Copiados)
	}
}

// El EXE puede compilar bien y salir sin el runtime adentro (ya pasó en F2: el
// linker descarta los globales que nadie referencia). Sin este corte, una
// instalación sin Crystal terminaría diciendo "OK".
func TestInstallCrystalFallaSiElRuntimeEstaVacio(t *testing.T) {
	res, err := InstallCrystal(fstest.MapFS{}, filepath.Join(t.TempDir(), "syswow64"), sinRegistrar, func(string) {})
	if err == nil {
		t.Fatal("con el runtime vacío no falló: el binario se compiló sin los assets")
	}
	if !strings.Contains(err.Error(), "vacío") {
		t.Errorf("error = %q, quiero que diga que el runtime embebido está vacío", err)
	}
	if !res.NadaQueHacer() {
		t.Errorf("resultado = %+v, quiero nada que hacer", res)
	}
}

// Un embed incompleto no puede pasar como instalación completa: si falta uno de
// los componentes que hay que registrar, hay que decirlo.
func TestInstallCrystalAvisaSiFaltaUnComponenteCOM(t *testing.T) {
	src := runtimeFalso()
	delete(src, "crviewer.dll")

	res, err := InstallCrystal(src, filepath.Join(t.TempDir(), "syswow64"), sinRegistrar, func(string) {})
	if err != nil {
		t.Fatalf("InstallCrystal: %v", err)
	}
	if len(res.Fallos) != 1 || !strings.Contains(res.Fallos[0], "crviewer.dll") {
		t.Errorf("fallos = %v, quiero el componente que falta en el embed", res.Fallos)
	}
}

// El nombre real del archivo en el runtime manda: si el embed lo trae con otra
// mayúscula, regsvr32 tiene que recibir esa ruta, no la del listado.
func TestInstallCrystalRegistraLaRutaReal(t *testing.T) {
	src := fstest.MapFS{
		"CRVIEWER.DLL": {Data: []byte("visor")},
	}
	reg := &registro{}

	if _, err := InstallCrystal(src, filepath.Join(t.TempDir(), "syswow64"), reg.fn, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if len(reg.paths) != 1 || filepath.Base(reg.paths[0]) != "CRVIEWER.DLL" {
		t.Errorf("registró %v, quiero la ruta real CRVIEWER.DLL", reg.paths)
	}
}

// Una falla de registro no puede tumbar la instalación: el runtime ya quedó
// copiado y eso es lo que la app necesita para abrir un reporte. Se reporta como
// pendiente, igual que los OCX.
func TestInstallCrystalReportaElRegistroQueFalla(t *testing.T) {
	reg := &registro{err: os.ErrPermission}

	res, err := InstallCrystal(runtimeFalso(), filepath.Join(t.TempDir(), "syswow64"), reg.fn, func(string) {})
	if err != nil {
		t.Fatalf("InstallCrystal: %v", err)
	}
	if len(res.Fallos) != 4 {
		t.Errorf("fallos = %v, quiero los 4 componentes", res.Fallos)
	}
	if len(res.Registrados) != 0 {
		t.Errorf("registrados = %v, quiero ninguno", res.Registrados)
	}
	if len(res.Copiados) != 6 {
		t.Errorf("copiados = %v, quiero los 6", res.Copiados)
	}
}

// Lo que cambió se dice en pantalla y lo que ya estaba no: 43 líneas de "ya estaba"
// tapan los errores que importan.
func TestInstallCrystalEmiteSoloLoQueCambia(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "syswow64")
	src := runtimeFalso()

	var primera []string
	if _, err := InstallCrystal(src, dst, sinRegistrar, func(s string) { primera = append(primera, s) }); err != nil {
		t.Fatal(err)
	}
	// 6 copias + 4 registros.
	if len(primera) != 10 {
		t.Errorf("primera pasada emitió %d líneas (%v), quiero 10", len(primera), primera)
	}

	var segunda []string
	if _, err := InstallCrystal(src, dst, sinRegistrar, func(s string) { segunda = append(segunda, s) }); err != nil {
		t.Fatal(err)
	}
	// Nada que copiar; los 4 registros se repiten.
	if len(segunda) != 4 {
		t.Errorf("segunda pasada emitió %d líneas (%v), quiero solo los 4 registros", len(segunda), segunda)
	}
}

// Los 4 componentes del LEEME (paso 2), que difiere de los 5 archivos que el test
// del embed exige presentes: crpe32.dll se copia, no se registra.
func TestCrystalCOM(t *testing.T) {
	quiero := []string{"Crystl32.OCX", "craxddrt.dll", "craxdrt.dll", "crviewer.dll"}
	if got := soloNombres(CrystalCOM); strings.Join(got, ",") != strings.Join(quiero, ",") {
		t.Errorf("CrystalCOM = %v, quiero %v", got, quiero)
	}
}

// RegistrarCOM es la única puerta al regsvr32 de 32 bits. Con un archivo que no
// existe tiene que devolver error y no colgarse ni abrir un cartel.
func TestRegisterCOMConArchivoInexistenteFalla(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("regsvr32 solo existe en Windows")
	}
	fantasma := filepath.Join(t.TempDir(), "no-existe.dll")
	if err := RegisterCOM(fantasma); err == nil {
		t.Errorf("registrar %s no falló", fantasma)
	}
}

func TestPatchCrystalODBCBridge(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("puente Crystal ODBC solo aplica en Windows")
	}
	appDir := t.TempDir()
	var logs []string
	logFn := func(s string) { logs = append(logs, s) }

	// Primera pasada con clave
	err := PatchCrystalODBCBridge(appDir, "Password123#Test", logFn)
	if err != nil {
		t.Fatalf("PatchCrystalODBCBridge falló: %v", err)
	}

	p2s := filepath.Join(appDir, "p2sodbc.dll")
	if fi, err := os.Stat(p2s); err != nil || fi.Size() == 0 {
		t.Fatalf("no se generó p2sodbc.dll en appDir: %v", err)
	}

	// Idempotencia: segunda pasada con la misma clave no debe fallar
	logs = nil
	err = PatchCrystalODBCBridge(appDir, "Password123#Test", logFn)
	if err != nil {
		t.Fatalf("segunda pasada falló: %v", err)
	}
	if len(logs) == 0 || !strings.Contains(logs[0], "ya se encuentra configurado") {
		t.Errorf("segunda pasada debió ser idempotente, logs: %v", logs)
	}
}
