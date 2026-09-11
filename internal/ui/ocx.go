// © Antony Monge López — Costa Rica — Céd. 604700548
package ui

import (
	"io/fs"

	"aegis-setup/assets"
	"aegis-setup/internal/setup"
)

// El origen y el destino de los controles OCX son variables por la misma razón que los
// del runtime de Crystal: el valor por defecto copia 22 archivos a SysWOW64 y registra
// 11 controles COM de verdad, y una prueba del orden de los pasos de Setup App no tiene
// que tocar la máquina donde se desarrolla.
var (
	// ocxFS es el kit de controles que viaja dentro del propio EXE.
	ocxFS fs.FS = assets.OCX()
	// ocxDir es dónde se instalan: el SysWOW64 de la PC destino.
	ocxDir = setup.SysWOW64
)

// instalarOCX copia los controles desde el propio binario y registra los requeridos.
//
// El origen es el binario y no la carpeta de la PC vieja: en una PC limpia esa carpeta
// no existe, así que depender de ella obligaba a copiar 22 archivos a mano antes de
// poder instalar. legacyDir queda como origen extra para lo que no venga adentro.
//
// Devuelve lo que falló para que lo nombre el llamador, igual que el runtime de Crystal:
// el CLI y el TUI no tienen por qué inventar el mismo prefijo en dos lugares.
func instalarOCX(legacyDir string, emit func(string)) []string {
	origen := setup.OrigenOCX{Binario: ocxFS, Carpeta: legacyDir}
	return setup.InstallOCX(origen, ocxDir, registrarCOM, emit)
}
