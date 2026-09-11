// © Antony Monge López — Costa Rica — Céd. 604700548
package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestScrollEnPantallasLargas(t *testing.T) {
	m := NewModel(devCfg(), "")
	// Simulamos una terminal pequeña de 80x15
	out, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 15})
	m = out.(Model)

	// 1. Probar Help con scroll
	m.screen = screenHelp
	m.helpHTML = renderHelp(m.styles)
	m = m.setupViewport(m.helpHTML, 0, 2)

	if m.viewport.YOffset() != 0 {
		t.Fatalf("help arrancó con offset %d, quiero 0", m.viewport.YOffset())
	}
	m = pulsar(m, "down", "down")
	if m.viewport.YOffset() == 0 {
		t.Errorf("help no scrolleó hacia abajo tras presionar 'down'")
	}
	m = pulsar(m, "up", "up")
	if m.viewport.YOffset() != 0 {
		t.Errorf("help no volvió arriba tras presionar 'up'")
	}

	// 2. Probar Done con muchas líneas
	m.screen = screenDone
	m.lines = make([]string, 50)
	for i := range m.lines {
		m.lines[i] = "Línea de log larga..."
	}
	m = m.setupViewport(m.doneBody(), 4, 2)

	if m.viewport.YOffset() != 0 {
		t.Fatalf("done arrancó con offset %d, quiero 0", m.viewport.YOffset())
	}
	m = pulsar(m, "down", "down", "down")
	if m.viewport.YOffset() == 0 {
		t.Errorf("done no scrolleó hacia abajo tras presionar 'down'")
	}

	// Esc debe volver al menú
	m = pulsar(m, "esc")
	if m.screen != screenMenu {
		t.Errorf("esc en done no volvió al menú: pantalla = %v", m.screen)
	}

	// 3. Probar Checklist con scroll
	m.screen = screenChecklist
	m = m.setupViewport(m.checklistBody(), 4, 2)
	m = pulsar(m, "down")
	// Enter debe volver al menú
	m = pulsar(m, "enter")
	if m.screen != screenMenu {
		t.Errorf("enter en checklist no volvió al menú: pantalla = %v", m.screen)
	}
}

func TestViewDoneIncluyeTextoDeScroll(t *testing.T) {
	m := NewModel(devCfg(), "")
	out, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m = out.(Model)
	m.screen = screenDone
	m.lines = []string{"Resultado 1", "Resultado 2"}
	m = m.setupViewport(m.doneBody(), 4, 2)

	v := m.viewDone()
	if !strings.Contains(v, "desplazar") {
		t.Errorf("viewDone no incluye ayuda de desplazamiento: %s", v)
	}
}
