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

// El punto 1 del pedido: la tecla 0 arranca la instalacion completa y lo primero
// que hace, si faltan claves, es pedirlas (una sola vez) en vez de fallar
// a mitad del RESTORE.
func TestTecla0PideLasClavesQueFaltan(t *testing.T) {
	t.Setenv(envSAPassword, "")
	t.Setenv(envAppPassword, "")

	m := pulsar(NewModel(devCfg(), ""), "0")
	if m.screen != screenAsk {
		t.Fatalf("pantalla = %v, quiero screenAsk (prompt de claves)", m.screen)
	}
	if len(m.askQueue) != 2 {
		t.Fatalf("cola = %v, quiero las 2 claves", m.askQueue)
	}
	if m.askQueue[0] != envSAPassword || m.askQueue[1] != envAppPassword {
		t.Errorf("orden = %v, quiero [%s %s]", m.askQueue, envSAPassword, envAppPassword)
	}
}

// Con las claves ya en el entorno no hay nada que preguntar: arranca derecho.
func TestTecla0ConClavesEnEntornoNoPregunta(t *testing.T) {
	t.Setenv(envSAPassword, "Sa*2026*Dev")
	t.Setenv(envAppPassword, "Dv*123")

	m := pulsar(NewModel(devCfg(), ""), "0")
	if m.screen != screenWorking {
		t.Fatalf("pantalla = %v, quiero screenWorking", m.screen)
	}
	if m.task != taskInstall {
		t.Errorf("tarea = %v, quiero taskInstall", m.task)
	}
}

// Prod usa Windows Auth: nunca guarda claves, asi que nunca las pide.
func TestTecla0EnProdNoPideNada(t *testing.T) {
	t.Setenv(envSAPassword, "")
	t.Setenv(envAppPassword, "")

	m := pulsar(NewModel(prodCfg(), ""), "0")
	if m.screen != screenWorking || m.task != taskInstall {
		t.Fatalf("prod pidio claves: pantalla=%v tarea=%v", m.screen, m.task)
	}
	if len(m.askQueue) != 0 {
		t.Errorf("prod encolo %v", m.askQueue)
	}
}

// El prompt guarda la clave y pasa a la siguiente, en orden.
func TestPromptPideUnaYDespuesLaOtra(t *testing.T) {
	t.Setenv(envSAPassword, "")
	t.Setenv(envAppPassword, "")

	m := pulsar(NewModel(devCfg(), ""), "0")
	m = escribir(m, "Sa*2026*Dev")

	if m.secrets[envSAPassword] != "Sa*2026*Dev" {
		t.Errorf("no guardo la SA: %v", m.secrets)
	}
	if len(m.askQueue) != 1 || m.askQueue[0] != envAppPassword {
		t.Fatalf("cola = %v, quiero solo %s", m.askQueue, envAppPassword)
	}
	if m.screen != screenAsk {
		t.Fatalf("pantalla = %v, quiero seguir en screenAsk", m.screen)
	}

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

	m := pulsar(NewModel(devCfg(), ""), "0")
	m = escribir(m, "123456789") // 9 chars con user dev = 21 > 20

	if m.screen != screenAsk {
		t.Fatalf("pantalla = %v, arranco con una clave invalida", m.screen)
	}
	if m.askErr == "" {
		t.Fatal("no explico por que la rechaza")
	}
	if m.secrets[envAppPassword] != "" {
		t.Error("guardo una clave que el parche va a rechazar")
	}
}

func TestEscEnElPromptCancelaSinGuardar(t *testing.T) {
	t.Setenv(envSAPassword, "")
	t.Setenv(envAppPassword, "")

	m := pulsar(NewModel(devCfg(), ""), "0", "esc")
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

	quiero := map[string]taskKind{
		"0": taskInstall,
		"1": taskSetupDB,
		"2": taskSetupApp,
		"3": taskCheck,
	}
	for k, want := range quiero {
		m := pulsar(NewModel(devCfg(), ""), k)
		if m.task != want {
			t.Errorf("tecla %q -> tarea %v, quiero %v", k, m.task, want)
		}
		if m.screen != screenWorking {
			t.Errorf("tecla %q -> pantalla %v, quiero screenWorking", k, m.screen)
		}
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
	for _, s := range []screen{screenMenu, screenWorking, screenDone, screenAsk, screenChecklist} {
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
