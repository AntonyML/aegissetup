// © Antony Monge López — Costa Rica — Céd. 604700548
package main

import (
	"os"

	// Import en blanco: obliga a enlazar el runtime embebido en el binario.
	// Sin esta línea, el //go:embed de assets/ es código muerto (Go descarta los
	// paquetes que nadie importa) y el EXE sale sin Crystal Reports ni OCX.
	_ "aegis-setup/assets"

	"aegis-setup/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
