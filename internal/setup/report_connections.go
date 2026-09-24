// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
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

type reportConnectionRule struct {
	from string
	to   string
}

var reportConnectionRules = []reportConnectionRule{
	{from: "Trusted_Connection=Yes", to: "Trusted_Connection=No"},
	{from: "Trusted_Connection=True", to: "Trusted_Connection=No"},
	{from: "Trusted_Connection=SSPI", to: "Trusted_Connection=No"},
	{from: "TrustedConnection=Yes", to: "TrustedConnection=No"},
	{from: "TrustedConnection=True", to: "TrustedConnection=No"},
	{from: "TrustedConnection=SSPI", to: "TrustedConnection=No"},
	{from: "Integrated Security=Yes", to: "Integrated Security=No"},
	{from: "Integrated Security=True", to: "Integrated Security=No"},
	{from: "Integrated Security=SSPI", to: "Integrated Security=No"},
}

type reportConnectionEncoding struct {
	width  int
	encode func(string) []byte
	decode func([]byte) string
}

func reportConnectionEncodings() []reportConnectionEncoding {
	return []reportConnectionEncoding{
		{width: 1, encode: func(s string) []byte { return []byte(s) }, decode: decodeASCII},
		{width: 2, encode: utf16le, decode: decodeUTF16LE},
		{width: 2, encode: utf16be, decode: decodeUTF16BE},
	}
}

func decodeASCII(data []byte) string {
	runes := make([]rune, len(data))
	for i, value := range data {
		runes[i] = rune(value)
	}
	return string(runes)
}

// PatchReportConnectionBytes convierte autenticación integrada en autenticación
// del DSN. Se aplica a las dos representaciones que aparecen en plantillas
// Crystal 8 (ASCII y UTF-16LE) y conserva exactamente el tamaño del archivo.
func PatchReportConnectionBytes(data []byte) ([]byte, int, error) {
	return patchReportConnectionBytes(data, "")
}

// PatchReportConnectionBytesForUser corrige autenticación integrada y reemplaza
// el UID del autor por el usuario SQL configurado. El valor nuevo se rellena con
// espacios hasta ocupar exactamente el campo que ya existe en el .rpt.
func PatchReportConnectionBytesForUser(data []byte, sqlUser string) ([]byte, int, error) {
	return patchReportConnectionBytes(data, canonicalReportUser(sqlUser))
}

func patchReportConnectionBytes(data []byte, sqlUser string) ([]byte, int, error) {
	if len(data) == 0 {
		return nil, 0, fmt.Errorf("plantilla .rpt vacía")
	}
	out := append([]byte(nil), data...)
	count := 0
	for _, enc := range reportConnectionEncodings() {
		text := enc.decode(out)
		n, err := patchEncodedFlags(out, text, enc)
		if err != nil {
			return nil, 0, err
		}
		count += n
		if sqlUser != "" {
			text = enc.decode(out)
			n, err = patchEncodedUser(out, text, enc, sqlUser)
			if err != nil {
				return nil, 0, err
			}
			count += n
		}
	}
	if len(out) != len(data) {
		return nil, 0, fmt.Errorf("parche de conexión alteró el tamaño (%d != %d)", len(out), len(data))
	}
	return out, count, nil
}

func canonicalReportUser(sqlUser string) string {
	user := strings.TrimSpace(sqlUser)
	if user == "" {
		return "sidc"
	}
	return user
}

func patchEncodedFlags(data []byte, text string, enc reportConnectionEncoding) (int, error) {
	lower := []rune(strings.ToLower(text))
	count := 0
	for _, rule := range reportConnectionRules {
		from := []rune(strings.ToLower(rule.from))
		fromLen := len(from)
		to := []rune(rule.to)
		if len(to) > fromLen {
			return 0, fmt.Errorf("regla de conexión excede el tamaño original")
		}
		replacement := make([]rune, fromLen)
		copy(replacement, to)
		encoded := enc.encode(string(replacement))
		search := 0
		for search < len(lower) {
			idx := indexRuneSequence(lower, from, search)
			if idx < 0 {
				break
			}
			byteOffset := idx * enc.width
			if byteOffset+len(encoded) > len(data) {
				return 0, fmt.Errorf("bloque de conexión fuera de los límites")
			}
			copy(data[byteOffset:byteOffset+len(encoded)], encoded)
			count++
			search = idx + fromLen
		}
	}
	return count, nil
}

