// © Antony Monge López — Costa Rica — Céd. 604700548
package cli

import (
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
	// escribirDSN crea el System DSN de 32 bits en HKLM y parcheDocker reescribe el
	// .exe de SIDC. Son variables para poder probar el paso 2 completo sin tocar el
	// registro ni el binario de la app de la máquina donde se desarrolla.
	escribirDSN  = setup.WriteDSN
	parcheDocker = setup.PatchDockerExe
)

// InstalarCrystal copia el runtime completo a SysWOW64 y registra los 4 componentes
// COM que los reportes necesitan. Devuelve lo que quedó pendiente: igual que los OCX,
// un archivo que no se pudo poner no aborta el resto de la instalación.
func InstalarCrystal(out func(string)) []string {
	res, err := setup.InstallCrystal(crystalFS, crystalDir, registrarCOM, out)
	if err != nil {
		return []string{err.Error()}
	}
	return res.Fallos
}
