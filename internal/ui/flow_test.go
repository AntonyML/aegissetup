// © Antony Monge López — Costa Rica — Céd. 604700548
package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// tecla arma una pulsacion sintetica. Permite probar el cableado del TUI (que
// tecla lleva a donde) sin una terminal.
func tecla(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
	case "esc":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc})
	}
	r := []rune(s)
	return tea.KeyPressMsg(tea.Key{Text: s, Code: r[0]})
}

func pulsar(m Model, keys ...string) Model {
	for _, k := range keys {
		out, _ := m.Update(tecla(k))
		m = out.(Model)
	}
	return m
}

// escribir tipea una clave en el prompt y la confirma con Enter.
func escribir(m Model, texto string) Model {
	for _, r := range texto {
		out, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: string(r), Code: r}))
		m = out.(Model)
	}
	return pulsar(m, "enter")
}

// La tecla 1 abre confirmación y al confirmar pide las claves que faltan.
func TestTecla1PideLasClavesQueFaltan(t *testing.T) {
	t.Setenv(envSAPassword, "")
	t.Setenv(envAppPassword, "")

	m := pulsar(NewModel(devCfg(), ""), "1")
	if m.screen != screenConfirmInstall {
		t.Fatalf("pantalla = %v, quiero screenConfirmInstall", m.screen)
	}
	m = pulsar(m, "enter")
	if m.screen != screenAsk {
		t.Fatalf("pantalla = %v, quiero screenAsk (prompt de claves)", m.screen)
	}
	if len(m.askQueue) != 1 {
		t.Fatalf("cola = %v, quiero 1 clave (app)", m.askQueue)
	}
	if m.askQueue[0] != envAppPassword {
		t.Errorf("orden = %v, quiero [%s]", m.askQueue, envAppPassword)
	}
}

// Con las claves ya en el entorno no hay nada que preguntar: arranca derecho tras confirmación.
func TestTecla1ConClavesEnEntornoNoPregunta(t *testing.T) {
	t.Setenv(envSAPassword, "Sa*2026*Dev")
	t.Setenv(envAppPassword, "Dv*123")

	m := pulsar(NewModel(devCfg(), ""), "1")
	if m.screen != screenConfirmInstall {
		t.Fatalf("pantalla = %v, quiero screenConfirmInstall", m.screen)
	}
	m = pulsar(m, "enter")
	if m.screen != screenWorking {
		t.Fatalf("pantalla = %v, quiero screenWorking", m.screen)
	}
	if m.task != taskInstall {
		t.Errorf("tarea = %v, quiero taskInstall", m.task)
	}
}

// Prod usa Windows Auth: nunca guarda claves, asi que nunca las pide.
func TestTecla1EnProdNoPideNada(t *testing.T) {
	t.Setenv(envSAPassword, "")
	t.Setenv(envAppPassword, "")

	m := pulsar(NewModel(prodCfg(), ""), "1")
	if m.screen != screenConfirmInstall {
		t.Fatalf("pantalla = %v, quiero screenConfirmInstall", m.screen)
	}
	m = pulsar(m, "enter")
	if m.screen != screenWorking || m.task != taskInstall {
		t.Fatalf("prod pidio claves: pantalla=%v tarea=%v", m.screen, m.task)
	}
	if len(m.askQueue) != 0 {
		t.Errorf("prod encolo %v", m.askQueue)
	}
}

// El prompt guarda la clave y arranca la instalación.
func TestPromptPideUnaYDespuesLaOtra(t *testing.T) {
	t.Setenv(envSAPassword, "")
	t.Setenv(envAppPassword, "")

	m := pulsar(NewModel(devCfg(), ""), "1")
	m = pulsar(m, "enter")
	m = escribir(m, "Dv*123")

	if m.secrets[envAppPassword] != "Dv*123" {
		t.Errorf("no guardo la de app: %v", m.secrets)
	}
	if m.screen != screenWorking || m.task != taskInstall {
		t.Errorf("pantalla=%v tarea=%v, quiero arrancar la instalacion", m.screen, m.task)
	}
}

