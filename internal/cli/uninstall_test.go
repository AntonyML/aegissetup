// © Antony Monge López — Costa Rica — Céd. 604700548
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aegis-setup/internal/config"
	"aegis-setup/internal/setup"
)

// cfgAislado deja el ProgramData y el %APPDATA% de mentira y devuelve un config que
// apunta ahí. Sin esto el test corre el plan de desinstalación contra la máquina real.
func cfgAislado(t *testing.T) (config.Config, string) {
	t.Helper()
	t.Setenv("AEGIS_PROGRAMDATA", t.TempDir())
	t.Setenv("AEGIS_APPDATA", t.TempDir())

	cfg := config.Default()
	for _, dir := range []string{config.DirConfig(), config.DirBackups()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(config.DirConfig(), "config.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfg, t.TempDir()
}

// sinRegistro saca del plan los pasos que escriben en HKLM.
//
// Un test que ejecute el plan completo borra el DSN SIDC_SQL de la MÁQUINA DONDE CORRE
// go test. No es hipotético: pasó, y el DSN se tuvo que restaurar a mano. Los pasos de
// registro se verifican por su contenido en el plan (ver TestUninstallDiceQueElDSNSeVa),
// nunca ejecutándolos.
func sinRegistro(pasos []setup.UninstallStep) []setup.UninstallStep {
	var out []setup.UninstallStep
	for _, p := range pasos {
		if p.Kind != setup.StepDSN && p.Kind != setup.StepDSNList {
			out = append(out, p)
		}
	}
	return out
}

// escribirUninstall con yes=false es un plan, no una desinstalación: tiene que decir qué
// se va a borrar, avisar qué NO se toca y no tocar el disco. Sin esta guarda, el comando
// borra la configuración apenas alguien lo escribe para ver qué hace.
func TestUninstallSinYesSoloMuestraElPlanYNoBorra(t *testing.T) {
	cfg, exeDir := cfgAislado(t)
	pasos := setup.PlanUninstall(cfg, exeDir)

	var buf bytes.Buffer
	if err := escribirUninstall(&buf, pasos, false); err != nil {
		t.Fatalf("el plan no puede fallar: %v", err)
	}

	if _, err := os.Stat(config.DirConfig()); err != nil {
		t.Errorf("el plan borró la config: %v", err)
	}
	if _, err := os.Stat(config.DirProgramData()); err != nil {
		t.Errorf("el plan borró el ProgramData: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "== AEGIS UNINSTALL ==") {
		t.Errorf("sin encabezado:\n%s", out)
	}
	if !strings.Contains(out, "--yes") {
		t.Errorf("no dice cómo ejecutar de verdad:\n%s", out)
	}
	if !strings.Contains(out, "no se borró nada") {
		t.Errorf("no aclara que fue solo un plan:\n%s", out)
	}
}

func TestUninstallConYesBorra(t *testing.T) {
	cfg, exeDir := cfgAislado(t)
	pasos := sinRegistro(setup.PlanUninstall(cfg, exeDir))

	var buf bytes.Buffer
	if err := escribirUninstall(&buf, pasos, true); err != nil {
		t.Fatalf("con --yes no debería fallar: %v", err)
	}
	for _, dir := range []string{config.DirConfig(), config.DirProgramData()} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Errorf("%s sigue existiendo (err=%v)", dir, err)
		}
	}
	if !strings.Contains(buf.String(), "OK") {
		t.Errorf("no reportó el resultado de cada paso:\n%s", buf.String())
	}
}

// El runtime de Crystal y los OCX en SysWOW64 los comparte Windows y otros programas VB6.
// Borrarlos puede romper software que no es SIDC, así que la desinstalación los deja. Eso
// tiene que estar DICHO: un operador que desinstala esperando una PC limpia y encuentra 54
// archivos en SysWOW64 piensa que el comando falló.
func TestUninstallDiceQueElRuntimeQuedaEnSysWOW64(t *testing.T) {
	cfg, exeDir := cfgAislado(t)

	var buf bytes.Buffer
	if err := escribirUninstall(&buf, setup.PlanUninstall(cfg, exeDir), false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, esperado := range []string{"SysWOW64", "Crystal", "no se tocan"} {
		if !strings.Contains(out, esperado) {
			t.Errorf("no aparece %q en:\n%s", esperado, out)
		}
	}
}

// Un plan que se lleva el respaldo de 39 MB sin decirlo deja al operador sin poder
// reinstalar y sin saber por qué.
func TestUninstallAvisaSiSeLlevaElRespaldo(t *testing.T) {
	cfg, exeDir := cfgAislado(t)
	if err := os.WriteFile(filepath.Join(config.DirBackups(), "SIDC_2014.bak"), make([]byte, 4096), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := escribirUninstall(&buf, setup.PlanUninstall(cfg, exeDir), false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "OJO") || !strings.Contains(out, ".bak") {
		t.Errorf("no avisó del respaldo:\n%s", out)
	}
	if !strings.Contains(out, "aegis bak") {
		t.Errorf("no dice cómo recuperarlo después:\n%s", out)
	}
}

// El DSN tiene que estar en el plan: si queda vivo, el próximo install lo actualiza en
// vez de crearlo y la máquina arranca con el servidor o la base del cliente anterior.
// Se verifica mirando el plan, sin ejecutarlo (ver sinRegistro).
func TestUninstallDiceQueElDSNSeVa(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("el DSN solo existe en Windows")
	}
	cfg, exeDir := cfgAislado(t)

	var buf bytes.Buffer
	if err := escribirUninstall(&buf, setup.PlanUninstall(cfg, exeDir), false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, cfg.DsnName) {
		t.Errorf("el plan no nombra el DSN %q:\n%s", cfg.DsnName, out)
	}
	if !strings.Contains(out, "ODBC") {
		t.Errorf("el plan no dice dónde vive el DSN:\n%s", out)
	}
	// La entrada vive dentro de una clave compartida: hay que mostrar el VALOR, no solo
	// la clave, o el operador no sabe qué se lleva.
	if !strings.Contains(out, "ODBC Data Sources") {
		t.Errorf("el plan no muestra la clave compartida de la lista de ODBC:\n%s", out)
	}
	// Y tiene que avisar que eso rompe la app, con la salida a mano.
	if !strings.Contains(out, "--keep-dsn") {
		t.Errorf("no avisa que sacar el DSN rompe SIDC ni cómo evitarlo:\n%s", out)
	}
}

// El caso real: Aegis es la herramienta de instalación, así que el operador que la saca
// suele querer SIDC andando. Sin --keep-dsn la desinstalación deja la app con error 3146.
func TestUninstallKeepDsnDejaElDSNVivo(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("el DSN solo existe en Windows")
	}
	cfg, _ := cfgAislado(t)

	var buf bytes.Buffer
	cmd := NewRootCmd("", func(string) (config.Config, error) { return cfg, nil })
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"uninstall", "--keep-dsn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("el plan con --keep-dsn no puede fallar: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, cfg.DsnName) {
		t.Errorf("con --keep-dsn el DSN no puede estar en el plan:\n%s", out)
	}
	if strings.Contains(out, "--keep-dsn") {
		t.Errorf("con --keep-dsn no tiene sentido avisar que saca el DSN:\n%s", out)
	}
	// El resto del plan sigue: --keep-dsn saca el DSN, no la desinstalación.
	if !strings.Contains(out, config.DirConfig()) || !strings.Contains(out, config.DirProgramData()) {
		t.Errorf("--keep-dsn se llevó el resto del plan:\n%s", out)
	}
}

