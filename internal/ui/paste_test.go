// © Antony Monge López — Costa Rica — Céd. 604700548
package ui

import (
	"path/filepath"
	"testing"

	"aegis-setup/internal/config"
	tea "charm.land/bubbletea/v2"
)

func pegar(m Model, texto string) Model {
	out, _ := m.Update(tea.PasteMsg{Content: texto})
	return out.(Model)
}

func TestPasteBracketedInInputs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	m := NewModel(config.Default(), path)

	// Ir a Configurar conexión (opción 1)
	m = pulsar(m, "1")
	if m.screen != screenServer {
		t.Fatalf("pantalla = %v, quiero screenServer", m.screen)
	}

	// 1. Pegar servidor con formato IP,puerto
	m = pegar(m, "192.168.2.145,54721")
	if m.srvInput.Value() != "192.168.2.145,54721" {
		t.Fatalf("srvInput.Value() = %q, quiero 192.168.2.145,54721", m.srvInput.Value())
	}
	m = pulsar(m, "enter")
	if m.screen != screenPort {
		t.Fatalf("pantalla = %v, quiero screenPort", m.screen)
	}
	m = pulsar(m, "enter")
	if m.screen != screenDatabase {
		t.Fatalf("pantalla = %v, quiero screenDatabase", m.screen)
	}

	// 2. Pegar base de datos (limpiando el valor pre-cargado primero)
	m = pulsar(m, "ctrl+u")
	m = pegar(m, "SIDC_LOCAL")
	if m.dbInput.Value() != "SIDC_LOCAL" {
		t.Fatalf("dbInput.Value() = %q, quiero SIDC_LOCAL", m.dbInput.Value())
	}
	m = pulsar(m, "enter")
	if m.screen != screenAuth {
		t.Fatalf("pantalla = %v, quiero screenAuth", m.screen)
	}

	// 3. Seleccionar SQL Auth
	m = pulsar(m, "2")
	if m.screen != screenSQLUser {
		t.Fatalf("pantalla = %v, quiero screenSQLUser", m.screen)
	}

	// 4. Pegar usuario SQL
	m = pegar(m, "sidc_admin")
	if m.userInput.Value() != "sidc_admin" {
		t.Fatalf("userInput.Value() = %q, quiero sidc_admin", m.userInput.Value())
	}
	m = pulsar(m, "enter")
	if m.screen != screenAppDir {
		t.Fatalf("pantalla = %v, quiero screenAppDir", m.screen)
	}

	// 5. Pegar carpeta de la aplicación
	m = pegar(m, `C:\SIDC`)
	if m.dirInput.Value() != `C:\SIDC` {
		t.Fatalf("dirInput.Value() = %q, quiero C:\\SIDC", m.dirInput.Value())
	}
	m = pulsar(m, "enter")
	if m.screen != screenDone {
		t.Fatalf("pantalla = %v, quiero screenDone", m.screen)
	}

	saved, err := config.Load(path)
	if err != nil {
		t.Fatalf("error leyendo config guardada: %v", err)
	}
	if saved.Server != "192.168.2.145,54721" {
		t.Errorf("server = %q", saved.Server)
	}
	if saved.Database != "SIDC_LOCAL" {
		t.Errorf("database = %q", saved.Database)
	}
	if saved.SQLUser != "sidc_admin" {
		t.Errorf("sql_user = %q", saved.SQLUser)
	}
	if saved.AppDir != `C:\SIDC` {
		t.Errorf("app_dir = %q", saved.AppDir)
	}
}

func TestPasteInAskInput(t *testing.T) {
	t.Setenv(envSAPassword, "")
	t.Setenv(envAppPassword, "")

	m := pulsar(NewModel(devCfg(), ""), "0")
	if m.screen != screenAsk {
		t.Fatalf("pantalla = %v, quiero screenAsk", m.screen)
	}

	m = pegar(m, "Clave123456")
	if m.askInput.Value() != "Clave123456" {
		t.Fatalf("askInput.Value() = %q, quiero Clave123456", m.askInput.Value())
	}
}

func TestPasteInLogin(t *testing.T) {
	m := NewModel(config.Default(), "")
	m = pulsar(m, "l")
	if m.screen != screenLogin {
		t.Fatalf("pantalla = %v, quiero screenLogin", m.screen)
	}

	// Pegar correo
	m = pegar(m, "operador@femucaribe.go.cr")
	if m.login.inputs[0].Value() != "operador@femucaribe.go.cr" {
		t.Fatalf("inputs[0].Value() = %q", m.login.inputs[0].Value())
	}

	// Tab a contraseña
	m = pulsar(m, "tab")
	if m.login.focused != 1 {
		t.Fatalf("focused = %v, quiero 1", m.login.focused)
	}

	// Pegar contraseña
	m = pegar(m, "SuperSecret123")
	if m.login.inputs[1].Value() != "SuperSecret123" {
		t.Fatalf("inputs[1].Value() = %q", m.login.inputs[1].Value())
	}
}

func TestKeyPasteTriggersCommand(t *testing.T) {
	m := NewModel(config.Default(), "")
	m = pulsar(m, "1")
	if m.screen != screenServer {
		t.Fatalf("pantalla = %v, quiero screenServer", m.screen)
	}

	_, cmd := m.Update(tecla("ctrl+v"))
	if cmd == nil {
		t.Fatal("ctrl+v en screenServer debería retornar un tea.Cmd para leer el portapapeles")
	}

	_, cmdAlt := m.Update(tecla("ctrl+alt+v"))
	if cmdAlt == nil {
		t.Fatal("ctrl+alt+v en screenServer debería retornar un tea.Cmd para leer el portapapeles")
	}
}