// La validacion del prompt tiene que cortar ANTES, no despues del RESTORE.
func TestPromptRechazaClaveQueElParcheNoPuedeEmbeber(t *testing.T) {
	t.Setenv(envSAPassword, "Sa*2026*Dev")
	t.Setenv(envAppPassword, "")

	m := pulsar(NewModel(devCfg(), ""), "1")
	m = pulsar(m, "enter")
	m = escribir(m, "123456789") // 9 chars con user dev = 21 > 20
	if m.screen != screenAsk {
		t.Fatalf("pantalla = %v, arranco con una clave invalida", m.screen)
	}
	if m.askErr == "" {
		t.Error("no mostro error al operador")
	}
}

// Esc cancela el prompt sin dejar claves guardadas ni arrancar nada.
func TestPromptEscCancela(t *testing.T) {
	t.Setenv(envSAPassword, "")
	t.Setenv(envAppPassword, "")

	m := pulsar(NewModel(devCfg(), ""), "1")
	m = pulsar(m, "enter")
	m = pulsar(m, "esc")

	if m.screen != screenMenu {
		t.Fatalf("pantalla = %v, quiero volver al menu", m.screen)
	}
	if len(m.secrets) != 0 {
		t.Errorf("guardo claves al cancelar: %v", m.secrets)
	}
	if m.askQueue != nil {
		t.Errorf("quedo cola pendiente: %v", m.askQueue)
	}
}

// Las teclas del menu son las que dice la ayuda y no se pisan entre si.
func TestTeclasDelMenuEjecutanSuAccion(t *testing.T) {
	t.Setenv(envSAPassword, "Sa*2026*Dev")
	t.Setenv(envAppPassword, "Dv*123")

	// Tecla 1 es Instalación completa (abre confirmación)
	m1 := pulsar(NewModel(devCfg(), ""), "1")
	if m1.screen != screenConfirmInstall {
		t.Errorf("tecla 1 -> pantalla %v, quiero screenConfirmInstall", m1.screen)
	}

	// Tecla 2 es Reparar
	m2 := pulsar(NewModel(devCfg(), ""), "2")
	if m2.task != taskRepair {
		t.Errorf("tecla 2 -> tarea %v, quiero taskRepair", m2.task)
	}
	if m2.screen != screenWorking {
		t.Errorf("tecla 2 -> pantalla %v, quiero screenWorking", m2.screen)
	}

	// Tecla 3 es Verificar (check)
	m3 := pulsar(NewModel(devCfg(), ""), "3")
	if m3.task != taskCheck {
		t.Errorf("tecla 3 -> tarea %v, quiero taskCheck", m3.task)
	}
	if m3.screen != screenWorking {
		t.Errorf("tecla 3 -> pantalla %v, quiero screenWorking", m3.screen)
	}

	// Tecla 4 es Configuración avanzada
	m4 := pulsar(NewModel(devCfg(), ""), "4")
	if m4.screen != screenAdvanced {
		t.Errorf("tecla 4 -> pantalla %v, quiero screenAdvanced", m4.screen)
	}
}

// viewAsk indexa la cola: si alguna vez queda vacia, no puede paniquear en la
// cara del operador.
func TestViewAskSinColaNoPaniquea(t *testing.T) {
	m := NewModel(devCfg(), "")
	m.screen = screenAsk
	m.askQueue = nil
	if v := m.View(); v.Content == "" {
		t.Error("viewAsk devolvio vacio")
	}
}

// Cualquier pantalla del TUI tiene que poder dibujarse.
func TestTodasLasPantallasSeDibujan(t *testing.T) {
	for _, s := range []screen{screenMenu, screenWorking, screenDone, screenAsk, screenChecklist, screenPerfil, screenServer} {
		m := NewModel(devCfg(), "")
		m.screen = s
		if s == screenAsk {
			m.askQueue = []string{envSAPassword}
		}
		if v := m.View(); v.Content == "" {
			t.Errorf("pantalla %v dibujo vacio", s)
		}
	}
}