// El paso del config heredado tiene que apuntar a la carpeta del ejecutable. Pasarle la
// ruta del config.json resuelto (que es el segundo valor que devuelve el resolvedor, y NO
// la carpeta del exe) armaba "...\config.json\config.json": un paso que muestra una ruta
// falsa y que borra el archivo equivocado.
func TestUninstallNoDuplicaLaRutaDelConfigHeredado(t *testing.T) {
	cfg, _ := cfgAislado(t)

	var buf bytes.Buffer
	cmd := NewRootCmd("", func(string) (config.Config, error) { return cfg, nil })
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"uninstall"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if dup := filepath.Join("config.json", "config.json"); strings.Contains(buf.String(), dup) {
		t.Errorf("la ruta del config heredado está duplicada (%s):\n%s", dup, buf.String())
	}
}

// Un comando sin registrar en Cobra compila, pasa los tests de la función y no existe para
// el usuario.
func TestUninstallEstaRegistradoEnElRoot(t *testing.T) {
	cmd := NewRootCmd("", func(string) (config.Config, error) { return config.Default(), nil })

	var nombres []string
	for _, c := range cmd.Commands() {
		nombres = append(nombres, c.Name())
	}
	if !contains(nombres, "uninstall") {
		t.Errorf("'uninstall' no está registrado; comandos: %v", nombres)
	}
}
