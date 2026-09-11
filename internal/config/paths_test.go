// © Antony Monge López — Costa Rica — Céd. 604700548
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestDirProgramDataRespetaOverride fija el layout machine-wide. El instalador
// corre elevado, así que no puede depender de "Documentos" (apuntaría al perfil
// del administrador): todo lo de máquina vive bajo ProgramData.
func TestDirProgramDataRespetaOverride(t *testing.T) {
	base := t.TempDir()
	t.Setenv(envProgramData, base)

	casos := []struct {
		nombre string
		got    string
		quiero string
	}{
		{"DirProgramData", DirProgramData(), filepath.Join(base, "AegisSetup")},
		{"DirAssets", DirAssets(), filepath.Join(base, "AegisSetup", "assets")},
		{"DirBackups", DirBackups(), filepath.Join(base, "AegisSetup", "assets", "backups", "sqlserver2014")},
	}
	for _, c := range casos {
		if c.got != c.quiero {
			t.Errorf("%s = %q, quiero %q", c.nombre, c.got, c.quiero)
		}
	}
}

// TestDirConfigRespetaOverride fija la configuración en %APPDATA%\AegisSetup,
// que es la ubicación convencional de Windows para datos por usuario.
func TestDirConfigRespetaOverride(t *testing.T) {
	base := t.TempDir()
	t.Setenv(envAppData, base)

	if got, quiero := DirConfig(), filepath.Join(base, "AegisSetup"); got != quiero {
		t.Errorf("DirConfig = %q, quiero %q", got, quiero)
	}
	if got, quiero := RutaConfig(), filepath.Join(base, "AegisSetup", "config.json"); got != quiero {
		t.Errorf("RutaConfig = %q, quiero %q", got, quiero)
	}
}

// TestAsegurarRutasCreaArbol confirma que las carpetas se crean en el primer
// arranque y que llamarlo dos veces no falla (idempotente).
func TestAsegurarRutasCreaArbol(t *testing.T) {
	t.Setenv(envProgramData, t.TempDir())
	t.Setenv(envAppData, t.TempDir())

	for intento := 1; intento <= 2; intento++ {
		if err := AsegurarRutas(); err != nil {
			t.Fatalf("intento %d: AsegurarRutas: %v", intento, err)
		}
	}
	for _, dir := range []string{DirBackups(), DirConfig()} {
		fi, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("no se creó %s: %v", dir, err)
		}
		if !fi.IsDir() {
			t.Errorf("%s no es directorio", dir)
		}
	}
}

// TestBackupDirsOrdenYPrioridad fija la compatibilidad: el .bak se busca donde
// diga la config y, si no está, en los dos layouts junto al binario. Sin esto,
// mover BackupDir a ProgramData rompería el flujo dev sin aviso.
func TestBackupDirsOrdenYPrioridad(t *testing.T) {
	// Build de desarrollo: el EXE sale a bin/ y los assets quedan un nivel arriba.
	exeDir := `C:\DEV\SIDC\AegisSetup\bin`
	juntoAlExe := filepath.Join(exeDir, "assets", "backups", "sqlserver2014")
	unNivelArriba := filepath.Join(exeDir, "..", "assets", "backups", "sqlserver2014")
	canonico := `C:\ProgramData\AegisSetup\assets\backups\sqlserver2014`

	casos := []struct {
		nombre string
		cfg    Config
		quiero []string
	}{
		{
			"config primero, layouts del proyecto después",
			Config{BackupDir: canonico},
			[]string{canonico, juntoAlExe, unNivelArriba},
		},
		{
			"sin duplicados si la config ya apunta arriba",
			Config{BackupDir: unNivelArriba},
			[]string{unNivelArriba, juntoAlExe},
		},
		{
			"sin duplicados si la config apunta junto al EXE",
			Config{BackupDir: juntoAlExe},
			[]string{juntoAlExe, unNivelArriba},
		},
		{
			"backup_dir vacío no genera candidatos vacíos",
			Config{BackupDir: ""},
			[]string{juntoAlExe, unNivelArriba},
		},
	}
	for _, c := range casos {
		got := BackupDirs(c.cfg, exeDir)
		if !reflect.DeepEqual(got, c.quiero) {
			t.Errorf("%s: BackupDirs =\n  %#v\nquiero\n  %#v", c.nombre, got, c.quiero)
		}
	}
}

// TestDefaultUsaRutasDeMaquina confirma que Default() ya no arrastra rutas del
// repo de desarrollo: en una PC limpia el .bak vive bajo ProgramData.
func TestDefaultUsaRutasDeMaquina(t *testing.T) {
	base := t.TempDir()
	t.Setenv(envProgramData, base)

	cfg := Default()
	if quiero := filepath.Join(base, "AegisSetup", "assets", "backups", "sqlserver2014"); cfg.BackupDir != quiero {
		t.Errorf("Default().BackupDir = %q, quiero %q", cfg.BackupDir, quiero)
	}
	if cfg.Validate() != nil {
		t.Errorf("Default() debe ser válido, dio: %v", cfg.Validate())
	}
}

