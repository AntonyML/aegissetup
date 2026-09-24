// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ReportConnectionPatch describe una sustitución declarativa dentro de los
// registros de conexión que Crystal 8 guarda en un .rpt. Las dos cadenas tienen
// exactamente la misma longitud: no se desplaza ningún registro binario.
type ReportConnectionPatch struct {
	Files       int
	Occurrences int
}

var (
	trustedYesASCII = []byte("Trusted_Connection=Yes")
	trustedNoASCII  = []byte("Trusted_Connection=No\x00")
	trustedYesUTF16 = utf16le("Trusted_Connection=Yes")
	trustedNoUTF16  = utf16le("Trusted_Connection=No\x00")
)

// PatchReportConnectionBytes convierte autenticación integrada en autenticación
// del DSN. Se aplica a las dos representaciones que aparecen en plantillas
// Crystal 8 (ASCII y UTF-16LE) y conserva exactamente el tamaño del archivo.
func PatchReportConnectionBytes(data []byte) ([]byte, int, error) {
	if len(data) == 0 {
		return nil, 0, fmt.Errorf("plantilla .rpt vacía")
	}
	out := append([]byte(nil), data...)
	count := 0
	for _, p := range [][2][]byte{
		{trustedYesASCII, trustedNoASCII},
		{trustedYesUTF16, trustedNoUTF16},
	} {
		n := bytes.Count(out, p[0])
		if n > 0 {
			out = bytes.ReplaceAll(out, p[0], p[1])
			count += n
		}
	}
	if len(out) != len(data) {
		return nil, 0, fmt.Errorf("parche de conexión alteró el tamaño (%d != %d)", len(out), len(data))
	}
	return out, count, nil
}

// PatchReportsConnections parchea todos los .rpt de repDir. Cada reemplazo se
// escribe mediante temporal + rename y se verifica el tamaño antes de publicar;
// no deja una plantilla parcialmente escrita si una operación falla.
func PatchReportsConnections(repDir string, out func(string)) (ReportConnectionPatch, error) {
	if out == nil {
		out = func(string) {}
	}
	ents, err := os.ReadDir(repDir)
	if err != nil {
		return ReportConnectionPatch{}, fmt.Errorf("leyendo reportes %s: %w", repDir, err)
	}
	sort.Slice(ents, func(i, j int) bool { return strings.ToLower(ents[i].Name()) < strings.ToLower(ents[j].Name()) })
	var result ReportConnectionPatch
	for _, ent := range ents {
		if ent.IsDir() || !strings.EqualFold(filepath.Ext(ent.Name()), ".rpt") {
			continue
		}
		path := filepath.Join(repDir, ent.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return result, fmt.Errorf("leyendo %s: %w", ent.Name(), err)
		}
		patched, n, err := PatchReportConnectionBytes(data)
		if err != nil {
			return result, fmt.Errorf("parcheando %s: %w", ent.Name(), err)
		}
		if n == 0 {
			continue
		}
		tmp, err := os.CreateTemp(repDir, ".rpt-connection-*.tmp")
		if err != nil {
			return result, fmt.Errorf("temporal para %s: %w", ent.Name(), err)
		}
		tmpName := tmp.Name()
		if chmodErr := tmp.Chmod(0644); chmodErr != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
			return result, fmt.Errorf("ajustando temporal para %s: %w", ent.Name(), chmodErr)
		}
		if _, err := tmp.Write(patched); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
			return result, fmt.Errorf("escribiendo temporal para %s: %w", ent.Name(), err)
		}
		if err := tmp.Close(); err != nil {
			_ = os.Remove(tmpName)
			return result, fmt.Errorf("cerrando temporal para %s: %w", ent.Name(), err)
		}
		if fi, statErr := os.Stat(tmpName); statErr != nil || fi.Size() != int64(len(data)) {
			_ = os.Remove(tmpName)
			return result, fmt.Errorf("integridad falló para %s", ent.Name())
		}
		if err := os.Rename(tmpName, path); err != nil {
			_ = os.Remove(path)
			if err2 := os.Rename(tmpName, path); err2 != nil {
				_ = os.Remove(tmpName)
				return result, fmt.Errorf("publicando %s: %w", ent.Name(), err2)
			}
		}
		result.Files++
		result.Occurrences += n
		out(fmt.Sprintf("Reporte conexión OK: %s (%d sustituciones)", ent.Name(), n))
	}
	return result, nil
}

// CountUnpatchedReports cuenta plantillas que todavía fuerzan autenticación
// integrada. Sirve para check y no modifica ningún archivo.
func CountUnpatchedReports(repDir string) (files, occurrences int, err error) {
	ents, err := os.ReadDir(repDir)
	if err != nil {
		return 0, 0, err
	}
	for _, ent := range ents {
		if ent.IsDir() || !strings.EqualFold(filepath.Ext(ent.Name()), ".rpt") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(repDir, ent.Name()))
		if err != nil {
			return files, occurrences, err
		}
		n := bytes.Count(data, trustedYesASCII) + bytes.Count(data, trustedYesUTF16)
		if n > 0 {
			files++
			occurrences += n
		}
	}
	return files, occurrences, nil
}
