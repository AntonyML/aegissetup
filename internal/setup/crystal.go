// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SysWOW64 es la carpeta de 32 bits de un Windows de 64 bits. La app es VB6 de 32
// bits, así que todo su runtime vive acá, no en System32.
//
// Es exportada porque los que instalan (Setup App del CLI y del TUI) tienen que
// apuntar al mismo lugar: que cada uno arme su ruta es cómo se termina copiando a
// System32 por error, donde un control de 32 bits no sirve.
const SysWOW64 = `C:\Windows\SysWOW64`

// CrystalCOM son los componentes que hay que REGISTRAR además de copiar. El resto
// del runtime solo tiene que existir en SysWOW64: son dependencias, no componentes
// COM. Sale del procedimiento verificado en la PC de producción (LEEME.txt, paso 2).
var CrystalCOM = []string{"crviewer.dll", "Crystl32.OCX", "craxdrt.dll", "craxddrt.dll"}

// CrystalResult dice qué pasó al instalar el runtime. Existe para que el operador
// vea qué cambió: "ya estaba" y "se copió" se investigan distinto cuando un reporte
// no sale.
type CrystalResult struct {
	Copiados    []string
	YaEstaban   []string
	Registrados []string
	Fallos      []string
}

// NadaQueHacer dice si la instalación no tocó nada (todo estaba y todo registró).
func (r CrystalResult) NadaQueHacer() bool { return len(r.Copiados) == 0 && len(r.Fallos) == 0 }

// InstallCrystal copia el runtime completo a dir y registra los componentes COM.
//
// src viene del propio binario (assets), así que su raíz ya es la carpeta del
// runtime: si tuviera el prefijo legacy/crystal/SIDC_CRYSTAL/, esto copiaría
// carpetas a SysWOW64 en vez de archivos.
//
// Copiar es solo para lo que falta o lo que es distinto, comparando bytes: correr
// Setup App dos veces no puede reescribir 24 MB de DLL que la app puede tener
// cargadas. Comparar por tamaño no alcanza: dos versiones distintas de la misma DLL
// casi siempre pesan lo mismo, y el síntoma (un reporte que no sale) aparece mucho
// después, lejos de la instalación.
//
// Registrar, en cambio, se hace SIEMPRE: que el archivo esté no prueba que esté
// registrado —lo pudo copiar alguien a mano, o venir de un Windows restaurado— y el
// registro COM es idempotente (unos 200 ms). Es la única forma de no dejar un
// "presente pero sin registrar" que el checklist daría por bueno.
//
// out recibe solo lo que avanza (cada copia y cada registro). Lo que falla se
// devuelve en Fallos para que el llamador decida cómo nombrarlo: el CLI del TUI no
// tienen por qué inventar el mismo prefijo en dos lugares.
//
// Un destino sin permiso de escritura no aborta todo: se acumula en Fallos y sigue
// con el resto. Abortar en el primero dejaría al operador sin saber cuántos más
// faltan, y en la práctica la falta de permisos afecta a todos por igual.
func InstallCrystal(src fs.FS, dir string, registrar func(string) error, out func(string)) (CrystalResult, error) {
	var res CrystalResult
	if registrar == nil {
		registrar = func(string) error { return nil }
	}
	if out == nil {
		out = func(string) {}
	}

	archivos, err := listarRuntime(src)
	if err != nil {
		return res, err
	}
	// Guardia del embebido: si el binario se compiló sin el runtime adentro (el
	// linker de Go descarta los globales que nadie referencia: ya pasó en F2), esto
	// tiene que cortar acá y no reportar una instalación "completa" sin Crystal.
	if len(archivos) == 0 {
		return res, fmt.Errorf("el runtime de Crystal embebido está vacío: el binario se compiló sin los archivos")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return res, fmt.Errorf("no se pudo preparar %s: %w", dir, err)
	}

	// El nombre real del archivo en el embed manda: regsvr32 tiene que recibir esa
	// ruta, no la del listado.
	real := make(map[string]string, len(archivos))
	for _, name := range archivos {
		real[strings.ToLower(name)] = name
	}

	for _, name := range archivos {
		dst := filepath.Join(dir, name)
		if iguales(src, name, dst) {
			res.YaEstaban = append(res.YaEstaban, name)
			continue
		}
		if err := extraer(src, name, dst); err != nil {
			res.Fallos = append(res.Fallos, fmt.Sprintf("%s (no se pudo copiar a %s: %v)", name, dir, err))
			continue
		}
		res.Copiados = append(res.Copiados, name)
		out("CRYSTAL OK: " + name)
	}

	for _, com := range CrystalCOM {
		name, ok := real[strings.ToLower(com)]
		if !ok {
			res.Fallos = append(res.Fallos, com+" (no está en el runtime embebido)")
			continue
		}
		if err := registrar(filepath.Join(dir, name)); err != nil {
			res.Fallos = append(res.Fallos, fmt.Sprintf("%s (no se pudo registrar: %v)", name, err))
			continue
		}
		res.Registrados = append(res.Registrados, name)
		out("CRYSTAL COM OK: " + name)
	}
	return res, nil
}

// listarRuntime devuelve los archivos del runtime en orden, sin carpetas.
func listarRuntime(src fs.FS) ([]string, error) {
	var nombres []string
	err := fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		nombres = append(nombres, p)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer el runtime embebido: %w", err)
	}
	sort.Strings(nombres)
	return nombres, nil
}

// iguales compara el contenido real. Si no se puede leer el destino, se trata como
// distinto y se intenta copiar: el error de la copia es más claro que el del stat.
func iguales(src fs.FS, name, dst string) bool {
	quiero, err := fs.ReadFile(src, name)
	if err != nil {
		return false
	}
	tengo, err := os.ReadFile(dst)
	if err != nil {
		return false
	}
	return bytes.Equal(quiero, tengo)
}

// extraer escribe un archivo del embed en disco sin pasar por memoria de más: el
// runtime completo son ~24 MB y no hay motivo para tenerlos todos juntos en RAM.
func extraer(src fs.FS, name, dst string) error {
	in, err := src.Open(name)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
