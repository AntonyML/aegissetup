// © Antony Monge López — Costa Rica — Céd. 604700548
package ui

import (
	"errors"
	"strings"
	"testing"

	"aegis-setup/internal/auth"
	"aegis-setup/internal/config"

	tea "charm.land/bubbletea/v2"
)

func TestTUI_LoginObligatorioAlIniciar(t *testing.T) {
	mgr := auth.NewManager("https://test.supabase.co", "test-key", t.TempDir()+"/session.json", nil)

	m := NewModel(config.Default(), "").SetAuthManager(mgr)

	if m.screen != screenLogin {
		t.Fatalf("se esperaba pantalla screenLogin (%d), se obtuvo %d", screenLogin, m.screen)
	}

	vista := m.View().Content
	if !strings.Contains(vista, "Autenticación Obligatoria") {
		t.Errorf("la vista debe mostrar 'Autenticación Obligatoria', se obtuvo:\n%s", vista)
	}
	if !strings.Contains(vista, "Correo institucional:") {
		t.Errorf("la vista debe pedir correo institucional, se obtuvo:\n%s", vista)
	}
}

func TestTUI_LoginExitosoTransicionaAMenu(t *testing.T) {
	mgr := auth.NewManager("https://test.supabase.co", "test-key", t.TempDir()+"/session.json", nil)
	m := NewModel(config.Default(), "").SetAuthManager(mgr)

	sess := &auth.Session{
		AccessToken: "token-abc",
		User: auth.User{
			Email: "operador@femucaribe.go.cr",
		},
	}

	out, _ := m.Update(loginSuccessMsg{Session: sess})
	m2 := out.(Model)

	if m2.screen != screenMenu {
		t.Errorf("se esperaba pantalla screenMenu (%d), se obtuvo %d", screenMenu, m2.screen)
	}
	if !m2.authenticated {
		t.Error("m2.authenticated debe ser true")
	}
	if m2.operatorUser != "operador@femucaribe.go.cr" {
		t.Errorf("operatorUser esperado operador@femucaribe.go.cr, obtenido %s", m2.operatorUser)
	}

	vista := m2.View().Content
	if !strings.Contains(vista, "OPERADOR: operador@femucaribe.go.cr") {
		t.Errorf("la vista del menú debe incluir el badge del operador, se obtuvo:\n%s", vista)
	}
}

func TestTUI_LoginFallidoNoCongelaYMuestraError(t *testing.T) {
	mgr := auth.NewManager("https://test.supabase.co", "test-key", t.TempDir()+"/session.json", nil)
	m := NewModel(config.Default(), "").SetAuthManager(mgr)

	out, _ := m.Update(loginFailedMsg{Err: errors.New("credenciales inválidas")})
	m2 := out.(Model)

	if m2.screen != screenLogin {
		t.Errorf("se esperaba permanecer en screenLogin (%d), se obtuvo %d", screenLogin, m2.screen)
	}
	if m2.login.loading {
		t.Error("m2.login.loading debe quedar en false tras un fallo")
	}

	vista := m2.View().Content
	if !strings.Contains(vista, "credenciales inválidas") {
		t.Errorf("la vista debe mostrar el error en pantalla, se obtuvo:\n%s", vista)
	}
}

func TestTUI_TeclaLCierraSesionYVuelveALogin(t *testing.T) {
	mgr := auth.NewManager("https://test.supabase.co", "test-key", t.TempDir()+"/session.json", nil)
	m := NewModel(config.Default(), "").SetAuthManager(mgr)

	out, _ := m.Update(loginSuccessMsg{
		Session: &auth.Session{
			AccessToken: "token-xyz",
			User:        auth.User{Email: "admin@femucaribe.go.cr"},
		},
	})
	m2 := out.(Model)

	if m2.screen != screenMenu {
		t.Fatalf("se esperaba screenMenu")
	}

	out3, _ := m2.Update(tea.KeyPressMsg(tea.Key{Text: "l", Code: 'l'}))
	m3 := out3.(Model)

	if m3.screen != screenLogin {
		t.Errorf("al presionar L se esperaba volver a screenLogin, se obtuvo %d", m3.screen)
	}
	if m3.authenticated {
		t.Error("m3.authenticated debe ser false tras presionar L")
	}
}

func TestTUI_EscEnLoginObligatorioSale(t *testing.T) {
	mgr := auth.NewManager("https://test.supabase.co", "test-key", t.TempDir()+"/session.json", nil)
	m := NewModel(config.Default(), "").SetAuthManager(mgr)

	out, cmd := m.Update(cancelLoginMsg{})
	_ = out.(Model)
	if cmd == nil {
		t.Error("cancelLoginMsg sin estar autenticado debe emitir tea.Quit cmd")
	}
}
