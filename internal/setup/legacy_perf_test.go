// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatchThirdReportCacheRejectsUnknownVersion(t *testing.T) {
	data := legacyPerfSyntheticImage()
	if err := validateThirdReportCacheImage(data); err != nil {
		t.Fatalf("la fixture sintética debe superar el preflight estructural: %v", err)
	}
	before := append([]byte(nil), data...)
	got, err := PatchThirdReportCache(data, 100)
	if err == nil || got != nil || !strings.Contains(err.Error(), "versión") {
		t.Fatalf("una imagen desconocida debe rechazarse: %v", err)
	}
	if !bytes.Equal(data, before) {
		t.Fatal("el rechazo modificó el input")
	}
}

func TestPatchThirdReportCacheRejectsDamage(t *testing.T) {
	cases := []struct {
		name string
		edit func([]byte) []byte
	}{
		{"truncado", func(b []byte) []byte { return b[:0xc823e2] }},
		{"arquitectura", func(b []byte) []byte { b[0xbc] = 0x64; return b }},
		{"PE64", func(b []byte) []byte { b[0xd0] = 0x0b; b[0xd1] = 2; return b }},
		{"sección", func(b []byte) []byte { b[0x1b8] ^= 1; return b }},
		{"hook", func(b []byte) []byte { b[0xc823e2] ^= 1; return b }},
		{"padding", func(b []byte) []byte { b[0xea9e60] = 0x90; return b }},
		{"tipoCursor", func(b []byte) []byte { b[0xc823ff] = 0; return b }},
		{"setterReceptor", func(b []byte) []byte { b[0xc8238c] ^= 1; return b }},
		{"nativo", func(b []byte) []byte { b[0xe970] ^= 1; return b }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := tc.edit(legacyPerfSyntheticImage())
			before := append([]byte(nil), data...)
			out, err := PatchThirdReportCache(data, 100)
			if err == nil || out != nil {
				t.Fatal("se aceptó una imagen alterada")
			}
			if strings.Contains(err.Error(), "versión") {
				t.Fatalf("el preflight no detectó la corrupción; sólo falló el SHA: %v", err)
			}
			if !bytes.Equal(data, before) {
				t.Fatal("el rechazo modificó el input")
			}
		})
	}
}

func TestPatchThirdReportCacheRejectsUnsupportedSize(t *testing.T) {
	for _, n := range []int32{-1, 0, 1, 31, 33, 257, 2147483647} {
		if out, err := PatchThirdReportCache(nil, n); err == nil || out != nil || !strings.Contains(err.Error(), "CacheSize") {
			t.Fatalf("se aceptó CacheSize=%d", n)
		}
	}
}

// La imagen sintética tiene el mapa y las firmas esperados, pero nunca un SHA permitido.
func legacyPerfSyntheticImage() []byte {
	b := make([]byte, 0xeb2000)
	put16 := func(o int, v uint16) { binary.LittleEndian.PutUint16(b[o:o+2], v) }
	put32 := func(o int, v uint32) { binary.LittleEndian.PutUint32(b[o:o+4], v) }
	copy(b, "MZ")
	put32(0x3c, 0xb8)
	copy(b[0xb8:], "PE\x00\x00")
	put16(0xbc, 0x14c)
	put16(0xbe, 3)
	put16(0xcc, 0xe0)
	put16(0xce, 0x10f)
	put16(0xd0, 0x10b)
	put32(0xec, 0x400000)
	put32(0xf0, 0x1000)
	put32(0xf4, 0x1000)
	put32(0x12c, 16)
	for _, s := range []struct {
		o                            int
		name                         string
		vs, rva, rawSize, raw, flags uint32
	}{
		{0x1b0, ".text", 0xea8e60, 0x1000, 0xea9000, 0x1000, 0x60000020},
		{0x1d8, ".data", 0x4e584, 0xeaa000, 0x1000, 0xeaa000, 0xc0000040},
		{0x200, ".rsrc", 0x60e0, 0xef9000, 0x7000, 0xeab000, 0x40000040},
	} {
		copy(b[s.o:], s.name)
		put32(s.o+8, s.vs)
		put32(s.o+12, s.rva)
		put32(s.o+16, s.rawSize)
		put32(s.o+20, s.raw)
		put32(s.o+36, s.flags)
	}
	copy(b[0xe740:], "VB5!")
	put32(0xe770, 0x40e950)
	put32(0xe970, 0x12aa000)
	copy(b[0xc8238a:], []byte{0x8b, 0x45, 0xc4})
	copy(b[0xc823e2:], []byte{0x39, 0x35, 0x24, 0xa0, 0x2a, 0x01})
	copy(b[0xc823fa:], []byte{0x6a, 0xff, 0x6a, 3, 0x6a, 3})
	copy(b[0xc82446:], []byte{0xff, 0x97, 0xa0, 0, 0, 0})
	return b
}

