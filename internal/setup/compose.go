// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"io/fs"
	"os"
	"path/filepath"
)

// InstallDockerAssets deja el kit de Docker (compose + scripts de init) en dir, listo para
// que el paso siguiente sea `docker compose -f <dir>\docker-compose.yml up -d`.
//
// Se compara contenido y no fecha: un compose viejo en la carpeta destino es peor que no
// tenerlo, porque el motor levanta con la configuración anterior y el operador mide una
// máquina que no es la que cree. Lo que ya está igual no se toca, así un ajuste a mano
// (puerto, memoria, collation) no se pierde en la corrida siguiente.
//
// Un archivo que no se puede escribir no aborta el resto: se acumula y sigue, y el operador
// ve de una sola vez todo lo que falta. Devuelve la lista de fallos nombrando el archivo.
func InstallDockerAssets(origen fs.FS, dir string, out func(string)) []string {
	if out == nil {
		out = func(string) {}
	}
	if origen == nil {
		return []string{"el kit de Docker no está en este binario (no se puede levantar el SQL de pruebas)"}
	}

	var fallos []string
	err := fs.WalkDir(origen, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			fallos = append(fallos, p+" (no se pudo leer del binario: "+err.Error()+")")
			return nil
		}
		if d.IsDir() {
			return nil
		}
		dst := filepath.Join(dir, filepath.FromSlash(p))
		if iguales(origen, p, dst) {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			fallos = append(fallos, p+" (no se pudo crear "+filepath.Dir(dst)+", corré como Admin: "+err.Error()+")")
			return nil
		}
		if err := extraer(origen, p, dst); err != nil {
			fallos = append(fallos, p+" (no se pudo copiar a "+dst+", corré como Admin: "+err.Error()+")")
			return nil
		}
		out("DOCKER OK: " + p)
		return nil
	})
	if err != nil {
		fallos = append(fallos, dir+" (no se pudo recorrer el kit: "+err.Error()+")")
	}
	if len(fallos) == 0 && vacio(origen) {
		fallos = append(fallos, "el kit de Docker embebido está vacío: este EXE se armó mal")
	}
	return fallos
}

// vacio dice si el kit no trae ningún archivo. Un binario sin el embed no falla al
// recorrerlo: simplemente no recorre nada, y sin este chequeo la instalación terminaría
// diciendo que salió todo bien dejando la carpeta vacía.
func vacio(origen fs.FS) bool {
	n := 0
	_ = fs.WalkDir(origen, ".", func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
		}
		return nil
	})
	return n == 0
}
