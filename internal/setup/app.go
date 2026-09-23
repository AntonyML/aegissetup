// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"os"
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
//
// Es una lista de diagnóstico, no de instalación: el runtime completo (~43
// archivos) lo instala InstallCrystal desde el propio binario. Los dos controles
// que hay que REGISTRAR son CrystalCOM; crpe32 y los puentes ODBC solo se copian.
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

// OrigenOCX dice de dónde salen los controles que Setup App instala.
//
// Existe un origen y no dos parámetros sueltos porque la precedencia es una regla, no
// un detalle: los dos orígenes juntos deciden si un control se puede instalar, y esa
// pregunta la hace el checklist antes de intentar nada.
type OrigenOCX struct {
	// Binario es el kit embebido en el ejecutable. Es el origen normal: en una PC
	// limpia no existe la carpeta de la PC vieja, así que un instalador que solo
	// lea de disco obliga a copiar 22 archivos a mano, que es exactamente el
	// requisito externo que F7 elimina.
	Binario fs.FS
	// Carpeta es un origen extra en disco, para el control que no venga en el
	// binario (kit incompleto, o un control más nuevo en la PC vieja).
	//
	// Lo que ya viaja adentro gana: un archivo suelto y viejo en disco no puede
	// degradar una instalación que ya es autocontenida.
	Carpeta string
}

// archivoDe busca un archivo en los orígenes y devuelve el FS y el nombre real con el
// que hay que abrirlo. En el binario la búsqueda es sin distinguir mayúsculas: el embed
// de Go es sensible (los nombres salen de la PC de producción y no son prolijos) y en
// Windows el archivo destino no lo es, así que respetar el caso del origen evitaría que
// algo copiado antes a mano se comparara bien.
func (o OrigenOCX) archivoDe(indice map[string]string, nombre string) (fs.FS, string, bool) {
	if real, ok := indice[strings.ToLower(nombre)]; ok {
		return o.Binario, real, true
	}
	if o.Carpeta == "" {
		return nil, "", false
	}
	disco := os.DirFS(o.Carpeta)
	if _, err := fs.Stat(disco, nombre); err != nil {
		return nil, "", false
	}
	return disco, nombre, true
}

// indiceOCX mapea nombre en minúsculas -> nombre real dentro del kit embebido. Un
// binario sin el kit devuelve un índice vacío y no un error: el diagnóstico posterior
// ("falta en SysWOW64 y no hay de dónde copiarlo") es más útil que una falla de arranque.
func indiceOCX(src fs.FS) map[string]string {
	indice := map[string]string{}
	if src == nil {
		return indice
	}
	err := fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		indice[strings.ToLower(p)] = p
		return nil
	})
	if err != nil {
		return map[string]string{}
	}
	return indice
}

// FaltanOCXEn reporta qué RequiredOCX no están en ningún origen desde donde Setup App
// los copia a SysWOW64. Sirve para distinguir "falta registrarlos" (se arregla solo) de
// "no hay de dónde copiarlos" (hay que conseguirlos).
func FaltanOCXEn(o OrigenOCX) []string {
	indice := indiceOCX(o.Binario)
	var faltan []string
	for _, f := range RequiredOCX {
		if _, _, ok := o.archivoDe(indice, f); !ok {
			faltan = append(faltan, f)
		}
	}
	return faltan
}

