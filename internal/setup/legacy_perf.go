// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"bytes"
	"crypto/sha256"
	"debug/pe"
	"encoding/binary"
	"fmt"
)

const (
	thirdReportImageBytes = 0xeb2000
	thirdReportHook       = 0xc823e2
	thirdReportReturn     = 0xc823e8
	thirdReportCave       = 0xea9e60
	thirdReportCaveBytes  = 416
	thirdReportVSField    = 0x1b8
	thirdReportTextRaw    = 0x1000
	thirdReportTextVS     = 0xea8e60
)

// PatchThirdReportCache crea una variante experimental de los dos EXE SIDC
// identificados por SHA. Usala antes de cambiar conexión/logos. No escribe archivos,
// no modifica data y rechaza versiones desconocidas, salidas previas y otros perfiles.
// El resultado necesita pruebas de proveedor y equivalencia contable en QA.
func PatchThirdReportCache(data []byte, cacheSize int32) ([]byte, error) {
	if !thirdReportCacheSizeAllowed(cacheSize) {
		return nil, fmt.Errorf("CacheSize experimental no admitido: usá 32, 100 o 256")
	}
	if err := validateThirdReportCacheImage(data); err != nil {
		return nil, err
	}
	if !IsThirdReportCacheSupported(data) {
		return nil, fmt.Errorf("versión SIDC no admitida: el SHA256 no coincide con los baselines verificados")
	}
	stub, err := thirdReportCacheTrampoline(cacheSize)
	if err != nil {
		return nil, err
	}
	out := bytes.Clone(data)
	out[thirdReportHook] = 0xe9 // JMP rel32 y NOP para reemplazar la instrucción completa.
	binary.LittleEndian.PutUint32(out[thirdReportHook+1:thirdReportHook+5], uint32(int32(thirdReportCave-(thirdReportHook+5))))
	out[thirdReportHook+5] = 0x90
	copy(out[thirdReportCave:thirdReportCave+len(stub)], stub)
	binary.LittleEndian.PutUint32(out[thirdReportVSField:thirdReportVSField+4], uint32(thirdReportCave+len(stub)-thirdReportTextRaw))
	// La extensión queda dentro del raw de .text y de la misma página ya reservada.
	// No cambia longitud, SizeOfImage, secciones, conexiones, fórmulas o recursos.
	for i := range data {
		if data[i] != out[i] && !thirdReportCacheOffsetAllowed(i) {
			return nil, fmt.Errorf("el parche produjo una diferencia fuera del rango permitido en 0x%x", i)
		}
	}
	if !bytes.Equal(out[thirdReportCave:thirdReportCave+len(stub)], stub) {
		return nil, fmt.Errorf("falló la verificación del trampoline en memoria")
	}
	if _, err := pe.NewFile(bytes.NewReader(out)); err != nil {
		return nil, fmt.Errorf("el mapa PE resultante no es válido: %w", err)
	}
	return out, nil
}

// IsThirdReportCacheSupported reconoce únicamente los baselines completos
// verificados. Una imagen ya modificada o una versión diferente no es compatible.
func IsThirdReportCacheSupported(data []byte) bool {
	if len(data) != thirdReportImageBytes {
		return false
	}
	switch fmt.Sprintf("%x", sha256.Sum256(data)) {
	case "7515e4e96886f76031514d128d87b0f66baed4581e29d688ccac745f5b56891f",
		"1c1fc6b9c156050ea0c8e3005353b916ab687ab8defaebfd9592b5c19c7c159d":
		return true
	default:
		return false
	}
}

func thirdReportCacheSizeAllowed(n int32) bool {
	return n == 32 || n == 100 || n == 256
}

func thirdReportCacheOffsetAllowed(i int) bool {
	return i >= thirdReportHook && i < thirdReportReturn ||
		i >= thirdReportCave && i < thirdReportCave+thirdReportCaveBytes ||
		i >= thirdReportVSField && i < thirdReportVSField+4
}

