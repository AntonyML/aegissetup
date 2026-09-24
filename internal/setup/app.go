// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"aegis-setup/internal/config"
	"aegis-setup/internal/securestore"
	"aegis-setup/internal/version"
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
	exe := filepath.Join(appDir, SIDCExeName)
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

// Nombres de archivos y constantes del ejecutable SIDC en VB6
const (
	SIDCBaseExeName     = "Sistema Intergrado de Controles y Presupuesto"
	SIDCExeName         = SIDCBaseExeName + "_AegisSetup.exe"
	SIDCOriginalExeName = SIDCBaseExeName + "_ORIGINAL.exe"
	MaxExeConnBudget    = 88
	SlotExeConnBytes    = 176
)

// SIDCDockerExeName usa la misma versión inyectada en todo el binario de Aegis.
// Es una variable porque version.Current puede recibir -ldflags en Release.
var SIDCDockerExeName = SIDCBaseExeName + "_Docker_AegisSetup_v" + version.Current + ".exe"

// FindOriginalExe ubica el ejecutable original intacto de SIDC.
// Revisa primero appDir y luego appDir/Respaldo_SIDC.
// NUNCA modifica ni borra este archivo.
func FindOriginalExe(appDir string) (string, error) {
	candidates := []string{
		filepath.Join(appDir, SIDCOriginalExeName),
		filepath.Join(appDir, "Respaldo_SIDC", SIDCOriginalExeName),
	}
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Size() > 0 {
			return p, nil
		}
	}
	return "", fmt.Errorf("no se encontró el ejecutable original intacto (%s)", SIDCOriginalExeName)
}

// BuildExeConnString construye la cadena de conexión optimizada para el buffer UTF-16LE de 88 caracteres del ejecutable.
// Si useWinAuth es true, no incluye credenciales UID/PWD.
// Si useWinAuth es false:
//  1. Intenta incluir Initial Catalog si cabe (máximo 88 chars).
//  2. Si excede 88 chars, omite Initial Catalog (el DSN ODBC de 32 bits ya apunta a la base de datos).
//  3. Si aun así excede 88 chars, devuelve error indicando el límite.
func BuildExeConnString(dsnName, database, user, pass string, useWinAuth bool) (string, error) {
	dsnName = strings.TrimSpace(dsnName)
	if dsnName == "" {
		dsnName = "SIDC_SQL"
	}
	database = strings.TrimSpace(database)
	if database == "" {
		database = "SIDC"
	}

	if useWinAuth {
		return fmt.Sprintf("Provider=MSDASQL.1;Data Source=%s;Initial Catalog=%s;", dsnName, database), nil
	}

	user = strings.TrimSpace(user)
	if user == "" {
		user = "sidc"
	}
	pass = strings.TrimSpace(pass)

	// Intento 1: Con Initial Catalog
	full := fmt.Sprintf("Provider=MSDASQL.1;Data Source=%s;Initial Catalog=%s;UID=%s;PWD=%s;", dsnName, database, user, pass)
	if len(full) <= MaxExeConnBudget {
		return full, nil
	}

	// Intento 2: Sin Initial Catalog (DSN ODBC ya tiene Database=<db>)
	compact := fmt.Sprintf("Provider=MSDASQL.1;Data Source=%s;UID=%s;PWD=%s;", dsnName, user, pass)
	if len(compact) <= MaxExeConnBudget {
		return compact, nil
	}

	maxPass := MaxExeConnBudget - len(fmt.Sprintf("Provider=MSDASQL.1;Data Source=%s;UID=%s;PWD=;", dsnName, user))
	return "", fmt.Errorf("contraseña demasiado larga para el ejecutable (máximo %d caracteres con usuario %q, recibí %d)", maxPass, user, len(pass))
}

