// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aegis-setup/internal/config"
)

// soloArchivos saca del plan los pasos de registro. Una prueba que ejecute el plan
// completo borraría el DSN SIDC_SQL de la máquina donde corre el test, que es el
// equivalente a "el test escribe en el ProgramData real": el mismo error de siempre, pero
// en HKLM y sin forma de deshacerlo.
func soloArchivos(pasos []UninstallStep) []UninstallStep {
	var out []UninstallStep
	for _, p := range pasos {
		if p.Kind == StepConfig || p.Kind == StepData {
			out = append(out, p)
		}
	}
	return out
}

// arbolDePrueba deja un ProgramData y un %APPDATA% de mentira con un .bak adentro, y
// devuelve el config que apunta ahí.
func arbolDePrueba(t *testing.T) config.Config {
	t.Helper()
	t.Setenv("AEGIS_PROGRAMDATA", t.TempDir())
	t.Setenv("AEGIS_APPDATA", t.TempDir())

	cfg := config.Default()
	for _, dir := range []string{config.DirConfig(), config.DirProgramData(),
		filepath.Join(config.DirProgramData(), "docker"), config.DirBackups()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	escribir := func(path string, n int) {
		if err := os.WriteFile(path, make([]byte, n), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	escribir(filepath.Join(config.DirConfig(), "config.json"), 100)
	escribir(filepath.Join(config.DirBackups(), "SIDC_2014.bak"), 2048)
	escribir(filepath.Join(config.DirProgramData(), "docker", "docker-compose.yml"), 512)
	return cfg
}

// Lo que NO entra en el plan es tan importante como lo que entra: la base, el login app y
// la carpeta de SIDC no los puso Aegis.
func TestPlanBorraLoDeAegisYNadaMas(t *testing.T) {
	cfg := arbolDePrueba(t)
	cfg.AppDir = filepath.Join(t.TempDir(), "SIDC")

	pasos := PlanUninstall(cfg, t.TempDir())

	var rutas []string
	for _, p := range pasos {
		rutas = append(rutas, p.Path)
		if p.Path != "" && cfg.AppDir != "" && strings.EqualFold(filepath.Clean(p.Path), filepath.Clean(cfg.AppDir)) {
			t.Errorf("el plan incluye app_dir (%s): los reportes y fotos son del cliente", p.Path)
		}
	}
	for _, esperado := range []string{config.DirConfig(), config.DirProgramData()} {
		if !containsStr(rutas, esperado) {
			t.Errorf("el plan no incluye %q; rutas: %v", esperado, rutas)
		}
	}
	if runtime.GOOS == "windows" {
		var dsn *UninstallStep
		for i := range pasos {
			if pasos[i].Kind == StepDSN {
				dsn = &pasos[i]
			}
		}
		if dsn == nil {
			t.Fatal("en Windows el plan tiene que sacar el DSN: si queda, el próximo install lo actualiza en vez de crearlo")
		}
		if !strings.HasSuffix(dsn.Path, cfg.DsnName) {
			t.Errorf("el paso del DSN apunta a %q, quiero que termine en %q", dsn.Path, cfg.DsnName)
		}
	}
}

// Un "--yes" a ciegas que se lleva el respaldo de 39 MB deja al operador sin poder
// reinstalar y sin saber por qué. El plan tiene que contarlo antes.
func TestPlanCuentaElBakQueSeVaALlevar(t *testing.T) {
	cfg := arbolDePrueba(t)
	pasos := PlanUninstall(cfg, t.TempDir())

	var datos *UninstallStep
	for i := range pasos {
		if pasos[i].Kind == StepData {
			datos = &pasos[i]
		}
	}
	if datos == nil {
		t.Fatal("no hay paso de datos de máquina")
	}
	if datos.BakFiles != 1 {
		t.Errorf("BakFiles = %d, quiero 1", datos.BakFiles)
	}
	if datos.BakBytes != 2048 {
		t.Errorf("BakBytes = %d, quiero 2048", datos.BakBytes)
	}
	// .bak (2048) + docker-compose.yml (512). El config.json NO cuenta: vive en
	// %APPDATA%, no en ProgramData.
	if datos.Files != 2 {
		t.Errorf("Files = %d, quiero 2", datos.Files)
	}

	n, bytes := FaltaElRespaldo(pasos)
	if n != 1 || bytes != 2048 {
		t.Errorf("FaltaElRespaldo = (%d, %d), quiero (1, 2048)", n, bytes)
	}
}

func TestDesinstalarBorraLosArboles(t *testing.T) {
	cfg := arbolDePrueba(t)
	var lineas []string
	fallos := Desinstalar(soloArchivos(PlanUninstall(cfg, t.TempDir())), func(s string) { lineas = append(lineas, s) })

	if len(fallos) != 0 {
		t.Fatalf("fallos = %v (salida: %v)", fallos, lineas)
	}
	for _, dir := range []string{config.DirConfig(), config.DirProgramData()} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Errorf("%s sigue existiendo (err=%v)", dir, err)
		}
	}
	// El mensaje tiene que decir qué se llevó: es lo único que queda si algo se borra
	// de más.
	salida := strings.Join(lineas, "\n")
	if !strings.Contains(salida, "2 archivos") {
		t.Errorf("no reportó cuánto borró:\n%s", salida)
	}
	if runtime.GOOS == "windows" && !strings.Contains(salida, "Datos de máquina") {
		t.Errorf("no nombró el paso de datos:\n%s", salida)
	}
}

// Correrlo dos veces tiene que ser seguro: el segundo pase no encuentra nada y eso no es
// un fallo. Un desinstalador que falla la segunda vez obliga a correrlo una sola vez y
// con miedo.
func TestDesinstalarEsIdempotente(t *testing.T) {
	cfg := arbolDePrueba(t)
	pasos := soloArchivos(PlanUninstall(cfg, t.TempDir()))

	if fallos := Desinstalar(pasos, func(string) {}); len(fallos) != 0 {
		t.Fatalf("primera pasada: %v", fallos)
	}
	var lineas []string
	fallos := Desinstalar(pasos, func(s string) { lineas = append(lineas, s) })
	if len(fallos) != 0 {
		t.Fatalf("segunda pasada: %v", fallos)
	}
	if !strings.Contains(strings.Join(lineas, "\n"), "no estaba") {
		t.Errorf("no dijo que ya no estaba:\n%s", strings.Join(lineas, "\n"))
	}
}

// El archivo que el operador tiene en la carpeta de SIDC no lo puso Aegis: la
// desinstalación no puede llevárselo ni de casualidad.
func TestDesinstalarNoTocaLaCarpetaDeSIDC(t *testing.T) {
	cfg := arbolDePrueba(t)
	appDir := t.TempDir()
	cfg.AppDir = appDir
	reporte := filepath.Join(appDir, "Reportes", "factura.rpt")
	if err := os.MkdirAll(filepath.Dir(reporte), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reporte, []byte("reporte"), 0o644); err != nil {
		t.Fatal(err)
	}

	Desinstalar(soloArchivos(PlanUninstall(cfg, t.TempDir())), func(string) {})

	if _, err := os.Stat(reporte); err != nil {
		t.Errorf("se llevó un archivo del cliente: %v", err)
	}
}

// SinDSN existe por un caso de uso real: Aegis es la herramienta de instalación, así que
// el operador que la saca suele querer SIDC andando. Si el plan se lleva el DSN sin decir
// nada, la app queda con error 3146 y parece que la desinstalación rompió SIDC.
func TestSinDSNDejaSoloLoDeArchivos(t *testing.T) {
	cfg := arbolDePrueba(t)
	pasos := SinDSN(PlanUninstall(cfg, t.TempDir()))

	if len(pasos) != 3 {
		t.Fatalf("quedaron %d pasos, quiero 3 (config, data, config heredado)", len(pasos))
	}
	for _, p := range pasos {
		if p.Kind == StepDSN || p.Kind == StepDSNList {
			t.Errorf("quedó un paso de registro: %s (%s)", p.Title, p.Path)
		}
	}
}

func containsStr(xs []string, x string) bool {
	for _, s := range xs {
		if strings.EqualFold(filepath.Clean(s), filepath.Clean(x)) {
			return true
		}
	}
	return false
}