// TestDefaultNoTraeRutasDelDesarrollador es el requisito central de F7: la config por
// defecto no puede apuntar al repo de quien programa Aegis. En una PC limpia esas rutas
// no existen, y peor: si existen, el instalador mide la máquina equivocada y da por
// buenos archivos que no son los de la PC donde se está instalando.
func TestDefaultNoTraeRutasDelDesarrollador(t *testing.T) {
	cfg := Default()
	for nombre, valor := range map[string]string{
		"app_dir":    cfg.AppDir,
		"legacy_dir": cfg.LegacyDir,
		"docker_dir": cfg.DockerDir,
	} {
		if strings.Contains(valor, `C:\DEV\`) {
			t.Errorf("Default().%s = %q: es la ruta de la máquina donde se desarrolla Aegis", nombre, valor)
		}
	}
	// app_dir en particular no tiene default: la carpeta de SIDC la dice el operador
	// (perfil en el TUI o --app-dir en la CLI).
	if cfg.AppDir != "" {
		t.Errorf("Default().AppDir = %q, quiero vacío: no hay dónde adivinar dónde vive SIDC", cfg.AppDir)
	}
}

// El compose del SQL de pruebas se instala desde el propio EXE, así que su carpeta por
// defecto es la del layout del producto (ProgramData) y no una carpeta del repo que en la
// PC destino no existe.
func TestDockerDirEsLaDelProducto(t *testing.T) {
	t.Setenv(envProgramData, `C:\ProgramData`)
	if got, quiero := DirDocker(), `C:\ProgramData\AegisSetup\docker`; got != quiero {
		t.Errorf("DirDocker() = %q, quiero %q", got, quiero)
	}
	if got := Default().DockerDir; got != DirDocker() {
		t.Errorf("Default().DockerDir = %q, quiero %q", got, DirDocker())
	}
}

// Un config.json escrito antes de F7 trae docker_dir apuntando a la carpeta del repo de
// quien programa Aegis. Esa carpeta no existe en la PC destino, así que el arreglo del motor
// mandaría a correr un comando que no puede funcionar. Al cargar, esa carpeta se repara a la
// del producto; una carpeta propia que sí existe se respeta.
func TestLoadReparaElDockerDirInexistente(t *testing.T) {
	dir := t.TempDir()
	propio := filepath.Join(dir, "mi-compose")
	if err := os.MkdirAll(propio, 0o755); err != nil {
		t.Fatal(err)
	}

	casos := []struct {
		nombre string
		valor  string
		quiero string
	}{
		{"carpeta del repo de desarrollo", `C:\DEV\SIDC\docker-dev`, DirDocker()},
		{"ruta relativa", "docker-dev", DirDocker()},
		{"vacío", "", DirDocker()},
		{"carpeta propia que existe", propio, propio},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(p, []byte(`{"docker_dir": `+jsonCita(c.valor)+`}`), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(p)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.DockerDir != c.quiero {
				t.Errorf("DockerDir = %q, quiero %q", cfg.DockerDir, c.quiero)
			}
		})
	}
}

// jsonCita arma un string JSON sin pelear con las barras invertidas de Windows.
func jsonCita(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// Un app_dir relativo se resuelve contra el directorio de trabajo del proceso que corre
// Aegis, así que la misma config mide carpetas distintas según desde dónde se lance.
func TestValidateRechazaAppDirRelativo(t *testing.T) {
	casos := []struct {
		valor string
		ok    bool
	}{
		{`C:\SIDC`, true},
		{`C:\Program Files (x86)\SIDC`, true},
		{`c:/sidc`, true},
		{`\\CONTABILIDAD\SIDC`, true},
		{`SIDC`, false},
		{`.\SIDC`, false},
		{`..\SIDC`, false},
	}
	for _, c := range casos {
		cfg := Default()
		cfg.AppDir = c.valor
		err := cfg.Validate()
		if c.ok && err != nil {
			t.Errorf("app_dir %q debería ser válido, dio: %v", c.valor, err)
		}
		if !c.ok && err == nil {
			t.Errorf("app_dir %q es relativo y tiene que ser rechazado", c.valor)
		}
	}
}

// app_dir vacío es un estado válido y no un error de config: es la PC recién instalada
// donde todavía nadie dijo dónde está SIDC. El que avisa es el requisito del checklist,
// que sabe decirlo con una acción al lado.
func TestValidateAceptaAppDirVacio(t *testing.T) {
	cfg := Default()
	cfg.AppDir = ""
	if err := cfg.Validate(); err != nil {
		t.Fatalf("app_dir vacío es válido (la carpeta la dice el operador), dio: %v", err)
	}
}