// ReadExeConnString extrae la cadena de conexión UTF-16LE del ejecutable SIDC.
func ReadExeConnString(exePath string) (string, error) {
	data, err := os.ReadFile(exePath)
	if err != nil {
		return "", err
	}
	marker := utf16le("Provider=MSDASQL")
	idx := bytes.Index(data, marker)
	if idx < 0 {
		return "", fmt.Errorf("no se encontró la firma de conexión en %s", filepath.Base(exePath))
	}
	slot := data[idx:]
	if len(slot) > SlotExeConnBytes {
		slot = slot[:SlotExeConnBytes]
	}
	var runes []rune
	for i := 0; i+1 < len(slot); i += 2 {
		r := rune(uint16(slot[i]) | (uint16(slot[i+1]) << 8))
		if r == 0 {
			break
		}
		runes = append(runes, r)
	}
	return string(runes), nil
}

// PatchExeBuffer reemplaza la cadena de conexión y opcionalmente los logos en la imagen binaria en memoria.
func PatchExeBuffer(origData []byte, connStr string, logoImg image.Image) ([]byte, PatchReport, error) {
	if len(connStr) > MaxExeConnBudget {
		return nil, PatchReport{}, fmt.Errorf("cadena de conexión excede el presupuesto (%d > %d caracteres)", len(connStr), MaxExeConnBudget)
	}

	data := make([]byte, len(origData))
	copy(data, origData)

	marker := utf16le("Provider=MSDASQL")
	idx := bytes.Index(data, marker)
	if idx < 0 {
		return nil, PatchReport{}, fmt.Errorf("no se encontró el slot de conexión MSDASQL en el binario")
	}

	if idx+SlotExeConnBytes > len(data) {
		return nil, PatchReport{}, fmt.Errorf("el slot de conexión en 0x%X excede los límites del binario", idx)
	}

	for k := 0; k < SlotExeConnBytes; k++ {
		data[idx+k] = 0
	}

	newBytes := utf16le(connStr)
	copy(data[idx:idx+len(newBytes)], newBytes)

	var rep PatchReport
	if logoImg != nil {
		patched, r, err := PatchLogos(data, logoImg)
		if err != nil {
			return nil, PatchReport{}, fmt.Errorf("error aplicando logos: %w", err)
		}
		data = patched
		rep = r
	}

	if len(data) != len(origData) {
		return nil, PatchReport{}, fmt.Errorf("integridad falló: tamaño resultante (%d) != original (%d)", len(data), len(origData))
	}

	return data, rep, nil
}

