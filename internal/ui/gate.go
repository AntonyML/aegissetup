// © Antony Monge López — Costa Rica — Céd. 604700548
package ui

import (
	"fmt"
	"strings"

	"aegis-setup/internal/precheck"
)

// etapaDe traduce una acción del menú a la etapa del flujo que representa. Las
// acciones que no son etapas (diagnóstico y presets) no tienen puerta.
func etapaDe(a action) (precheck.Etapa, bool) {
	switch a {
	case actInstall, actRepair:
		return precheck.EtapaInstall, true
	case actSetupDB:
		return precheck.EtapaSetupDB, true
	case actSetupApp:
		return precheck.EtapaSetupApp, true
	}
	return "", false
}

// gate dice qué requisitos tienen que estar OK antes de correr cada acción.
//
// El "qué traba qué" no se decide acá: vive en precheck.trabaPorRequisito, que
// es el mismo que alimenta el checklist de la CLI. Acá solo se traduce una tecla
// del menú a la etapa del flujo que le corresponde, para que la TUI y el
// diagnóstico no puedan discrepar sobre quién desbloquea a quién.
//
// Check y los presets no tienen etapa, y por eso no tienen puerta: son el
// diagnóstico y la configuración, y tienen que seguir disponibles justo cuando
// todo lo demás está roto. Si el check se bloqueara, el operador perdería la
// herramienta que le dice qué arreglar.
func gate(a action) []precheck.ID {
	e, ok := etapaDe(a)
	if !ok {
		return nil
	}
	return precheck.BloqueantesDe(e)
}

// bloqueosDe devuelve los requisitos que impiden correr la acción.
//
// Un checklist vacío no bloquea nada: significa que las sondas todavía no
// corrieron. Preferimos dejar pasar y que el error salga del paso mismo, antes
// que encerrar al operador por una sonda lenta.
func bloqueosDe(a action, rs []precheck.Requisito) []precheck.Requisito {
	if len(rs) == 0 {
		return nil
	}
	var out []precheck.Requisito
	for _, id := range gate(a) {
		r, ok := precheck.Buscar(rs, id)
		// Los avisos no bloquean: son cosas que se resuelven solas en un paso
		// posterior del propio flujo.
		if ok && r.Estado == precheck.EstadoFalta {
			out = append(out, r)
		}
	}
	return out
}

// accionesQueDependenDe dice qué acciones del menú quedan trabadas por un
// requisito. Es lo que convierte el checklist en un plan: sin esto, el operador
// ve un rojo y no sabe para qué sirve resolverlo.
func accionesQueDependenDe(id precheck.ID) []string {
	traba := precheck.Traba(id)
	var out []string
	for _, it := range menuItems {
		e, ok := etapaDe(it.action)
		if !ok {
			continue
		}
		for _, t := range traba {
			if t == e {
				out = append(out, it.name)
				break
			}
		}
	}
	return out
}

// resumenChecklist es la línea que va arriba del menú.
func resumenChecklist(rs []precheck.Requisito, verificando bool) string {
	if verificando || len(rs) == 0 {
		return "CHECKLIST: verificando la máquina..."
	}
	faltan := precheck.Bloqueantes(rs)
	if len(faltan) == 0 {
		if len(rs) == 0 {
			return "CHECKLIST: verificando la máquina..."
		}
		return fmt.Sprintf("CHECKLIST: %d requisitos, ninguno bloquea la instalación", len(rs))
	}
	// "1 bloquean" se lee como un bug del programa, no como un dato.
	verbo := "bloquean"
	if len(faltan) == 1 {
		verbo = "bloquea"
	}
	return fmt.Sprintf("CHECKLIST: %d %s (%s) - [C] ver cómo resolverlos", len(faltan), verbo, titulosDe(faltan))
}

// resumenFaltantes es el pie del checklist. El orden de la lista es el mensaje:
// resolver de arriba hacia abajo es lo que va destrabando el resto.
func resumenFaltantes(n int) string {
	if n == 1 {
		return "Falta 1 requisito. Resolvelo y volvé a mirar el checklist."
	}
	return fmt.Sprintf("Faltan %d requisitos. Resolvelos en el orden de la lista: los de arriba destraban a los de abajo.", n)
}

// titulosDe lista los títulos en una línea, con el ID de respaldo.
func titulosDe(rs []precheck.Requisito) string {
	xs := make([]string, 0, len(rs))
	for _, r := range rs {
		if r.Titulo != "" {
			xs = append(xs, r.Titulo)
			continue
		}
		xs = append(xs, string(r.ID))
	}
	return strings.Join(xs, ", ")
}

// arreglosDe junta los pasos a seguir de varios requisitos. Se usa cuando una
// acción queda bloqueada: el operador recibe la lista de lo que tiene que hacer,
// no un "no se puede".
func arreglosDe(rs []precheck.Requisito) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Titulo+": "+r.Arreglo)
	}
	return out
}

// nombreDeAccion es el nombre del menú de una acción, para los mensajes.
func nombreDeAccion(a action) string {
	for _, it := range menuItems {
		if it.action == a {
			return it.name
		}
	}
	return "esa opción"
}