func validateThirdReportCacheImage(data []byte) error {
	if len(data) != thirdReportImageBytes {
		return fmt.Errorf("tamaño SIDC no admitido: esperado %d bytes", thirdReportImageBytes)
	}
	if !bytes.Equal(data[:2], []byte("MZ")) || binary.LittleEndian.Uint32(data[0x3c:0x40]) != 0xb8 || !bytes.Equal(data[0xb8:0xbc], []byte{'P', 'E', 0, 0}) {
		return fmt.Errorf("cabecera PE SIDC no admitida")
	}
	f, err := pe.NewFile(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("mapa PE SIDC inválido: %w", err)
	}
	h, ok := f.OptionalHeader.(*pe.OptionalHeader32)
	if !ok || f.Machine != pe.IMAGE_FILE_MACHINE_I386 || h.Magic != 0x10b || h.ImageBase != 0x400000 || f.SizeOfOptionalHeader != 0xe0 || f.Characteristics != 0x10f {
		return fmt.Errorf("arquitectura PE SIDC no admitida: se requiere el baseline nativo PE32/I386")
	}
	if h.SectionAlignment != 0x1000 || h.FileAlignment != 0x1000 || h.DllCharacteristics != 0 || h.DataDirectory[4].VirtualAddress != 0 || h.DataDirectory[4].Size != 0 || h.DataDirectory[5].VirtualAddress != 0 || h.DataDirectory[5].Size != 0 {
		return fmt.Errorf("alineación, firma o relocaciones SIDC no admitidas")
	}
	want := []struct {
		name                         string
		vs, rva, size, offset, flags uint32
	}{
		{".text", thirdReportTextVS, 0x1000, 0xea9000, thirdReportTextRaw, 0x60000020},
		{".data", 0x4e584, 0xeaa000, 0x1000, 0xeaa000, 0xc0000040},
		{".rsrc", 0x60e0, 0xef9000, 0x7000, 0xeab000, 0x40000040},
	}
	if len(f.Sections) != len(want) {
		return fmt.Errorf("cantidad de secciones SIDC no admitida")
	}
	for i, w := range want {
		s := f.Sections[i]
		if s.Name != w.name || s.VirtualSize != w.vs || s.VirtualAddress != w.rva || s.Size != w.size || s.Offset != w.offset || s.Characteristics != w.flags {
			return fmt.Errorf("mapa de sección SIDC %s no admitido", w.name)
		}
	}
	if !bytes.Equal(data[0xe740:0xe744], []byte("VB5!")) || binary.LittleEndian.Uint32(data[0xe770:0xe774]) != 0x40e950 || binary.LittleEndian.Uint32(data[0xe970:0xe974]) != 0x12aa000 {
		return fmt.Errorf("metadatos nativos VB6 SIDC no admitidos")
	}
	for _, sig := range []struct {
		offset int
		data   []byte
	}{
		{0xc8238a, []byte{0x8b, 0x45, 0xc4}}, // Receptor III en [ebp-0x3c].
		{thirdReportHook, []byte{0x39, 0x35, 0x24, 0xa0, 0x2a, 1}},
		{0xc823fa, []byte{0x6a, 0xff, 0x6a, 3, 0x6a, 3}}, // Options, LockType y CursorType.
		{0xc82446, []byte{0xff, 0x97, 0xa0, 0, 0, 0}},    // Recordset.Open.
	} {
		if !bytes.Equal(data[sig.offset:sig.offset+len(sig.data)], sig.data) {
			return fmt.Errorf("instrucción SIDC no admitida en 0x%x", sig.offset)
		}
	}
	for _, b := range data[thirdReportCave : thirdReportCave+thirdReportCaveBytes] {
		if b != 0 {
			return fmt.Errorf("el padding SIDC para el trampoline no está vacío")
		}
	}
	return nil
}

// Las llamadas COM usan stdcall x86: this y Long son dos DWORD, y el callee
// limpia los argumentos. PUSHFD/PUSHAD protegen el contexto del código original.
// El HRESULT negativo usa el mismo helper VB6 y el mismo IID que Recordset.Open.
// Después de restaurar el contexto se repite CMP, por lo que el JNE original
// recibe exactamente sus flags. No se altera ninguna fórmula ni SQL.
func thirdReportCacheTrampoline(cacheSize int32) ([]byte, error) {
	if !thirdReportCacheSizeAllowed(cacheSize) {
		return nil, fmt.Errorf("CacheSize experimental no admitido")
	}
	stub := []byte{
		0x9c, 0x60, // pushfd; pushad.
		0x8b, 0x45, 0xc4, // mov eax,[ebp-0x3c].
		0x68, 0, 0, 0, 0, // push CacheSize.
		0x50,       // push eax (this).
		0x8b, 0x08, // mov ecx,[eax] (vtable).
		0xff, 0x51, 0x44, // call [ecx+0x44] (put_CacheSize).
		0x85, 0xc0, // test eax,eax.
		0x79, 0x11, // jns restaurar: saltar los 17 bytes de comprobación.
		0x6a, 0x44, // push offset del método.
		0x68, 0xcc, 0x7f, 0x52, 0, // push IID del recordset.
		0xff, 0x75, 0xc4, // push [ebp-0x3c] (this).
		0x50,                            // push HRESULT.
		0xff, 0x15, 0x5c, 0x10, 0x40, 0, // call [__vbaHresultCheckObj].
		0x61, 0x9d, // popad; popfd.
		0x39, 0x35, 0x24, 0xa0, 0x2a, 1, // CMP original desplazado.
		0xe9, 0, 0, 0, 0, // jmp rel32 al código original.
	}
	binary.LittleEndian.PutUint32(stub[6:10], uint32(cacheSize))
	jump := len(stub) - 5
	binary.LittleEndian.PutUint32(stub[jump+1:jump+5], uint32(int32(thirdReportReturn-(thirdReportCave+jump+5))))
	if len(stub) > thirdReportCaveBytes {
		return nil, fmt.Errorf("el trampoline excede el padding verificado")
	}
	return stub, nil
}
