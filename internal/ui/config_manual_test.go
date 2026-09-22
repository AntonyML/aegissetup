// © Antony Monge López — Costa Rica — Céd. 604700548
package ui

import (
	"path/filepath"
	"testing"

	"aegis-setup/internal/config"
)

func TestConfigManualConSQLAuth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	m := NewModel(config.Default(), path)

	// Opción 1: Configurar conexión
	m = pulsar(m, "1")
	if m.screen != screenServer {
		t.Fatalf("pantalla = %v, quiero screenServer", m.screen)
	}

	// 1. Servidor
	m = escribir(m, "192.168.2.145,54721")
	if m.screen != screenDatabase {
		t.Fatalf("pantalla = %v, quiero screenDatabase", m.screen)
	}

	// 2. Base de datos (Enter acepta el default SIDC)
	m = pulsar(m, "enter")
	if m.screen != screenAuth {
		t.Fatalf("pantalla = %v, quiero screenAuth", m.screen)
	}

	// 3. Autenticación: "2" para SQL Server Auth
	m = pulsar(m, "2")
	if m.screen != screenSQLUser {
		t.Fatalf("pantalla = %v, quiero screenSQLUser", m.screen)
	}

	// 4. Usuario SQL
	m = escribir(m, "sidc")
	if m.screen != screenAppDir {
		t.Fatalf("pantalla = %v, quiero screenAppDir", m.screen)
	}

	// 5. Carpeta de SIDC
	m = escribir(m, `C:\SIDC`)
	if m.screen != screenDone {
		t.Fatalf("pantalla = %v, quiero screenDone", m.screen)
	}

	saved, err := config.Load(path)
	if err != nil {
		t.Fatalf("error al leer config: %v", err)
	}
	if saved.Server != "192.168.2.145,54721" {
		t.Errorf("server = %q, quiero 192.168.2.145,54721", saved.Server)
	}
	if saved.Database != "SIDC" {
		t.Errorf("database = %q, quiero SIDC", saved.Database)
	}
	if saved.UseWinAuth {
		t.Errorf("use_win_auth = true, quiero false para SQL Auth")
	}
	if saved.SQLUser != "sidc" {
		t.Errorf("sql_user = %q, quiero sidc", saved.SQLUser)
	}
	if saved.AppDir != `C:\SIDC` {
		t.Errorf("app_dir = %q, quiero C:\\SIDC", saved.AppDir)
	}
}

func TestConfigManualConWindowsAuth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	m := NewModel(config.Default(), path)

	m = pulsar(m, "1")
	m = escribir(m, "192.168.2.145,54721")
	m = pulsar(m, "enter")

	// 3. Autenticación: "1" para Windows Auth
	m = pulsar(m, "1")
	if m.screen != screenAppDir {
		t.Fatalf("pantalla = %v, quiero screenAppDir (Windows Auth no pide usuario)", m.screen)
	}

	m = escribir(m, `C:\SIDC`)
	if m.screen != screenDone {
		t.Fatalf("pantalla = %v, quiero screenDone", m.screen)
	}

	saved, err := config.Load(path)
	if err != nil {
		t.Fatalf("error al leer config: %v", err)
	}
	if !saved.UseWinAuth {
		t.Errorf("use_win_auth = false, quiero true para Windows Auth")
	}
}

func TestCheckPideClaveConSQLAuth(t *testing.T) {
	t.Setenv(envAppPassword, "")
	cfg := config.Default()
	cfg.UseWinAuth = false
	cfg.SQLUser = "sidc"

	m := pulsar(NewModel(cfg, ""), "3") // Tecla 3 = Check
	if m.screen != screenAsk {
		t.Fatalf("pantalla = %v, quiero screenAsk (debe pedir clave para Check)", m.screen)
	}
	if len(m.askQueue) != 1 || m.askQueue[0] != envAppPassword {
		t.Fatalf("cola = %v, quiero [%s]", m.askQueue, envAppPassword)
	}
}
