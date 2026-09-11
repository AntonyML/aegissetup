// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"aegis-setup/internal/config"
)

// UninstallStep es una acción de desinstalación. Es dato puro a propósito: el plan se
// puede imprimir, contar y revisar sin ejecutar nada. Una acción con su función adentro
// no se puede auditar antes de correrla, y esto borra cosas.
type UninstallStep struct {
	Kind  StepKind
	Title string
	// Path es la carpeta o la clave de registro. Queda vacío en lo que no tiene ruta.
	Path string
	// Files y Bytes describen lo que hay adentro hoy, para que "--yes" no sea un
	// borrado a ciegas. El respaldo .bak es el caso que importa: son 39 MB que el
	// operador trajo de afuera.
	Files int
	Bytes int64
	// Value es el nombre del valor dentro de una clave compartida. Solo lo usa la
	// entrada del DSN en "ODBC Data Sources": esa clave tiene los DSN de toda la
	// máquina y borrarla entera se llevaría los ajenos.
	Value string
	// BakFiles y BakBytes cuentan los .bak dentro de la carpeta. Van aparte porque un
	// uninstall que se lleva el respaldo sin decirlo deja al operador sin poder
	// reinstalar y sin saber por qué.
	BakFiles int
	BakBytes int64
}

// StepKind distingue cómo se ejecuta cada paso. La ejecución vive en Desinstalar; el
// plan no lleva código.
type StepKind string

const (
	StepConfig  StepKind = "config"  // carpeta de config del operador
	StepData    StepKind = "data"    // árbol de datos de máquina
	StepDSN     StepKind = "dsn"     // clave del DSN 32-bit
	StepDSNList StepKind = "dsnlist" // entrada del DSN en la lista de ODBC
)

// PlanUninstall arma la lista de lo que Aegis creó en esta PC.
//
// Lo que NO entra es tan importante como lo que entra: la base SIDC, el login app y la
// carpeta de SIDC (app_dir) no los puso Aegis, así que no los saca. Un desinstalador que
// se lleva datos del cliente porque "venían en la misma carpeta" es peor que no tener
// desinstalador.
func PlanUninstall(cfg config.Config, exeDir string) []UninstallStep {
	pasos := []UninstallStep{
		{
			Kind:  StepConfig,
			Title: "Config del operador (%APPDATA%)",
			Path:  config.DirConfig(),
		},
		{
			Kind:  StepData,
			Title: "Datos de máquina (ProgramData)",
			Path:  config.DirProgramData(),
		},
	}
	// El config junto al binario es el layout viejo (previo a F3). Sigue existiendo en
	// PCs instaladas con una versión anterior, así que la desinstalación tiene que
	// sacarlo o queda un config.json huérfano que el próximo Aegis va a leer.
	pasos = append(pasos, UninstallStep{
		Kind:  StepConfig,
		Title: "Config heredado junto al ejecutable",
		Path:  filepath.Join(exeDir, "config.json"),
	})

	if soportaDSN() {
		pasos = append(pasos,
			UninstallStep{
				Kind:  StepDSN,
				Title: "DSN 32-bit",
				Path:  claveDSN(cfg.DsnName),
			},
			UninstallStep{
				Kind:  StepDSNList,
				Title: "Entrada del DSN en la lista de ODBC",
				Path:  claveDSNLista(),
				Value: cfg.DsnName,
			},
		)
	}

	for i := range pasos {
		pasos[i] = medirPaso(pasos[i])
	}
	return pasos
}

// SinDSN saca del plan los pasos de registro. Lo usa "aegis uninstall --keep-dsn".
//
// Hace falta porque el DSN no es de Aegis: es de la app. Aegis es la herramienta de
// instalación, así que el caso normal es sacar Aegis y dejar SIDC andando. Un uninstall
// que se lleva el DSN sin preguntar deja la app rota con error 3146 y el operador
// convencido de que la desinstalación "rompió SIDC".
func SinDSN(pasos []UninstallStep) []UninstallStep {
	var out []UninstallStep
	for _, p := range pasos {
		if p.Kind != StepDSN && p.Kind != StepDSNList {
			out = append(out, p)
		}
	}
	return out
}

// medirPaso completa los contadores de un paso de disco. Un paso sobre el registro no
// tiene nada que medir.
func medirPaso(p UninstallStep) UninstallStep {
	if p.Kind != StepConfig && p.Kind != StepData {
		return p
	}
	fi, err := os.Stat(p.Path)
	if err != nil {
		return p
	}
	if !fi.IsDir() {
		p.Files = 1
		p.Bytes = fi.Size()
		return p
	}
	_ = filepath.WalkDir(p.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		p.Files++
		p.Bytes += info.Size()
		if strings.EqualFold(filepath.Ext(d.Name()), ".bak") {
			p.BakFiles++
			p.BakBytes += info.Size()
		}
		return nil
	})
	return p
}

// Desinstalar ejecuta el plan y devuelve los fallos por nombre. Idempotente: una carpeta
// que ya no está o una clave que ya no existe no son fallos. Un desinstalador que falla
// la segunda vez obliga a correrlo una sola vez y con miedo.
func Desinstalar(pasos []UninstallStep, out func(string)) []string {
	var fallos []string
	for _, p := range pasos {
		if err := borrarPaso(p); err != nil {
			fallos = append(fallos, fmt.Sprintf("%s (%s): %v", p.Title, p.Path, err))
			out("FALLO  " + p.Title + ": " + err.Error())
			continue
		}
		out("OK     " + p.Title + ": " + estadoDePaso(p))
	}
	return fallos
}

func borrarPaso(p UninstallStep) error {
	switch p.Kind {
	case StepConfig, StepData:
		// Un archivo suelto (el config heredado) se borra con Remove; una carpeta con
		// RemoveAll. RemoveAll sobre un archivo también funciona, así que el Stat decide
		// solo por claridad del mensaje.
		if fi, err := os.Stat(p.Path); err == nil && !fi.IsDir() {
			return os.Remove(p.Path)
		}
		return os.RemoveAll(p.Path)
	case StepDSN:
		return borrarDSN(p.Path)
	case StepDSNList:
		return borrarDSNLista(p.Value)
	}
	return fmt.Errorf("paso desconocido %q", p.Kind)
}

// estadoDePaso dice qué pasó, distinguiendo "lo borré" de "no estaba". Los dos son OK:
// el segundo es el caso normal de una desinstalación repetida.
func estadoDePaso(p UninstallStep) string {
	if p.Kind == StepDSN || p.Kind == StepDSNList {
		return "borrado"
	}
	switch {
	case p.Files == 0:
		return "no estaba"
	case p.BakFiles > 0:
		return fmt.Sprintf("borrado (%d archivos, %s — incluido %d .bak, %s)",
			p.Files, HumanBytes(p.Bytes), p.BakFiles, HumanBytes(p.BakBytes))
	default:
		return fmt.Sprintf("borrado (%d archivos, %s)", p.Files, HumanBytes(p.Bytes))
	}
}

// FaltaElRespaldo avisa si el plan se va a llevar un .bak. El texto de la advertencia
// viaja aparte del plan para que la CLI pueda imprimirla antes del "--yes".
func FaltaElRespaldo(pasos []UninstallStep) (int, int64) {
	var n int
	var bytes int64
	for _, p := range pasos {
		n += p.BakFiles
		bytes += p.BakBytes
	}
	return n, bytes
}

// HumanBytes formatea tamaños para que el operador lea "38.9 MB" y no "40786944".
func HumanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