func TestPatchThirdReportCacheKnownFixtures(t *testing.T) {
	paths := []string{
		filepath.Join("..", "..", "..", "app", "SIDC.baseline.bin"),
		filepath.Join("..", "..", "..", "referencia", "SIDC.original.bin"),
	}
	if explicit := os.Getenv("AEGIS_LEGACY_PERF_FIXTURE"); explicit != "" {
		paths = []string{explicit}
	}
	found := false
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) && os.Getenv("AEGIS_LEGACY_PERF_FIXTURE") == "" {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		found = true
		for _, n := range []int32{32, 100, 256} {
			before := append([]byte(nil), data...)
			out, err := PatchThirdReportCache(data, n)
			if err != nil {
				t.Fatalf("fixture %s: %v", filepath.Base(path), err)
			}
			if len(out) != len(data) || !bytes.Equal(before, data) {
				t.Fatal("cambió el tamaño o el input")
			}
			for i := range data {
				allowed := i >= 0xc823e2 && i < 0xc823e8 || i >= 0xea9e60 && i < 0xeaa000 || i >= 0x1b8 && i < 0x1bc
				if data[i] != out[i] && !allowed {
					t.Fatalf("cambio no permitido en 0x%x", i)
				}
			}
			if out[0xc823e2] != 0xe9 || out[0xc823e7] != 0x90 {
				t.Fatal("el hook no es JMP+NOP")
			}
			dst := int64(0xc823e7) + int64(int32(binary.LittleEndian.Uint32(out[0xc823e3:0xc823e7])))
			if dst != 0xea9e60 {
				t.Fatalf("el hook llega a 0x%x", dst)
			}
			stub, err := thirdReportCacheTrampoline(n)
			if err != nil {
				t.Fatal(err)
			}
			vs := binary.LittleEndian.Uint32(out[0x1b8:0x1bc])
			if vs != uint32(0xea9e60+len(stub)-0x1000) || vs > 0xea9000 {
				t.Fatalf("VirtualSize no cubre exactamente el stub: 0x%x", vs)
			}
			if !bytes.Equal(out[0xea9e60:0xea9e60+len(stub)], stub) || !bytes.Equal(out[0xea9e60+len(stub):0xeaa000], data[0xea9e60+len(stub):0xeaa000]) {
				t.Fatal("el cave difiere del stub o modificó padding adicional")
			}
			if _, err := PatchThirdReportCache(out, n); err == nil {
				t.Fatal("se aceptó parchear nuevamente la salida")
			}
		}
	}
	if !found {
		t.Skip("no hay fixture legacy local; fijá AEGIS_LEGACY_PERF_FIXTURE para probar el binario conocido")
	}
}

func TestThirdReportTrampolineCallingConvention(t *testing.T) {
	for _, n := range []int32{32, 100, 256} {
		for _, hr := range []uint32{0, 1, 0x80004005} {
			stub, err := thirdReportCacheTrampoline(n)
			if err != nil {
				t.Fatal(err)
			}
			checkLegacyPerfABI(t, stub, uint32(n), hr)
		}
	}
}