func patchEncodedUser(data []byte, text string, enc reportConnectionEncoding, sqlUser string) (int, error) {
	if strings.ContainsAny(sqlUser, ";\x00\r\n") {
		return 0, fmt.Errorf("usuario SQL inválido para una plantilla Crystal")
	}
	runes := []rune(text)
	lower := []rune(strings.ToLower(text))
	key := []rune("uid=")
	count := 0
	search := 0
	for search < len(lower) {
		keyStart := indexRuneSequence(lower, key, search)
		if keyStart < 0 {
			break
		}
		valueStart := keyStart + len(key)
		valueEnd := valueStart
		for valueEnd < len(runes) && runes[valueEnd] != ';' && runes[valueEnd] != 0 && runes[valueEnd] != '\r' && runes[valueEnd] != '\n' {
			valueEnd++
		}
		oldValue := string(runes[valueStart:valueEnd])
		trimmed := strings.TrimSpace(oldValue)
		if trimmed != "" && trimmed != sqlUser {
			oldRunes := runes[valueStart:valueEnd]
			newRunes := []rune(sqlUser)
			if len(newRunes) > len(oldRunes) {
				return 0, fmt.Errorf("usuario SQL no cabe en el bloque Crystal (campo de %d caracteres)", len(oldRunes))
			}
			replacement := make([]rune, len(oldRunes))
			for i := range replacement {
				replacement[i] = ' '
			}
			copy(replacement, newRunes)
			encoded := enc.encode(string(replacement))
			byteOffset := valueStart * enc.width
			if byteOffset+len(encoded) > len(data) {
				return 0, fmt.Errorf("UID fuera de los límites del bloque Crystal")
			}
			copy(data[byteOffset:byteOffset+len(encoded)], encoded)
			count++
		}
		search = valueEnd + 1
	}
	return count, nil
}

func indexRuneSequence(haystack, needle []rune, start int) int {
	if len(needle) == 0 {
		return start
	}
	for i := start; i+len(needle) <= len(haystack); i++ {
		matches := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				matches = false
				break
			}
		}
		if matches {
			return i
		}
	}
	return -1
}

// PatchReportsConnections parchea todos los .rpt de repDir. Cada reemplazo se
// escribe mediante temporal + rename y se verifica el tamaño antes de publicar;
// no deja una plantilla parcialmente escrita si una operación falla.
func PatchReportsConnections(repDir string, out func(string)) (ReportConnectionPatch, error) {
	return patchReportsConnections(repDir, "", out)
}

// PatchReportsConnectionsForUser aplica el parche a todas las plantillas y
// normaliza el UID de cada bloque al usuario SQL de la configuración.
func PatchReportsConnectionsForUser(repDir, sqlUser string, out func(string)) (ReportConnectionPatch, error) {
	return patchReportsConnections(repDir, canonicalReportUser(sqlUser), out)
}

func patchReportsConnections(repDir, sqlUser string, out func(string)) (ReportConnectionPatch, error) {
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
		var patched []byte
		var n int
		if sqlUser == "" {
			patched, n, err = PatchReportConnectionBytes(data)
		} else {
			patched, n, err = PatchReportConnectionBytesForUser(data, sqlUser)
		}
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
		if err := tmp.Sync(); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
			return result, fmt.Errorf("sincronizando temporal para %s: %w", ent.Name(), err)
		}
		if err := tmp.Close(); err != nil {
			_ = os.Remove(tmpName)
			return result, fmt.Errorf("cerrando temporal para %s: %w", ent.Name(), err)
		}
		if fi, statErr := os.Stat(tmpName); statErr != nil || fi.Size() != int64(len(data)) {
			_ = os.Remove(tmpName)
			return result, fmt.Errorf("integridad falló para %s", ent.Name())
		}
		if err := replaceFileAtomic(tmpName, path); err != nil {
			_ = os.Remove(tmpName)
			return result, fmt.Errorf("publicando %s: %w", ent.Name(), err)
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
		_, n, err := PatchReportConnectionBytes(data)
		if err != nil {
			return files, occurrences, err
		}
		if n > 0 {
			files++
			occurrences += n
		}
	}
	return files, occurrences, nil
}

func decodeUTF16LE(data []byte) string {
	return decodeUTF16(data, false)
}

func decodeUTF16BE(data []byte) string {
	return decodeUTF16(data, true)
}

func decodeUTF16(data []byte, bigEndian bool) string {
	runes := make([]rune, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		var value uint16
		if bigEndian {
			value = uint16(data[i])<<8 | uint16(data[i+1])
		} else {
			value = uint16(data[i]) | uint16(data[i+1])<<8
		}
		runes = append(runes, rune(value))
	}
	return string(runes)
}

func utf16be(s string) []byte {
	b := make([]byte, 0, len(s)*2)
	for _, r := range s {
		b = append(b, byte(r>>8), byte(r))
	}
	return b
}