// PatchSIDCApp regenera el ejecutable principal (y _DOCKER si aplica) a partir de _ORIGINAL.exe
// con la cadena de conexión optimizada y los logos actualizados.
// Es idempotente y nunca modifica ni borra _ORIGINAL.exe ni Respaldo_SIDC.
func PatchSIDCApp(cfg config.Config, appPass string, out func(string)) error {
	if cfg.AppDir == "" {
		return fmt.Errorf("app_dir no configurado")
	}
	if out == nil {
		out = func(string) {}
	}

	origPath, err := FindOriginalExe(cfg.AppDir)
	if err != nil {
		return err
	}

	origData, err := os.ReadFile(origPath)
	if err != nil {
		return fmt.Errorf("leyendo %s: %w", origPath, err)
	}
	origLen := len(origData)

	resolvedPass := securestore.ResolvePassword(appPass, func() string {
		return DSNPassword(cfg.DsnName)
	})

	connStr, err := BuildExeConnString(cfg.DsnName, cfg.Database, cfg.SQLUser, resolvedPass, cfg.UseWinAuth)
	if err != nil {
		return err
	}

	var logoImg image.Image
	if logoPath := PrincipalLogoPath(cfg.AppDir); logoPath != "" {
		if f, err := os.Open(logoPath); err == nil {
			logoImg, _, _ = image.Decode(f)
			f.Close()
		}
	}

	patchedData, rep, err := PatchExeBuffer(origData, connStr, logoImg)
	if err != nil {
		return fmt.Errorf("parcheando buffer: %w", err)
	}

	type targetExe struct {
		name string
	}
	targets := []targetExe{
		{name: SIDCExeName},
	}
	dockerExe := filepath.Join(cfg.AppDir, SIDCDockerExeName)
	if _, err := os.Stat(dockerExe); err == nil || cfg.DbMode == config.DbDocker {
		targets = append(targets, targetExe{name: SIDCDockerExeName})
	}

	newConnBytes := utf16le(connStr)
	manifest := generationManifest{
		Version:      version.Current,
		Source:       filepath.Base(origPath),
		SourceSHA256: sha256Hex(origData),
	}

	for _, t := range targets {
		dst := filepath.Join(cfg.AppDir, t.name)

		if curData, err := os.ReadFile(dst); err == nil && len(curData) == origLen {
			if bytes.Contains(curData, newConnBytes) {
				manifest.Executables = append(manifest.Executables, manifestFile{Name: t.name, Bytes: len(curData), SHA256: sha256Hex(curData)})
				out(fmt.Sprintf("%s: ya se encuentra configurado y alineado", t.name))
				continue
			}
		}

		killAppProcess(t.name)

		tmpPath := dst + ".tmp"
		if err := os.WriteFile(tmpPath, patchedData, 0755); err != nil {
			return fmt.Errorf("escribiendo temporal %s: %w", tmpPath, err)
		}

		if fi, err := os.Stat(tmpPath); err != nil || fi.Size() != int64(origLen) {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("falló verificación de integridad de tamaño en %s", tmpPath)
		}

		if err := os.Rename(tmpPath, dst); err != nil {
			_ = os.Remove(dst)
			if err2 := os.Rename(tmpPath, dst); err2 != nil {
				_ = os.Remove(tmpPath)
				return fmt.Errorf("reemplazando %s: %w", dst, err2)
			}
		}
		manifest.Executables = append(manifest.Executables, manifestFile{Name: t.name, Bytes: len(patchedData), SHA256: sha256Hex(patchedData)})

		sanitizedConn := sanitizeConnForLog(connStr, resolvedPass)
		out(fmt.Sprintf("%s OK: regenerado desde _ORIGINAL.exe (%s, %d logos, %d etiquetas)",
			t.name, sanitizedConn, rep.FormLogos+rep.Backgrounds+rep.Splashes, rep.Labels))
	}

	if logoImg != nil {
		repDir := filepath.Join(cfg.AppDir, "Reportes")
		if _, err := os.Stat(repDir); err == nil {
			_, _ = PatchReportsLogos(repDir, logoImg, out)
		}
	}

	if !cfg.UseWinAuth {
		repDir := filepath.Join(cfg.AppDir, "Reportes")
		if _, err := os.Stat(repDir); err == nil {
			patched, err := PatchReportsConnections(repDir, out)
			if err != nil {
				return fmt.Errorf("parcheando conexiones Crystal: %w", err)
			}
			out(fmt.Sprintf("Conexiones Crystal: %d plantillas, %d sustituciones", patched.Files, patched.Occurrences))
			manifest.ReportsFiles = patched.Files
			manifest.ReportOccurrences = patched.Occurrences
		}
	}
	if err := writeGenerationManifest(cfg.AppDir, manifest); err != nil {
		return fmt.Errorf("escribiendo manifiesto de generación: %w", err)
	}

	return nil
}

