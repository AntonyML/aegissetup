// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RequiredOCX se REGISTRAN con regsvr32 (son los que dan error 339).
// Salieron del ST6UNST.LOG del instalador original de prod.
var RequiredOCX = []string{
	"MSCOMCT2.OCX", "MSCOMCTL.OCX", "MSFLXGRD.OCX", "MSMASK32.OCX",
	"TABCTL32.OCX", "RICHTX32.OCX", "MSDATGRD.OCX", "MSADODC.OCX",
	"MSSTDFMT.DLL", "MSBIND.DLL", "MSDBRPTR.DLL",
}

// SupportFiles solo se COPIAN a SysWOW64, sin registrar:
// satélites en español (*ES.DLL, DATGDES, STDFTES) + data binding
// (VB5DB, DBRPRES, ADODCES). Sin ellos la app abre, pero controles
// salen en inglés o fallan pantallas con data-binding.
var SupportFiles = []string{
	"RCHTXES.DLL", "MSMSKES.DLL", "TABCTES.DLL", "FLXGDES.DLL",
	"MSCC2ES.DLL", "MSCMCES.DLL", "STDFTES.DLL", "DATGDES.DLL",
	"VB5DB.DLL", "DBRPRES.DLL", "ADODCES.DLL",
}

// CrystalFiles es el mínimo del runtime de Crystal Reports 8 para
// diagnosticar: motor + RDC + visor + puentes ODBC (los reportes de
// SIDC conectan por ODBC: sin p2sodbc/u2fodbc dan "database DLL").
// El runtime completo (~40 DLLs) NO se copia a mano: se instala con
// su Setup original. Ver assets/legacy/crystal/LEEME.txt.
var CrystalFiles = []string{
	"crpe32.dll", "craxDrt.dll", "craxddrt.dll", "crviewer.dll", "Crystl32.OCX",
	"p2sodbc.dll", "u2fodbc.dll",
}

// CheckCrystal reporta qué archivos del runtime faltan en SysWOW64.
// Vacío = runtime presente. Solo Windows; fuera de Windows se omite.
func CheckCrystal() []string {
	return checkCrystalSys()
}

// CheckOCX reporta qué RequiredOCX faltan en SysWOW64. Ojo: es presencia del
// archivo, no registro COM. Consultar el registro exigiría los GUID de cada
// control, que no los tenemos; la presencia en SysWOW64 es el proxy que ya usa
// el resto del diagnóstico.
func CheckOCX() []string {
	return checkOCXSys()
}

// FaltanOCXEn reporta qué RequiredOCX no están en dir, el origen desde donde
// Setup App los copia a SysWOW64. Sirve para distinguir "falta registrarlos"
// (se arregla solo) de "no hay de dónde copiarlos" (hay que traerlos).
func FaltanOCXEn(dir string) []string {
	var faltan []string
	for _, f := range RequiredOCX {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			faltan = append(faltan, f)
		}
	}
	return faltan
}

// CheckAppFiles verifica exe, reportes, fotos. No modifica nada.
func CheckAppFiles(appDir string) []string {
	var missing []string
	exe := filepath.Join(appDir, "Sistema Intergrado de Controles y Presupuesto.exe")
	if _, err := os.Stat(exe); err != nil {
		missing = append(missing, "exe: "+exe)
	}
	rep := filepath.Join(appDir, "Reportes")
	ents, err := os.ReadDir(rep)
	if err != nil {
		missing = append(missing, "Reportes/: "+rep)
	} else {
		n := 0
		for _, e := range ents {
			if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".rpt") {
				n++
			}
		}
		if n < 50 {
			missing = append(missing, fmt.Sprintf("Reportes/: solo %d .rpt (esperaba ~57)", n))
		}
	}
	if _, err := os.Stat(filepath.Join(appDir, "Fotos", "Principal.jpg")); err != nil {
		missing = append(missing, "Fotos/Principal.jpg")
	}
	return missing
}

