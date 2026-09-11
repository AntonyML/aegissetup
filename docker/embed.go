// © Antony Monge López — Costa Rica — Céd. 604700548
package docker

import (
	"embed"
	"io/fs"
)

// El compose del SQL de pruebas viaja adentro del ejecutable.
//
// El paquete vive dentro de docker/ porque //go:embed no acepta "..": el patrón solo puede
// nombrar archivos del árbol del paquete, y este es el único lugar desde donde se pueden
// nombrar estos archivos.
//
// Los patrones están escritos uno por uno y no con un comodín. Un comodín se lleva .env,
// que tiene la clave del SA de la máquina del desarrollador y no se versiona: embebido
// viajaría dentro de cada EXE que se reparta. La lista corta es la garantía de que el
// secreto no sale de acá.
//
//go:embed docker-compose.yml GUIA-DOCKER.txt init
//go:embed all:.env.example
var contenido embed.FS

// FS devuelve el kit de Docker tal como viaja en el binario: docker-compose.yml,
// GUIA-DOCKER.txt, .env.example e init/ con los scripts de arranque. Sin .env.
func FS() fs.FS {
	return contenido
}
