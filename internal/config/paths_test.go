// © Antony Monge López — Costa Rica — Céd. 604700548
package config

import (
	"os"
	"path/filepath"
	"reflect"
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
