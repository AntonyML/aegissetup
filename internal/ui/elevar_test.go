// © Antony Monge López — Costa Rica — Céd. 604700548
package ui

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// stubElevacion deja el flujo de permisos bajo control del test y devuelve la lista
// de relanzamientos pedidos. Sin esto habría que aceptar un cartel de UAC para
// probar una pantalla.
func stubElevacion(t *testing.T, admin bool, err error) *[]string {
	t.Helper()
	prevAdmin, prevSkip, prevRel := soyAdmin, sinElevar, relanzar
	t.Cleanup(func() { soyAdmin, sinElevar, relanzar = prevAdmin, prevSkip, prevRel })

	llamadas := &[]string{}
	soyAdmin = func() bool { return admin }
	sinElevar = func() bool { return false }
	relanzar = func(args []string) error {
		*llamadas = append(*llamadas, strings.Join(args, " "))
		return err
	}
	return llamadas
}

// El operador apretó [E] sin necesitarlo: se le dice y no se relanza nada. Relanzar
// de gusto abriría una ventana nueva sin motivo.
func TestENoRelanzaSiYaHayPermisos(t *testing.T) {
	llamadas := stubElevacion(t, true, nil)

	m := pulsar(NewModel(devCfg(), ""), "e")
	if len(*llamadas) != 0 {
		t.Errorf("relanzó %v aunque ya tenía permisos", *llamadas)
	}
	if !strings.Contains(strings.Join(m.lines, " "), "administrador") {
		t.Errorf("líneas = %v, quiero que diga que ya tiene los permisos", m.lines)
	}
}

// El caso real: sin permisos, [E] abre Aegis elevado y esta terminal se cierra.
// Que el padre se cierre no es cosmético: si siguiera vivo, el operador tendría dos
// Aegis corriendo sobre la misma máquina.
func TestERelanzaElevadoYCierraElTui(t *testing.T) {
	llamadas := stubElevacion(t, false, nil)

	m := NewModel(devCfg(), "")
	out, cmd := m.Update(tecla("e"))
	m = out.(Model)

	quiero := strings.Join(os.Args, " ")
	if len(*llamadas) != 1 || (*llamadas)[0] != quiero {
		t.Errorf("relanzó con %v, quiero [%q]", *llamadas, quiero)
	}
	if cmd == nil {
		t.Fatal("no cerró el TUI después de relanzar")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("el comando no fue Quit: %T", cmd())
	}
	if !strings.Contains(strings.Join(m.lines, " "), "otra ventana") {
		t.Errorf("líneas = %v, quiero que diga dónde seguir", m.lines)
	}
}

// AEGIS_NO_ELEVAR marca al proceso que ya es el relanzado. Si igual intentara
// elevarse, cada cartel de UAC abriría otro: el operador no podría salir.
func TestENoRelanzaSiEstaCorridaYaSeRelanzo(t *testing.T) {
	llamadas := stubElevacion(t, false, nil)
	sinElevar = func() bool { return true }

	m := pulsar(NewModel(devCfg(), ""), "e")
	if len(*llamadas) != 0 {
		t.Errorf("volvió a relanzar (%v) en la corrida ya elevada", *llamadas)
	}
	if !strings.Contains(strings.Join(m.lines, " "), "administrador") {
		t.Errorf("líneas = %v, quiero la instrucción manual", m.lines)
	}
}

// Cancelar el UAC deja un error concreto (1223). Si el TUI lo tapara, el operador
// vería la misma pantalla que antes y no entendería por qué no pasó nada.
func TestSiSeCancelaElUacSeDiceYNoSeCierra(t *testing.T) {
	stubElevacion(t, false, errors.New("el operador canceló el pedido de permisos"))

	m := NewModel(devCfg(), "")
	out, cmd := m.Update(tecla("e"))
	m = out.(Model)

	if cmd != nil {
		t.Error("cerró el TUI aunque la elevación no se hizo")
	}
	if !strings.Contains(strings.Join(m.lines, " "), "cancel") {
		t.Errorf("líneas = %v, quiero el motivo de la cancelación", m.lines)
	}
}

// sinColor deja el texto sin las secuencias ANSI. Glamour corta el resaltado en
// pedazos ("[", "E]"), así que buscar "[E]" en la salida cruda solo funciona por
// casualidad: lo que importa es lo que se lee, no en cuántos tramos viene.
var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")

func sinColor(s string) string { return ansiRe.ReplaceAllString(s, "") }

// La puerta manda a resolver los permisos: si la ayuda no dice cómo, el operador
// termina buscando el .exe para hacer click derecho. Esta tecla es el camino.
func TestLaAyudaMencionaLaTeclaE(t *testing.T) {
	if !strings.Contains(sinColor(renderHelp(DefaultStyles())), "[E]") {
		t.Error("la ayuda no menciona [E] para pedir permisos")
	}
	if !strings.Contains(sinColor(NewModel(devCfg(), "").View().Content), "[E]") {
		t.Error("el menú no muestra [E] en la barra de teclas")
	}
}

// [E] también sirve con el resultado de una acción trabada a la vista, que es
// justamente donde el checklist dice "falta: permisos de administrador".
func TestETambienFuncionaConLaPantallaDeResultado(t *testing.T) {
	llamadas := stubElevacion(t, false, nil)

	m := NewModel(devCfg(), "")
	m.screen = screenDone
	m.bloqueo = true

	out, _ := m.Update(tecla("e"))
	if len(*llamadas) != 1 {
		t.Errorf("relanzó %v veces, quiero 1 desde la pantalla de resultado", len(*llamadas))
	}
	if out.(Model).taskName != "PERMISOS" {
		t.Errorf("pantalla = %q, quiero el aviso de permisos", out.(Model).taskName)
	}
}