// InstallOCX registra los OCX de legacyDir en SysWOW64 con regsvr32 de 32-bit.
// Solo Windows. Devuelve lista de los que fallaron.
func InstallOCX(legacyDir string, out func(string)) []string {
	var failed []string
	syswow := `C:\Windows\SysWOW64`
	reg := filepath.Join(syswow, "regsvr32.exe")
	for _, f := range RequiredOCX {
		src := filepath.Join(legacyDir, f)
		if _, err := os.Stat(src); err != nil {
			failed = append(failed, f+" (falta en "+legacyDir+", cópialo de la PC vieja)")
			continue
		}
		dst := filepath.Join(syswow, f)
		if _, err := os.Stat(dst); err != nil {
			if cpErr := copyFile(src, dst); cpErr != nil {
				failed = append(failed, f+" (no se pudo copiar a SysWOW64, corre como Admin: "+cpErr.Error()+")")
				continue
			}
		}
		cmd := exec.Command(reg, "/s", dst)
		if err := cmd.Run(); err != nil {
			failed = append(failed, f+" (regsvr32 falló: "+err.Error()+")")
			continue
		}
		out("OCX OK: " + f)
	}
	for _, f := range SupportFiles {
		src := filepath.Join(legacyDir, f)
		if _, err := os.Stat(src); err != nil {
			out("SOPORTE FALTA (opcional): " + f)
			continue
		}
		dst := filepath.Join(syswow, f)
		if _, err := os.Stat(dst); err != nil {
			if cpErr := copyFile(src, dst); cpErr != nil {
				failed = append(failed, f+" (no se pudo copiar a SysWOW64, corre como Admin: "+cpErr.Error()+")")
				continue
			}
		}
		out("SOPORTE OK: " + f)
	}
	return failed
}

// DockerPatchBudget es el maximo de chars que puede medir la clave del parche
// _DOCKER con un usuario dado. Sale de la aritmetica del slot: el parche
// reemplaza "Initial Catalog=SIDC" (20 chars) por "UID=<user>;PWD=<pass>".
// Es la fuente unica de la regla: la usan el parche y la validacion previa del
// TUI, para que el prompt no acepte algo que el parche va a rechazar.
func DockerPatchBudget(user string) int {
	return 20 - len("UID=") - len(";PWD=") - len(user)
}

// PatchDockerExe crea/copia _DOCKER.exe con UID/PWD embebidos sin alargar
// el binario: reemplaza "Initial Catalog=SIDC" (20 chars) por
// "UID=<user>;PWD=<pass>" que debe medir <=20 chars + null.
// Es solo para Docker/dev con SQL Auth. Prod con Windows Auth no lo necesita.
func PatchDockerExe(appDir, user, pass string, out func(string)) error {
	if len(pass) > DockerPatchBudget(user) {
		return fmt.Errorf("UID/PWD muy largos para el parche (max 20 chars en total 'UID=u;PWD=p'): recibí %d",
			len("UID="+user+";PWD="+pass))
	}
	src := filepath.Join(appDir, "Sistema Intergrado de Controles y Presupuesto.exe")
	dst := filepath.Join(appDir, "Sistema Intergrado de Controles y Presupuesto_DOCKER.exe")
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	old := utf16le("Initial Catalog=SIDC")
	nu := append(utf16le("UID="+user+";PWD="+pass), 0, 0)
	for len(nu) < len(old) {
		nu = append(nu, 0, 0)
	}
	count := 0
	for i := 0; i+len(old) <= len(data); i++ {
		match := true
		for j := range old {
			if data[i+j] != old[j] {
				match = false
				break
			}
		}
		if match {
			copy(data[i:i+len(old)], nu)
			count++
			i += len(old) - 1
		}
	}
	if count == 0 {
		return fmt.Errorf("no se encontró el string en el exe (¿ya está parchado o es otra versión?)")
	}
	if err := os.WriteFile(dst, data, 0644); err != nil {
		return err
	}
	out(fmt.Sprintf("_DOCKER.exe OK (%d parche) -> %s", count, dst))
	return nil
}

func utf16le(s string) []byte {
	b := make([]byte, 0, len(s)*2)
	for _, r := range s {
		b = append(b, byte(r), byte(r>>8))
	}
	return b
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
