// © Antony Monge López — Costa Rica — Céd. 604700548
package ui

import (
	"fmt"
	"io/fs"

	"aegis-setup/assets"
	"aegis-setup/internal/setup"
)

// El origen, el destino y el registro del runtime de Crystal son variables para que
// las pruebas puedan instalar en un directorio temporal: el valor por defecto escribe
// 24 MB en el SysWOW64 de la máquina y registra componentes COM de verdad.
var (
	// crystalFS es el runtime que viaja dentro del propio EXE (F2).
	crystalFS fs.FS = assets.Crystal()
	// crystalDir es dónde se instala: el SysWOW64 de la PC destino.
	crystalDir = setup.SysWOW64
	// registrarCOM es el regsvr32 de 32 bits.
	registrarCOM = setup.RegisterCOM
	// parcheDocker reescribe el .exe de SIDC. Es una variable por lo mismo que el DSN:
	// una prueba del orden de los pasos no tiene que tocar el binario de la app.
	parcheDocker = setup.PatchDockerExe
	// escribirDSN crea el System DSN de 32 bits en HKLM. Es una variable para poder
	// probar el paso de Setup App de punta a punta sin escribir en el registro de la
	// máquina donde se desarrolla.
	escribirDSN = setup.WriteDSN
)

// instalarCrystal copia el runtime completo a SysWOW64 y registra los 4 componentes
// COM que los reportes necesitan.
//
// Un error duro (runtime vacío, destino inaccesible) no tumba Setup App: la app de
// SIDC queda instalada igual y el checklist dice qué falta. Abortar acá dejaría la
// instalación a medias por un problema que se arregla reabriendo Aegis como
// administrador.
func instalarCrystal(emit func(string)) {
	res, err := setup.InstallCrystal(crystalFS, crystalDir, registrarCOM, emit)
	if err != nil {
		emit("CRYSTAL PENDIENTE: " + err.Error())
		return
	}
	for _, f := range res.Fallos {
		emit("CRYSTAL PENDIENTE: " + f)
	}
	emit("CRYSTAL: " + resumenCrystal(res))
}

// resumenCrystal arma la línea que ve el operador. Los conteos importan: "43
// archivos" con el runtime incompleto en el binario se nota de un vistazo, y
// distinguir copiados de ya estaban explica por qué una instalación fue instantánea.
func resumenCrystal(res setup.CrystalResult) string {
	linea := fmt.Sprintf("%d copiados, %d ya estaban, %d registrados", len(res.Copiados), len(res.YaEstaban), len(res.Registrados))
	if n := len(res.Fallos); n > 0 {
		linea += fmt.Sprintf(", %d pendientes", n)
	}
	return linea
}
