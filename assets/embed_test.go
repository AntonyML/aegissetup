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

// Crystal() es la puerta del instalador al runtime: quien la llama no tiene que
// saber cómo está armado el embed. Si la raíz quedara en legacy/crystal/...,
// InstallCrystal copiaría a SysWOW64 una carpeta con subcarpetas adentro.
func TestCrystalFSTieneLaRaizPlana(t *testing.T) {
	fsys := Crystal()
	for _, f := range componentesCOM {
		nombre := strings.TrimPrefix(f, "legacy/crystal/SIDC_CRYSTAL/")
		if _, err := fs.Stat(fsys, nombre); err != nil {
			t.Errorf("falta %s en la raíz del runtime: %v", nombre, err)
		}
	}
	if _, err := fs.Stat(fsys, "legacy"); err == nil {
		t.Error("la raíz todavía tiene legacy/: el instalador tendría que armar la ruta a mano")
	}
}

// El runtime que se instala tiene que ser plano y completo: InstallCrystal copia
// archivos a SysWOW64 y no crea subcarpetas, así que un directorio adentro del
// embed pasaría inadvertido hasta que el reporte fallara en la PC.
func TestCrystalFSEsPlanoYCompleto(t *testing.T) {
	fsys := Crystal()
	n := 0
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != "." {
				t.Errorf("el runtime tiene el subdirectorio %s y se instala plano", p)
			}
			return nil
		}
		n++
		f, err := fsys.Open(p)
		if err != nil {
			t.Errorf("no se puede abrir %s: %v", p, err)
			return nil
		}
		f.Close()
		return nil
	})
	if err != nil {
		t.Fatalf("no se pudo recorrer el runtime: %v", err)
	}
	if n < 40 {
		t.Errorf("el runtime tiene %d archivos, se esperan >= 40", n)
	}
}

// ocxRequeridos y ocxSoporte son el contrato del embed con el instalador: los 22
// archivos que InstallOCX necesita encontrar adentro, con el mismo nombre que usa
// Setup App. Si alguien renombra un archivo en assets/legacy/ocx, el EXE compila
// igual y el control falta recién en la PC.
var ocxRequeridos = []string{
	"MSCOMCT2.OCX", "MSCOMCTL.OCX", "MSFLXGRD.OCX", "MSMASK32.OCX",
	"TABCTL32.OCX", "RICHTX32.OCX", "MSDATGRD.OCX", "MSADODC.OCX",
	"MSSTDFMT.DLL", "MSBIND.DLL", "MSDBRPTR.DLL",
}

var ocxSoporte = []string{
	"RCHTXES.DLL", "MSMSKES.DLL", "TABCTES.DLL", "FLXGDES.DLL",
	"MSCC2ES.DLL", "MSCMCES.DLL", "STDFTES.DLL", "DATGDES.DLL",
	"VB5DB.DLL", "DBRPRES.DLL", "ADODCES.DLL",
}

// OCX() es la puerta del instalador a los controles: en una PC limpia no hay carpeta
// de la PC vieja, así que los 22 archivos tienen que salir del binario.
func TestOCXTraeTodosLosControlesConLaRaizPlana(t *testing.T) {
	fsys := OCX()
	for _, f := range append(append([]string{}, ocxRequeridos...), ocxSoporte...) {
		if _, err := fs.Stat(fsys, f); err != nil {
			t.Errorf("falta %s en la raíz de los OCX embebidos: %v", f, err)
		}
	}
	if _, err := fs.Stat(fsys, "legacy"); err == nil {
		t.Error("la raíz todavía tiene legacy/: Setup App copiaría una carpeta a SysWOW64")
	}
}
