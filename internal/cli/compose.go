// © Antony Monge López — Costa Rica — Céd. 604700548
package cli

import (
	"io/fs"

	"aegis-setup/docker"
	"aegis-setup/internal/config"
	"aegis-setup/internal/setup"
)

// El kit de Docker viaja dentro del propio EXE y se deja en la carpeta del producto. Son
// variables por la misma razón que las de Crystal y los OCX: el valor por defecto escribe en
// C:\ProgramData, y una prueba del paso 1 no tiene que tocar la máquina donde se desarrolla.
var (
	// composeFS es el kit que viaja en el binario, sin el .env del desarrollador.
	composeFS fs.FS = docker.FS()
	// composeDir es dónde se deja: donde el operador corre docker compose después.
	composeDir = config.DirDocker()
)

// InstalarCompose deja el compose listo en la PC destino. Solo aplica a db_mode=docker: en
// local y server el motor es un SQL Server que ya está instalado en la máquina.
func InstalarCompose(dbMode config.DbMode, out func(string)) []string {
	if dbMode != config.DbDocker {
		return nil
	}
	return setup.InstallDockerAssets(composeFS, composeDir, out)
}
