//go:build windows

// © Antony Monge López — Costa Rica — Céd. 604700548

package setup

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func checkCrystalSys() []string {
	var missing []string
	for _, f := range CrystalFiles {
		if _, err := os.Stat(filepath.Join(SysWOW64, f)); err != nil {
			missing = append(missing, f)
		}
	}
	return missing
}

// checkOCXSys reporta qué OCX faltan en SysWOW64. Vive acá, junto a
// checkCrystalSys, porque las dos son la misma pregunta: qué hay ya puesto en
// el subsistema de 32 bits.
func checkOCXSys() []string {
	var missing []string
	for _, f := range RequiredOCX {
		if _, err := os.Stat(filepath.Join(SysWOW64, f)); err != nil {
			missing = append(missing, f)
		}
	}
	return missing
}

// RegisterCOM registra un componente COM de 32 bits.
//
// Se usa el regsvr32 de SysWOW64 y no el de System32 a propósito: el de System32 es
// de 64 bits y rechaza (o registra en el lugar equivocado) un control de 32, que es
// lo que son todos los de esta aplicación. /s lo deja sin carteles: la instalación no
// se detiene a esperar que alguien apriete Aceptar en cada uno de los 43 archivos.
func RegisterCOM(dll string) error {
	reg := filepath.Join(SysWOW64, "regsvr32.exe")
	out, err := exec.Command(reg, "/s", dll).CombinedOutput()
	if err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}

// PatchCrystalODBCBridge instala una copia especializada de p2sodbc.dll en appDir
// que intercepta las llamadas de conexión de Crystal Reports hacia SIDC_SQL
// e inyecta de forma transparente el usuario 'sidc' y la contraseña SQL.
// Esto permite que reportes antiguos con Trusted_Connection=Yes o usuarios obsoletos
// abran y visualicen datos correctamente en equipos fuera de dominio sin modificar los .rpt.
func PatchCrystalODBCBridge(appDir, pass string, out func(string)) error {
	if appDir == "" {
		return fmt.Errorf("appDir no especificado")
	}
	if out == nil {
		out = func(string) {}
	}
	pass = strings.TrimSpace(pass)
	if pass == "" {
		return nil
	}

	srcDll := filepath.Join(SysWOW64, "p2sodbc.dll")
	dllBytes, err := os.ReadFile(srcDll)
	if err != nil {
		return fmt.Errorf("leyendo %s: %w", srcDll, err)
	}

	dstDll := filepath.Join(appDir, "p2sodbc.dll")
	passStr := append([]byte(pass), 0)

	// Idempotencia: si ya existe y contiene la credencial y el usuario, no reescribir
	if curData, err := os.ReadFile(dstDll); err == nil {
		if bytes.Contains(curData, passStr) && bytes.Contains(curData, []byte("sidc\x00")) {
			out("Crystal ODBC bridge: ya se encuentra configurado en " + appDir)
			return nil
		}
	}

	data := make([]byte, len(dllBytes))
	copy(data, dllBytes)

	imageBase := uint32(0x41A50000)

	// .data padding en 0x3E990
	dataOffset := 0x3E990
	userVA := imageBase + uint32(dataOffset)
	copy(data[dataOffset:], []byte("sidc\x00"))

	passOffset := dataOffset + 16
	passVA := imageBase + uint32(passOffset)
	copy(data[passOffset:], passStr)

	// .text padding en 0x386A0
	hookOffset := 0x386A0
	hookRVA := uint32(hookOffset)
	iatConnectA := uint32(0x41A8A1B8)

	var hook []byte
	hook = append(hook, 0x8B, 0x44, 0x24, 0x08)          // mov eax, [esp+8]
	hook = append(hook, 0x85, 0xC0)                      // test eax, eax
	hook = append(hook, 0x74, 0x19)                      // jz .orig (+25)
	hook = append(hook, 0x81, 0x38, 'S', 'I', 'D', 'C') // cmp dword ptr [eax], 'SIDC'
	hook = append(hook, 0x75, 0x11)                      // jne .orig (+17)
	// mov dword ptr [esp+0x10], userVA
	hook = append(hook, 0xC7, 0x44, 0x24, 0x10)
	uBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(uBuf, userVA)
	hook = append(hook, uBuf...)
	// mov dword ptr [esp+0x18], passVA
	hook = append(hook, 0xC7, 0x44, 0x24, 0x18)
	pBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(pBuf, passVA)
	hook = append(hook, pBuf...)
	// .orig: jmp dword ptr [0x41A8A1B8]
	hook = append(hook, 0xFF, 0x25)
	iatBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(iatBuf, iatConnectA)
	hook = append(hook, iatBuf...)

	copy(data[hookOffset:], hook)

	// Parchear thunk en 0x39040 (jmp relJmp)
	thunkOffset := 0x39040
	relJmp := int32(hookRVA) - int32(thunkOffset+5)
	thunkPatch := []byte{0xE9, 0, 0, 0, 0, 0x90}
	binary.LittleEndian.PutUint32(thunkPatch[1:5], uint32(relJmp))
	copy(data[thunkOffset:], thunkPatch)

	tmpDll := dstDll + ".tmp"
	if err := os.WriteFile(tmpDll, data, 0755); err != nil {
		return fmt.Errorf("escribiendo %s: %w", tmpDll, err)
	}
	defer os.Remove(tmpDll)

	if err := os.Rename(tmpDll, dstDll); err != nil {
		_ = os.Remove(dstDll)
		if err2 := os.Rename(tmpDll, dstDll); err2 != nil {
			return fmt.Errorf("reemplazando %s: %w", dstDll, err2)
		}
	}

	out("Crystal ODBC bridge OK: p2sodbc.dll configurado en " + appDir)
	return nil
}
