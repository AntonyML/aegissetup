// Package assets embebe el runtime legado que AegisSetup despliega en cada PC.
//
// El objetivo (F2) es que el ejecutable instale SIDC en una PC limpia sin
// depender de archivos sueltos: el runtime de Crystal Reports 8 (~43 archivos,
// ~24 MB) y los controles OCX/DLL de VB6 (~23 archivos, ~4 MB) viajan dentro
// del binario y se extraen a SysWOW64 en el momento de la instalación.
//
// Los respaldos de base de datos NO se embeben: cada instalación tiene el suyo.
// Viven en disco, en C:\ProgramData\AegisSetup\assets\backups\sqlserver2014.
package assets

import (
	"embed"
	"io/fs"
)

// Legacy es el sistema de archivos embebido con el runtime legado.
//
// Rutas dentro del FS:
//
//	legacy/crystal/SIDC_CRYSTAL/  → runtime de Crystal Reports 8 (57 reportes)
//	legacy/ocx/                   → controles OCX/DLL de VB6
//
//go:embed all:legacy/crystal/SIDC_CRYSTAL
//go:embed all:legacy/ocx
var Legacy embed.FS

// Resumen describe el contenido del runtime embebido.
type Resumen struct {
	Archivos int
	Bytes    int64
}

// Info resume el runtime embebido y se calcula al inicializar el paquete.
//
// No es decorativo: el linker de Go elimina los globales que nadie referencia,
// y el contenido de //go:embed es un global más. Si el paquete se importa en
// blanco desde cmd/aegis sin que nada toque el FS, el EXE se compila igual pero
// SIN el runtime adentro. Referenciarlo desde un init lo mantiene enlazado.
var Info = resumir()

func resumir() Resumen {
	var r Resumen
	err := fs.WalkDir(Legacy, ".", func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		i, err := d.Info()
		if err != nil {
			return err
		}
		r.Archivos++
		r.Bytes += i.Size()
		return nil
	})
	if err != nil {
		return Resumen{}
	}
	return r
}