type manifestFile struct {
	Name   string `json:"name"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type generationManifest struct {
	Version           string         `json:"version"`
	Source            string         `json:"source"`
	SourceSHA256      string         `json:"source_sha256"`
	Executables       []manifestFile `json:"executables"`
	ReportsFiles      int            `json:"reports_files"`
	ReportOccurrences int            `json:"report_occurrences"`
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func writeGenerationManifest(appDir string, manifest generationManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(appDir, "AegisSetup.manifest.json")
	tmp, err := os.CreateTemp(appDir, ".aegis-manifest-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(path)
		if err2 := os.Rename(tmpPath, path); err2 != nil {
			return err2
		}
	}
	return nil
}

func recordManifestExecutable(appDir string, file manifestFile) error {
	path := filepath.Join(appDir, "AegisSetup.manifest.json")
	manifest := generationManifest{Version: version.Current}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &manifest); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	for i := range manifest.Executables {
		if manifest.Executables[i].Name == file.Name {
			manifest.Executables[i] = file
			return writeGenerationManifest(appDir, manifest)
		}
	}
	manifest.Executables = append(manifest.Executables, file)
	return writeGenerationManifest(appDir, manifest)
}

func sanitizeConnForLog(connStr, pass string) string {
	if pass != "" {
		return strings.ReplaceAll(connStr, pass, "****")
	}
	return connStr
}

// DockerPatchBudget es el maximo de chars que puede medir la clave del parche
// _DOCKER con un usuario dado. Sale de la aritmetica del slot: el parche
// reemplaza "Initial Catalog=SIDC" (20 chars) por "UID=<user>;PWD=<pass>".
// Es la fuente unica de la regla: la usan el parche y la validacion previa del
// TUI, para que el prompt no acepte algo que el parche va a rechazar.
func DockerPatchBudget(user string) int {
	return 20 - len("UID=") - len(";PWD=") - len(user)
}

// PatchDockerExe crea/copia el ejecutable Docker versionado con UID/PWD embebidos sin alargar
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
	src := filepath.Join(appDir, SIDCExeName)
	dst := filepath.Join(appDir, SIDCDockerExeName)

	nu := append(utf16le("UID="+user+";PWD="+pass), 0, 0)
	old := utf16le("Initial Catalog=SIDC")
	for len(nu) < len(old) {
		nu = append(nu, 0, 0)
	}

	// Idempotencia: si dst ya existe y ya tiene la credencial exacta, no reescribir
	if dstData, err := os.ReadFile(dst); err == nil {
		if bytes.Contains(dstData, nu) {
			out(fmt.Sprintf("%s ya se encuentra configurado para %s", SIDCDockerExeName, user))
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
		return fmt.Errorf("integridad falló: tamaño del ejecutable Docker (%d) difiere del original (%d)", len(data), origLen)
	}

	// Si existe Fotos/Principal.jpg, actualizamos los logos en el ejecutable Docker
	if logoPath := PrincipalLogoPath(appDir); logoPath != "" {
		if logoFile, err := os.Open(logoPath); err == nil {
			if img, _, err := image.Decode(logoFile); err == nil {
				if patched, rep, err := PatchLogos(data, img); err == nil && rep.Total() > 0 {
					data = patched
					out(fmt.Sprintf("Logos actualizados en ejecutable Docker (%d imágenes, %d etiquetas)",
						rep.FormLogos+rep.Backgrounds+rep.Splashes, rep.Labels))
				}
			}
			logoFile.Close()
		}
	}

	// Respaldo del ejecutable Docker si ya existía
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

	if err := os.Rename(tmpPath, dst); err != nil {
		_ = os.Remove(dst)
		if err2 := os.Rename(tmpPath, dst); err2 != nil {
			return err2
		}
	}
	if fi, err := os.Stat(dst); err != nil {
		return err
	} else if fi.Size() != int64(origLen) {
		return fmt.Errorf("integridad falló: tamaño publicado (%d) difiere del original (%d)", fi.Size(), origLen)
	}
	if err := recordManifestExecutable(appDir, manifestFile{Name: SIDCDockerExeName, Bytes: len(data), SHA256: sha256Hex(data)}); err != nil {
		return fmt.Errorf("actualizando manifiesto Docker: %w", err)
	}
	out(fmt.Sprintf("%s OK (%d parche) -> %s", SIDCDockerExeName, count, dst))
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
		{filename: SIDCExeName, optional: false},
		{filename: SIDCDockerExeName, optional: true},
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
