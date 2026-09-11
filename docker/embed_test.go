// © Antony Monge López — Costa Rica — Céd. 604700548
package docker

import (
	"io/fs"
	"testing"
)

// Los archivos que hacen falta para levantar el SQL de pruebas tienen que viajar dentro
// del ejecutable. En una PC que no tiene el repo no existe AegisSetup/docker, y esa PC es
// justamente donde se instala SIDC: un compose que solo vive en el repo obliga a copiar
// carpetas a mano, que es el requisito externo que F7 saca del medio.
func TestElComposeViajaEnElBinario(t *testing.T) {
	raiz := FS()
	for _, f := range []string{"docker-compose.yml", "GUIA-DOCKER.txt", ".env.example", "init/01-sidc.sql"} {
		if _, err := fs.Stat(raiz, f); err != nil {
			t.Errorf("falta %s en el embed: %v", f, err)
		}
	}
}

// El .env de la máquina del desarrollador tiene la clave del SA y no se versiona
// (.gitignore). Embebido viajaría dentro de cada EXE que se reparta, así que la lista de
// patrones del embed no puede ser un comodín: el .env no se copia nunca.
func TestLaClaveDelDesarrolladorNoViajaEnElBinario(t *testing.T) {
	if _, err := fs.Stat(FS(), ".env"); err == nil {
		t.Fatal(".env quedó embebido: la clave del SA viaja dentro del EXE que se reparte")
	}
}
