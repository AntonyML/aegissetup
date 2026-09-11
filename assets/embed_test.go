package assets

import (
	"io/fs"
	"strings"
	"testing"
)

// componentesCOM son los únicos archivos del runtime que hay que registrar con
// regsvr32 en la PC destino. El resto de los DLL basta con que existan en
// SysWOW64: son dependencias, no componentes COM.
var componentesCOM = []string{
	"legacy/crystal/SIDC_CRYSTAL/crpe32.dll",
	"legacy/crystal/SIDC_CRYSTAL/craxdrt.dll",
	"legacy/crystal/SIDC_CRYSTAL/craxddrt.dll",
	"legacy/crystal/SIDC_CRYSTAL/crviewer.dll",
	"legacy/crystal/SIDC_CRYSTAL/Crystl32.OCX",
}

func contarArchivos(t *testing.T, raiz string) int {
	t.Helper()
	n := 0
	err := fs.WalkDir(Legacy, raiz, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			n++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("no se pudo recorrer %s dentro del embed: %v", raiz, err)
	}
	return n
}

// TestComponentesCOMEmbebidos cubre el caso en que el EXE compila bien pero a
// SIDC no le generan los reportes: pasa cuando falta algún componente COM del
// runtime dentro del binario.
func TestComponentesCOMEmbebidos(t *testing.T) {
	for _, f := range componentesCOM {
		if _, err := Legacy.ReadFile(f); err != nil {
			t.Errorf("falta en el binario: %s (%v)", f, err)
		}
	}
}

// TestRuntimeCompletoEmbebido es el guardia del pipeline: si los .dll del
// runtime no están versionados, el embed se arma incompleto y el Release
// publicaría un EXE inútil en una PC limpia. Este test corta antes del build.
func TestRuntimeCompletoEmbebido(t *testing.T) {
	const (
		minCrystal = 40 // 43 en disco; con el embed incompleto el conteo cae a 1
		minOCX     = 20 // 23 en disco
	)
	if n := contarArchivos(t, "legacy/crystal/SIDC_CRYSTAL"); n < minCrystal {
		t.Errorf("runtime de Crystal incompleto: %d archivos embebidos, se esperan >= %d (¿faltan los .dll versionados?)", n, minCrystal)
	}
	if n := contarArchivos(t, "legacy/ocx"); n < minOCX {
		t.Errorf("controles OCX incompletos: %d archivos embebidos, se esperan >= %d", n, minOCX)
	}
}

// TestRespaldosNoEmbebidos confirma la decisión de diseño: los .bak son
// distintos en cada instalación y nunca deben viajar dentro del binario.
func TestRespaldosNoEmbebidos(t *testing.T) {
	err := fs.WalkDir(Legacy, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.Contains(strings.ToLower(p), ".bak") {
			t.Errorf("un respaldo quedó embebido en el binario: %s", p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("no se pudo recorrer el embed: %v", err)
	}
}
