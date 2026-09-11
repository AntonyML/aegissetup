// © Antony Monge López — Costa Rica — Céd. 604700548
package main

import (
	"os"

	"aegis-setup/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
