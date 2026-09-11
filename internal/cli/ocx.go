// © Antony Monge López — Costa Rica — Céd. 604700548
package cli

import (
	"io/fs"

	"aegis-setup/assets"
	"aegis-setup/internal/setup"
)

// El origen y el destino de los controles OCX son variables por la misma razón que los
// del runtime de Crystal: el valor por defecto copia 22 archivos a SysWOW64 y registra
// 11 controles COM de verdad, y una prueba del paso 2 no tiene que tocar la máquina
// donde se desarrolla.
var (
	// ocxFS es el kit de controles que viaja dentro del propio EXE.
	ocxFS fs.FS = assets.OCX()
	// ocxDir es dónde se instalan: el SysWOW64 de la PC destino.
	ocxDir = setup.SysWOW64
)

// InstalarOCX copia los controles desde el propio binario y registra los requeridos.
// legacyDir queda como origen extra para el control que no venga adentro. Devuelve lo
// que falló: un control que no se pudo poner no aborta el resto de la instalación.
func InstalarOCX(legacyDir string, out func(string)) []string {
	origen := setup.OrigenOCX{Binario: ocxFS, Carpeta: legacyDir}
	return setup.InstallOCX(origen, ocxDir, registrarCOM, out)
}