// Este intérprete mínimo valida efectos de pila/registros del stub; no ejecuta código nativo.
func checkLegacyPerfABI(t *testing.T, code []byte, size, hresult uint32) {
	t.Helper()
	r := [8]uint32{0x101, 0x202, 0x303, 0x404, 0x9000, 0xa000, 0, 0x707}
	before := r
	flags := uint32(0x246)
	mem := map[uint32]uint32{r[5] - 0x3c: 0x123400, 0x123400: 0x567800, 0x12aa024: 0x333}
	push := func(v uint32) { r[4] -= 4; mem[r[4]] = v }
	pop := func() uint32 { v := mem[r[4]]; r[4] += 4; return v }
	setter, checker := 0, 0
	pc := 0
	for steps := 0; steps < 100; steps++ {
		if pc >= len(code) {
			t.Fatal("el stub cayó fuera del código")
		}
		switch code[pc] {
		case 0x9c:
			push(flags)
			pc++
		case 0x60:
			oldSP := r[4]
			for _, i := range []int{0, 1, 2, 3, 4, 5, 6, 7} {
				v := r[i]
				if i == 4 {
					v = oldSP
				}
				push(v)
			}
			pc++
		case 0x8b:
			if bytes.HasPrefix(code[pc:], []byte{0x8b, 0x45, 0xc4}) {
				r[0] = mem[r[5]-0x3c]
				pc += 3
			} else if bytes.HasPrefix(code[pc:], []byte{0x8b, 0x08}) {
				r[1] = mem[r[0]]
				pc += 2
			} else {
				t.Fatalf("MOV inesperado en %d", pc)
			}
		case 0x68:
			push(binary.LittleEndian.Uint32(code[pc+1 : pc+5]))
			pc += 5
		case 0x50:
			push(r[0])
			pc++
		case 0xff:
			switch {
			case bytes.HasPrefix(code[pc:], []byte{0xff, 0x51, 0x44}):
				if r[1] != 0x567800 || pop() != 0x123400 || pop() != size {
					t.Fatal("argumentos del setter incorrectos")
				}
				setter++
				r[0] = hresult
				r[1] = 0xdead
				r[2] = 0xbeef
				flags = 0
				pc += 3
			case bytes.HasPrefix(code[pc:], []byte{0xff, 0x75, 0xc4}):
				push(mem[r[5]-0x3c])
				pc += 3
			case bytes.HasPrefix(code[pc:], []byte{0xff, 0x15, 0x5c, 0x10, 0x40, 0}):
				if pop() != hresult || pop() != 0x123400 || pop() != 0x527fcc || pop() != 0x44 {
					t.Fatal("argumentos HRESULT incorrectos")
				}
				checker++
				r[0] = 0
				r[1] = 0xbad
				r[2] = 0xf00
				flags = 0
				pc += 6
			default:
				t.Fatalf("CALL/PUSH inesperado en %d", pc)
			}
		case 0x85:
			if !bytes.HasPrefix(code[pc:], []byte{0x85, 0xc0}) {
				t.Fatal("TEST incorrecto")
			}
			flags = r[0] & 0x80000000
			pc += 2
		case 0x79:
			delta := int(int8(code[pc+1]))
			pc += 2
			if flags&0x80000000 == 0 {
				pc += delta
			}
		case 0x6a:
			push(uint32(int32(int8(code[pc+1]))))
			pc += 2
		case 0x61:
			for _, i := range []int{7, 6, 5, 4, 3, 2, 1, 0} {
				v := pop()
				if i != 4 {
					r[i] = v
				}
			}
			pc++
		case 0x9d:
			flags = pop()
			if flags != 0x246 {
				t.Fatal("no se restauró EFLAGS antes del CMP")
			}
			pc++
		case 0x39:
			if !bytes.HasPrefix(code[pc:], []byte{0x39, 0x35, 0x24, 0xa0, 0x2a, 1}) {
				t.Fatal("no se repitió el CMP original")
			}
			if r != before {
				t.Fatalf("registros/pila alterados: %x != %x", r, before)
			}
			pc += 6
		case 0xe9:
			dst := int64(0xea9e60+pc+5) + int64(int32(binary.LittleEndian.Uint32(code[pc+1:pc+5])))
			if dst != 0xc823e8 || setter != 1 || checker != int(hresult>>31) {
				t.Fatalf("retorno/llamadas incorrectos: %x, %d, %d", dst, setter, checker)
			}
			return
		default:
			t.Fatalf("opcode inesperado 0x%x en %d", code[pc], pc)
		}
	}
	t.Fatal("el stub no retornó")
}