// CheckAppFiles verifica exe, reportes, fotos. No modifica nada.
func CheckAppFiles(appDir string) []string {
	// Sin app_dir no hay nada que medir, y medir "" daría rutas relativas al
	// directorio de trabajo del proceso: diría FALTA con una ruta que no existe en
	// ningún lado. El mensaje tiene que decir la causa, que es otra.
	if appDir == "" {
		return []string{"app_dir sin configurar (elegí perfil o pasá --app-dir)"}
	}
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

// InstallOCX copia los controles a dir y registra los requeridos, con el regsvr32 de
// 32 bits. Solo Windows. Devuelve la lista de los que fallaron.
//
// La presencia del archivo no se mira con un Stat: se compara el contenido. Un OCX
// viejo en SysWOW64 con el nombre correcto es el caso peor de todos, porque el checklist
// lo da por bueno y el error 339 aparece al abrir una pantalla puntual, lejos de la
// instalación (misma razón que en InstallCrystal).
//
// Registrar, en cambio, se hace SIEMPRE. Que el archivo esté no prueba que esté
// registrado, y regsvr32 es idempotente (~200 ms por control).
//
// Un archivo que no se puede escribir no aborta el resto: se acumula y sigue, así el
// operador ve de una sola vez todo lo que falta.
func InstallOCX(o OrigenOCX, dir string, registrar func(string) error, out func(string)) []string {
	if registrar == nil {
		registrar = func(string) error { return nil }
	}
	if out == nil {
		out = func(string) {}
	}
	indice := indiceOCX(o.Binario)

	var fallos []string
	for _, f := range RequiredOCX {
		src, nombre, ok := o.archivoDe(indice, f)
		if !ok {
			fallos = append(fallos, f+" (no está en el binario"+o.colaDeCarpeta()+": traelo de la PC vieja)")
			continue
		}
		dst := filepath.Join(dir, f)
		copiado := false
		if !iguales(src, nombre, dst) {
			if err := extraer(src, nombre, dst); err != nil {
				fallos = append(fallos, f+" (no se pudo copiar a "+dir+", corré como Admin: "+err.Error()+")")
				continue
			}
			copiado = true
		}
		// Mismo registrar que los componentes de Crystal: quién es el regsvr32
		// correcto (el de 32 bits) se decide en un solo lugar.
		if err := registrar(dst); err != nil {
			fallos = append(fallos, f+" (regsvr32 falló: "+err.Error()+")")
			continue
		}
		// Se informa siempre porque registrar siempre: "ya estaba" responde la
		// pregunta que se hace el operador al repetir una instalación ("¿hizo algo?")
		// y distingue el archivo que quedó de una corrida anterior del que se acaba
		// de copiar. Los de soporte no se registran y solo se nombran al copiar: en
		// una instalación repetida el silencio es la buena noticia.
		if copiado {
			out("OCX OK: " + f)
		} else {
			out("OCX YA ESTABA: " + f)
		}
	}
	for _, f := range SupportFiles {
		src, nombre, ok := o.archivoDe(indice, f)
		if !ok {
			out("SOPORTE FALTA (opcional): " + f)
			continue
		}
		dst := filepath.Join(dir, f)
		if !iguales(src, nombre, dst) {
			if err := extraer(src, nombre, dst); err != nil {
				fallos = append(fallos, f+" (no se pudo copiar a "+dir+", corré como Admin: "+err.Error()+")")
				continue
			}
			out("SOPORTE OK: " + f)
		}
	}
	return fallos
}

// colaDeCarpeta nombra la carpeta legacy solo si hay una configurada. Con legacy_dir
// vacío el mensaje no puede sugerir que el archivo se buscó en algún lado.
func (o OrigenOCX) colaDeCarpeta() string {
	if o.Carpeta == "" {
		return " y no hay carpeta legacy configurada"
	}
	return " ni en " + o.Carpeta
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
// PatchDockerExe crea/copia _DOCKER.exe con UID/PWD embebidos sin alargar
// el binario: reemplaza "Initial Catalog=SIDC" (20 chars) por
// "UID=<user>;PWD=<pass>" que debe medir <=20 chars + null.
// Es idempotente, genera respaldo y verifica integridad de tamaño.
func PatchDockerExe(appDir, user, pass string, out func(string)) error {
	user = strings.TrimSpace(user)
	pass = strings.TrimSpace(pass)
	if len(pass) > DockerPatchBudget(user) {
		return fmt.Errorf("UID/PWD muy largos para el parche (max 20 chars en total 'UID=u;PWD=p'): recibí %d",
			len("UID="+user+";PWD="+pass))
	}
	src := filepath.Join(appDir, "Sistema Intergrado de Controles y Presupuesto.exe")
	dst := filepath.Join(appDir, "Sistema Intergrado de Controles y Presupuesto_DOCKER.exe")

	nu := append(utf16le("UID="+user+";PWD="+pass), 0, 0)
	old := utf16le("Initial Catalog=SIDC")
	for len(nu) < len(old) {
		nu = append(nu, 0, 0)
	}

	// Idempotencia: si dst ya existe y ya tiene la credencial exacta, no reescribir
	if dstData, err := os.ReadFile(dst); err == nil {
		if bytes.Contains(dstData, nu) {
			out(fmt.Sprintf("_DOCKER.exe ya se encuentra configurado para %s", user))
			return nil
		}
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	origLen := len(data)

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

	if len(data) != origLen {
		return fmt.Errorf("integridad falló: tamaño de _DOCKER.exe (%d) difiere del original (%d)", len(data), origLen)
	}

	// Si existe Fotos/Principal.jpg, actualizamos los logos en _DOCKER.exe
	if logoPath := PrincipalLogoPath(appDir); logoPath != "" {
		if logoFile, err := os.Open(logoPath); err == nil {
			if img, _, err := image.Decode(logoFile); err == nil {
				if patched, rep, err := PatchLogos(data, img); err == nil && rep.Total() > 0 {
					data = patched
					out(fmt.Sprintf("Logos actualizados en _DOCKER.exe (%d imágenes, %d etiquetas)",
						rep.FormLogos+rep.Backgrounds+rep.Splashes, rep.Labels))
				}
			}
			logoFile.Close()
		}
	}

	// Respaldo de _DOCKER.exe si ya existía
	if _, err := os.Stat(dst); err == nil {
		bakPath := dst + ".bak"
		if curDst, err := os.ReadFile(dst); err == nil {
			_ = os.WriteFile(bakPath, curDst, 0644)
		}
	}

	tmpPath := dst + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	defer os.Remove(tmpPath)

	if err := os.WriteFile(dst, data, 0644); err != nil {
		return err
	}
	out(fmt.Sprintf("_DOCKER.exe OK (%d parche) -> %s", count, dst))
	return nil
}

// PrincipalLogoPath devuelve la ruta a Fotos/Principal.jpg si existe.
// Esta es la única fuente oficial de logos para la app y los reportes.
func PrincipalLogoPath(appDir string) string {
	p := filepath.Join(appDir, "Fotos", "Principal.jpg")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

// PatchAppAndReports actualiza los logos en el ejecutable y en todas las plantillas
// de Crystal Reports (.rpt) usando exclusivamente Fotos/Principal.jpg.
func PatchAppAndReports(appDir string, out func(string)) error {
	logoPath := PrincipalLogoPath(appDir)
	if logoPath == "" {
		expected := filepath.Join(appDir, "Fotos", "Principal.jpg")
		out(fmt.Sprintf("Aviso: No se encontró la imagen de origen %s", expected))
		return fmt.Errorf("no se encontró %s", expected)
	}

	logoFile, err := os.Open(logoPath)
	if err != nil {
		return fmt.Errorf("abriendo %s: %w", logoPath, err)
	}
	defer logoFile.Close()

	img, _, err := image.Decode(logoFile)
	if err != nil {
		return fmt.Errorf("decodificando %s: %w", logoPath, err)
	}

	// 1. Parchear ejecutables presentes (producción y/o docker)
	type exeTarget struct {
		filename string
		optional bool
	}
	targets := []exeTarget{
		{filename: "Sistema Intergrado de Controles y Presupuesto.exe", optional: false},
		{filename: "Sistema Intergrado de Controles y Presupuesto_DOCKER.exe", optional: true},
	}

	patchedCount := 0
	for _, t := range targets {
		exe := filepath.Join(appDir, t.filename)
		data, err := os.ReadFile(exe)
		if err != nil {
			if !t.optional {
				out(fmt.Sprintf("Aviso: %s no existe en %s", t.filename, appDir))
			}
			continue
		}
		patched, rep, err := PatchLogos(data, img)
		if err != nil {
			out(fmt.Sprintf("Error procesando %s: %v", t.filename, err))
			continue
		}
		if rep.Total() > 0 {
			if err := os.WriteFile(exe, patched, 0644); err != nil {
				out(fmt.Sprintf("ERROR al guardar %s (¿está en ejecución? cerrá la app antes de refrescar): %v", t.filename, err))
			} else {
				patchedCount++
				out(fmt.Sprintf("Logos SIDC actualizados en %s (%d imágenes, %d etiquetas)",
					t.filename, rep.FormLogos+rep.Backgrounds+rep.Splashes, rep.Labels))
			}
		} else {
			out(fmt.Sprintf("Sin modificaciones en %s (no se detectaron imágenes compatibles)", t.filename))
		}
	}

	if patchedCount == 0 {
		out("Aviso: No se actualizó ningún ejecutable en " + appDir)
	}

	// 2. Parchear plantillas de Crystal Reports en Reportes/
	repDir := filepath.Join(appDir, "Reportes")
	if _, err := os.Stat(repDir); err == nil {
		_, _ = PatchReportsLogos(repDir, img, out)
	} else {
		out(fmt.Sprintf("Aviso: Carpeta de reportes no encontrada (%s)", repDir))
	}

	return nil
}

// PatchAppLogos es un alias para PatchAppAndReports por compatibilidad con llamadores existentes.
func PatchAppLogos(appDir string, out func(string)) error {
	return PatchAppAndReports(appDir, out)
}

func utf16le(s string) []byte {
	b := make([]byte, 0, len(s)*2)
	for _, r := range s {
		b = append(b, byte(r), byte(r>>8))
	}
	return b
}
