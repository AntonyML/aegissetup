// © Antony Monge López — Costa Rica — Céd. 604700548
package ui

import (
	"os"

	"aegis-setup/internal/setup"

	tea "charm.land/bubbletea/v2"
)

// El flujo de permisos sale a variables para poder probar la pantalla sin un cartel
// de UAC de por medio: aceptar un pedido de permisos es una acción del operador, no
// algo que una prueba pueda apretar.
var (
	soyAdmin  = setup.IsAdmin
	sinElevar = setup.SkipElevation
	relanzar  = setup.RelaunchElevated
)

// pedirPermisos relanza el menú con permisos de administrador y cierra esta ventana.
//
// No se eleva al arrancar a propósito: check, checklist y dashboard tienen que poder
// correr sin permisos, que es justo cuando más se necesitan. Se pide cuando el
// operador quiere algo que escribe en la máquina, y desde acá el pedido es explícito
// (apretar [E]) en vez de un cartel de UAC que aparece sin explicación.
func (m Model) pedirPermisos() (tea.Model, tea.Cmd) {
	switch {
	case soyAdmin():
		return m.avisoDePermisos("Aegis ya está corriendo con permisos de administrador: no hace falta nada más."), nil
	case sinElevar():
		// Esta corrida ya es el relanzado elevado, o alguien la pidió así. Volver a
		// relanzar sería un cartel de UAC detrás del otro, sin salida.
		return m.avisoDePermisos("Esta ventana ya se abrió sin pedir permisos (AEGIS_NO_ELEVAR). Cerrá Aegis y abrilo con click derecho → Ejecutar como administrador."), nil
	}

	if err := relanzar(os.Args); err != nil {
		return m.avisoDePermisos("No se pudo pedir permisos de administrador: " + err.Error()), nil
	}
	// Se cierra esta ventana: si quedara viva, el operador terminaría con dos Aegis
	// sobre la misma máquina, cada uno con su checklist a medias.
	return m.avisoDePermisos("Se abrió Aegis en otra ventana con permisos de administrador. Esta se cierra: seguí en la otra."), tea.Quit
}

// avisoDePermisos muestra el resultado del pedido en la pantalla de resultado, que ya
// sabe dibujar un mensaje y volver al menú con Esc.
func (m Model) avisoDePermisos(texto string) Model {
	m.screen = screenDone
	m.taskName = "PERMISOS"
	m.taskErr = nil
	m.bloqueo = false
	m.lines = []string{texto}
	return m
}
