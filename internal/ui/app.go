// © Antony Monge López — Costa Rica — Céd. 604700548
package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"aegis-setup/internal/auth"
	"aegis-setup/internal/check"
	"aegis-setup/internal/config"
	"aegis-setup/internal/precheck"
	"aegis-setup/internal/securestore"
	"aegis-setup/internal/setup"
	"aegis-setup/internal/version"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
)

type screen int

const (
	screenMenu screen = iota
	screenWorking
	screenDone
	screenHelp
	screenAsk
	screenChecklist
	// screenPerfil es la primera pantalla cuando todavía no hay config: en qué PC
	// estamos. Va antes del menú porque el menú ya asume un ambiente.
	screenPerfil
	screenServer
	screenPort
	screenDatabase
	screenAuth
	screenSQLUser
	// screenAppDir pregunta dónde está la carpeta de SIDC. Va después del perfil porque
	// no es una preferencia sino un dato de esta PC: en una PC limpia no hay de dónde
	// sacarlo, y el default del desarrollador (C:\DEV\SIDC) hacía que el instalador
	// midiera la máquina donde se programó Aegis en vez de la que está instalando.
	screenAppDir
	screenLogin
	screenConfirmInstall
	screenAdvanced
)

type taskKind int

const (
	taskNone taskKind = iota
	taskSetupDB
	taskSetupApp
	taskCheck
	taskInstall
	taskRefreshLogos
	taskRepair
)

type taskFinishedMsg struct {
	kind  taskKind
	lines []string
	err   error
}

// checksMsg trae el checklist ya evaluado. Corre en background porque consulta
// el motor SQL: si fuera sincrónico, el menú tardaría en aparecer.

// Model es el TUI de Aegis: menú que EJECUTA el flujo, no solo lo dice.
type Model struct {
	cfg      config.Config
	cfgPath  string
	styles   Styles
	screen   screen
	cursor   int
	spinner  spinner.Model
	task     taskKind
	taskName string
	lines    []string
	taskErr  error
	width    int
	height   int
	helpHTML string
	viewport viewport.Model
	// secrets son las claves pedidas en el prompt, solo en memoria y solo para
	// esta corrida. No se escriben a disco: prod nunca guarda claves.
	secrets  map[string]string
	askQueue []string
	askInput textinput.Model
	askErr   string
	// presetServer es el server del preset "prod server" vía flag --server.
	// Vacío = comportamiento por defecto (CONTABILIDAD si la config actual
	// apunta a localhost, o la config que ya hubiera).
	presetServer  string
	presetPort    string
	portConfirmed bool
	presetDB      string
	// checks es el último checklist corrido. Vacío = todavía no corrió y por
	// eso no se bloquea nada: una sonda que no respondió no puede encerrar al
	// operador.
	checks      []precheck.Requisito
	verificando bool
	// bloqueo distingue "el paso falló" de "no te dejo empezar": el operador
	// reacciona distinto a cada uno y la pantalla no debería decir "error"
	// cuando la puerta hizo su trabajo.
	bloqueo bool
	// readyToOpen solo se activa cuando la instalación completa terminó bien.
	// Así el TUI ofrece abrir el SIDC que acaba de preparar, pero nunca abre una
	// estación a medias ni una corrida de reparación fallida.
	readyToOpen bool
	// primeraVez es "no hay config.json todavía". Mientras sea true no se mide
	// nada: sin perfil elegido, el checklist mide el ambiente por defecto, que no
	// es el de esta PC.
	primeraVez bool
	// srvInput y srvErr son el prompt del nombre del servidor en prod server.
	// Campo propio y no el de las claves: uno se muestra y el otro no, y eso
	// tiene que ser propiedad del campo, no de por dónde pasó el flujo.
	srvInput  textinput.Model
	srvErr    string
	portInput textinput.Model
	portErr   string
	dbInput   textinput.Model
	dbErr     string
	// presetWinAuth y presetSQLUser guardan la elección de autenticación del perfil pendiente.
	presetWinAuth   *bool
	presetSQLUser   string
	userInput       textinput.Model
	userErr         string
	authCursor      int
	pendingTask     taskKind
	pendingTaskName string
	// dirInput y dirErr son el prompt de la carpeta de SIDC, y perfilPendiente es el
	// perfil que se está aplicando mientras se contestan sus preguntas: el server
	// primero, la carpeta después, y la config se guarda recién cuando no falta ninguna.
	dirInput        textinput.Model
	dirErr          string
	perfilPendiente action
	// presetAppDir es el flag --app-dir: cuando viene, la carpeta ya está dicha y no hay
	// nada que preguntar (camino no interactivo).
	presetAppDir  string
	authMgr       *auth.Manager
	login         loginModel
	authenticated bool
	operatorUser  string
}

var menuItems = []menuEntry{
	{"1", "Instalación completa", "Configura y verifica la estación (DSN, OCX, Crystal, Logos, Check)", actInstall},
	{"2", "Reparar", "Corrige DSN, Crystal, OCX y logos si fallan", actRepair},
	{"3", "Verificar", "Verifica que App y DB se hablan (check)", actCheck},
	{"4", "Configuración avanzada", "Servidor, base de datos, credenciales y presets", actConfigAdvanced},
}

// launchSIDC es una seam pequeña para que la pantalla final se pueda probar sin
// arrancar el ejecutable legacy durante los tests del TUI.
var launchSIDC = setup.LaunchSIDC

var advancedItems = []menuEntry{
	{"1", "Configurar conexión", "Servidor, base de datos, autenticación y carpeta", actConfigManual},
	{"2", "Preset dev", "config dev docker localhost,14333", actPresetDev},
	{"3", "Preset FEMUCARIBE", "FEMUCARIBE\\AdministradorRed, Windows Auth, SIDC", actPresetProdLocal},
	{"4", "Preset prod server", "config prod servidor en la red, Windows Auth", actPresetProdServer},
	{"5", "Refrescar logos", "actualiza .exe y Reportes/ desde Fotos/Principal.jpg", actRefreshLogos},
	{"6", "Setup DB", "restaura el .bak como SIDC (compat, collation, logins)", actSetupDB},
}

// perfiles es la primera pregunta en una PC sin config: en qué PC estamos. Son las
// MISMAS acciones que los presets del menú (4/5/6) a propósito. Es la misma decisión
// tomada en dos momentos distintos, y con listas separadas el día que cambie una
// regla de ambiente el arranque y el menú dejarían configs distintas.
var perfiles = []menuEntry{
	{"1", "Pruebas", "SQL Server 2019 en Docker, en esta misma PC", actPresetDev},
	{"2", "Servidor FEMUCARIBE", "FEMUCARIBE\\AdministradorRed, Windows Auth, SIDC", actPresetProdLocal},
	{"3", "Producción en otro servidor", "SQL Server en otra PC de la red", actPresetProdServer},
}

type authOption struct {
	key     string
	name    string
	desc    string
	winAuth bool
}

var authOptions = []authOption{
	{"1", "Windows Authentication", "Para equipos en el dominio FEMUCARIBE. Sin claves en disco.", true},
	{"2", "Autenticación SQL Server", "Usuario y contraseña (ej. sidc). Para terminales fuera de dominio.", false},
}

// NewModel crea el modelo TUI con la config cargada.
func NewModel(cfg config.Config, cfgPath string) Model {
	styles := DefaultStyles()
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styles.Spinner
	vp := viewport.New()
	vp.MouseWheelEnabled = true
	return Model{cfg: cfg, cfgPath: cfgPath, styles: styles, spinner: sp, secrets: map[string]string{}, viewport: vp}
}

// SetPresetServer fija el server del preset "prod server" (flag --server del
// CLI). Devuelve una copia con el valor puesto; no muta el original.
func (m Model) SetPresetServer(s string) Model {
	m.presetServer = s
	m.portConfirmed = true
	return m
}

// SetPresetAppDir fija la carpeta de SIDC del perfil (flag --app-dir del CLI). Con
// esto puesto el TUI no pregunta: es el camino para la instalación desatendida.
func (m Model) SetPresetAppDir(dir string) Model {
	m.presetAppDir = dir
	return m
}

// SetAuthManager asocia el coordinador de autenticación Supabase al TUI y valida si existe sesión activa.
func (m Model) SetAuthManager(mgr *auth.Manager) Model {
	m.authMgr = mgr
	m.login = newLoginModel(mgr, m.styles)
	if mgr != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if mgr.IsAuthenticated(ctx) {
			m.authenticated = true
			m.operatorUser = mgr.CurrentUser(ctx)
		} else {
			m.authenticated = false
			m.operatorUser = ""
			m.screen = screenLogin
		}
	}
	return m
}

// SetPrimeraVez marca que no hay config todavía, así que lo primero es preguntar en
// qué PC estamos. Lo decide el CLI, que es quien sabe si el archivo existe: el
// modelo no toca el disco.
func (m Model) SetPrimeraVez(v bool) Model {
	m.primeraVez = v
	if v && (m.authMgr == nil || m.authenticated) {
		m.screen = screenPerfil
		m.cursor = 0
	}
	return m
}

type checksMsg struct{ rs []precheck.Requisito }

func (m Model) Init() tea.Cmd {
	// Sin perfil elegido o si falta autenticar no hay nada que medir:
	if m.primeraVez || m.screen == screenLogin {
		return nil
	}
	return m.runChecks()
}

// runChecks evalúa el checklist en background. El timeout de cada sonda lo pone
// la sonda misma, así que acá no hace falta un contexto.
func (m Model) runChecks() tea.Cmd {
	cfg, secret := m.cfg, m.secret
	return func() tea.Msg {
		return checksMsg{rs: precheck.Run(cfg, secret(envAppPassword), precheck.SondasReales())}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loginSuccessMsg:
		m.authenticated = true
		if msg.Session != nil {
			m.operatorUser = msg.Session.User.Email
		}
		if m.primeraVez {
			m.screen = screenPerfil
			m.cursor = 0
			return m, nil
		}
		m.screen = screenMenu
		m.verificando = true
		return m, m.runChecks()
	case loginFailedMsg:
		var cmd tea.Cmd
		m.login, cmd = m.login.update(msg)
		return m, cmd
	case cancelLoginMsg:
		if !m.authenticated {
			return m, tea.Quit
		}
		m.screen = screenMenu
		return m, nil
	case logoutMsg:
		if m.authMgr != nil {
			_ = m.authMgr.Logout()
		}
		m.authenticated = false
		m.operatorUser = ""
		m.screen = screenLogin
		m.login = newLoginModel(m.authMgr, m.styles)
		m.login.setSize(m.width, m.height)
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.login.setSize(msg.Width, msg.Height)
		switch m.screen {
		case screenDone:
			m = m.refreshViewportContent(m.doneBody(), 4, 2)
		case screenHelp:
			m = m.refreshViewportContent(m.helpHTML, 0, 2)
		case screenChecklist:
			m = m.refreshViewportContent(m.checklistBody(), 4, 2)
		}
		return m, nil
	case taskFinishedMsg:
		m.screen = screenDone
		m.lines = msg.lines
		m.taskErr = msg.err
		m.bloqueo = false
		m.readyToOpen = msg.kind == taskInstall && msg.err == nil
		m = m.setupViewport(m.doneBody(), 4, 2)
		// Se recalcula el checklist: al terminar un paso se desbloquea el
		// siguiente, y el operador lo tiene que ver sin pedirlo.
		m.verificando = true
		return m, m.runChecks()
	case checksMsg:
		m.checks = msg.rs
		m.verificando = false
		if m.screen == screenChecklist {
			m = m.refreshViewportContent(m.checklistBody(), 4, 2)
		}
		return m, nil
	case spinner.TickMsg:
		if m.screen == screenWorking {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.screen {
		case screenLogin:
			var cmd tea.Cmd
			m.login, cmd = m.login.update(msg)
			return m, cmd
		case screenPerfil:
			return m.updatePerfil(msg)
		case screenServer:
			return m.updateServer(msg)
		case screenPort:
			return m.updatePort(msg)
		case screenDatabase:
			return m.updateDatabase(msg)
		case screenAuth:
			return m.updateAuth(msg)
		case screenSQLUser:
			return m.updateSQLUser(msg)
		case screenAppDir:
			return m.updateAppDir(msg)
		case screenConfirmInstall:
			return m.updateConfirmInstall(msg)
		case screenAdvanced:
			return m.updateAdvanced(msg)
		case screenMenu:
			switch msg.String() {
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
				return m, nil
			case "down", "j":
				if m.cursor < len(menuItems)-1 {
					m.cursor++
				}
				return m, nil
			case "enter":
				return m.startItem(menuItems[m.cursor].action)
			case "q", "Q":
				return m, tea.Quit
			case "h", "H", "?":
				m.screen = screenHelp
				m.helpHTML = renderHelp(m.styles)
				m = m.setupViewport(m.helpHTML, 0, 2)
				return m, nil
			case "c", "C":
				m.screen = screenChecklist
				m = m.setupViewport(m.checklistBody(), 4, 2)
				return m, nil
			case "e", "E":
				return m.pedirPermisos()
			case "l", "L":
				if m.authMgr != nil && m.authenticated {
					_ = m.authMgr.Logout()
					m.authenticated = false
					m.operatorUser = ""
				}
				m.screen = screenLogin
				m.login = newLoginModel(m.authMgr, m.styles)
				m.login.setSize(m.width, m.height)
				return m, nil
			default:
				for _, it := range menuItems {
					if msg.String() == it.key {
						return m.startItem(it.action)
					}
				}
			}
			return m, nil
		case screenAsk:
			return m.updateAsk(msg)
		case screenDone, screenHelp, screenChecklist:
			// [E] vale también acá: es justo donde el checklist dice que falta
			// "permisos de administrador".
			if msg.String() == "e" || msg.String() == "E" {
				return m.pedirPermisos()
			}
			if (msg.String() == "c" || msg.String() == "C") && m.screen == screenChecklist {
				m.verificando = true
				return m, m.runChecks()
			}
			if (msg.String() == "a" || msg.String() == "A" || msg.String() == "enter") && m.screen == screenDone && m.readyToOpen {
				if err := launchSIDC(m.cfg.AppDir); err != nil {
					m.taskErr = err
					m.readyToOpen = false
					m = m.refreshViewportContent(m.doneBody(), 4, 2)
					return m, nil
				}
				return m, tea.Quit
			}
			if msg.String() == "esc" || msg.String() == "enter" || msg.String() == "q" {
				m.lines = nil
				m.taskErr = nil
				m.bloqueo = false
				m.readyToOpen = false
				// Si el perfil nunca se pudo guardar, volver al menú sería arrancar
				// con el ambiente por defecto sin que nadie lo haya elegido: se
				// vuelve a preguntar.
				if m.primeraVez {
					m.screen = screenPerfil
					m.cursor = 0
					return m, nil
				}
				m.screen = screenMenu
				return m, nil
			}
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		case screenWorking:
			return m, nil
		}
	default:
		switch m.screen {
		case screenLogin:
			var cmd tea.Cmd
			m.login, cmd = m.login.update(msg)
			return m, cmd
		case screenServer:
			return m.updateServer(msg)
		case screenPort:
			return m.updatePort(msg)
		case screenDatabase:
			return m.updateDatabase(msg)
		case screenSQLUser:
			return m.updateSQLUser(msg)
		case screenAppDir:
			return m.updateAppDir(msg)
		case screenAsk:
			return m.updateAsk(msg)
		case screenDone, screenHelp, screenChecklist:
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

// updatePerfil maneja la primera pantalla: en qué PC estamos. Acá no hay puerta ni
// checklist porque todavía no hay ambiente contra el cual medir.
func (m Model) updatePerfil(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down", "j":
		if m.cursor < len(perfiles)-1 {
			m.cursor++
		}
		return m, nil
	case "enter":
		return m.elegirPerfil(perfiles[m.cursor])
	case "q", "Q", "esc":
		// Salir sin elegir es válido; lo que no es válido es seguir al menú con
		// el perfil dev por defecto, que es el bug que esta pantalla arregla.
		return m, tea.Quit
	}
	for _, p := range perfiles {
		if msg.String() == p.key {
			m.cursor = indexDePerfil(p.action)
			return m.elegirPerfil(p)
		}
	}
	return m, nil
}

func indexDePerfil(a action) int {
	for i, p := range perfiles {
		if p.action == a {
			return i
		}
	}
	return 0
}

// elegirPerfil aplica el perfil elegido, pasando por sus preguntas. Prod server primero
// pregunta el nombre: un nombre inventado manda al operador a un "motor no alcanzable"
// que no describe su problema, y encima queda escrito en el config. La carpeta de SIDC
// se pregunta siempre que no la haya dicho un flag: es un dato de la PC, no un default.
func (m Model) elegirPerfil(p menuEntry) (tea.Model, tea.Cmd) {
	return m.aplicarPreset(p.action)
}

// aplicarPreset arranca la cadena de preguntas de un perfil y guarda cuál se está
// aplicando: la config no se toca hasta que estén todas contestadas, así que un perfil a
// medio contestar no puede quedar escrito en disco.
func (m Model) aplicarPreset(a action) (tea.Model, tea.Cmd) {
	m.perfilPendiente = a
	if a == actConfigManual {
		m.presetServer = ""
		m.presetPort = ""
		m.portConfirmed = false
		m.presetDB = ""
		m.presetWinAuth = nil
		m.presetSQLUser = ""
	} else if a == actPresetProdServer {
		if m.presetServer == "" {
			m.presetPort = ""
			m.portConfirmed = false
		}
	}
	return m.siguientePregunta("")
}

// siguientePregunta pide el próximo dato que falte y, si no falta ninguno, aplica el
// perfil. El server que vino por --server y la carpeta que vino por --app-dir ya están
// dichos: el camino no interactivo pasa de largo por las dos preguntas.
func (m Model) siguientePregunta(appDir string) (tea.Model, tea.Cmd) {
	a := m.perfilPendiente
	if (a == actPresetProdServer || a == actConfigManual) && m.presetServer == "" {
		m.srvInput = newServerInput(m.sugerenciaServer())
		m.srvErr = ""
		m.screen = screenServer
		return m, nil
	}
	if (a == actPresetProdServer || a == actConfigManual) && !m.portConfirmed {
		m.portInput = newPortInput(m.sugerenciaPort())
		m.portErr = ""
		m.screen = screenPort
		return m, nil
	}
	if a == actConfigManual && m.presetDB == "" {
		m.dbInput = newDatabaseInput(m.sugerenciaDatabase())
		m.dbErr = ""
		m.screen = screenDatabase
		return m, nil
	}
	if a == actConfigManual && m.presetWinAuth == nil {
		m.authCursor = 0
		m.screen = screenAuth
		return m, nil
	}
	if a == actConfigManual && m.presetWinAuth != nil && !*m.presetWinAuth && m.presetSQLUser == "" {
		m.userInput = newServerInput(m.sugerenciaSQLUser())
		m.userErr = ""
		m.screen = screenSQLUser
		return m, nil
	}
	if appDir == "" {
		appDir = m.presetAppDir
	}
	if appDir == "" {
		valor, ejemplo := m.sugerenciaAppDir()
		m.dirInput = newAppDirInput(valor, ejemplo)
		m.dirErr = ""
		m.screen = screenAppDir
		return m, nil
	}
	if a == actInstall {
		m.cfg.AppDir = appDir
		_ = m.cfg.Save(m.cfgPath)
		m.screen = screenConfirmInstall
		return m, nil
	}
	server := setup.UnirHostPuerto(m.presetServer, m.presetPort)
	return m.aplicarPerfil(a, server, appDir)
}

// sugerenciaServer es lo único que podemos ofrecer como ejemplo. No se precarga en
// el campo a propósito: un valor precargado se guarda tal cual sin que el operador
// lo lea, que es exactamente cómo se cuela un valor equivocado.
func (m Model) sugerenciaServer() string {
	if m.presetServer != "" {
		return m.presetServer
	}
	s := m.cfg.Server
	if s == "" || strings.HasPrefix(s, "localhost") || strings.HasPrefix(s, ".") || s == "127.0.0.1" {
		return "192.168.2.145"
	}
	h, _ := setup.SepararHostPuerto(s)
	return h
}

func (m Model) sugerenciaPort() string {
	if m.presetPort != "" {
		return m.presetPort
	}
	if m.cfg.Env != "dev" {
		_, p := setup.SepararHostPuerto(m.cfg.Server)
		if p != "" {
			return p
		}
	}
	if strings.Contains(m.presetServer, "\\") {
		return ""
	}
	return setup.PuertoPorDefecto
}

func newPortInput(sugerencia string) textinput.Model {
	return newServerInput(sugerencia)
}

func (m Model) sugerenciaDatabase() string {
	if m.presetDB != "" {
		return m.presetDB
	}
	if m.cfg.Database != "" {
		return m.cfg.Database
	}
	return "SIDC"
}

func (m Model) sugerenciaSQLUser() string {
	if m.presetSQLUser != "" {
		return m.presetSQLUser
	}
	if m.cfg.SQLUser != "" && m.cfg.SQLUser != "dev" {
		return m.cfg.SQLUser
	}
	return "sidc"
}

func newDatabaseInput(sugerencia string) textinput.Model {
	ti := newServerInput(sugerencia)
	if sugerencia != "" {
		ti.SetValue(sugerencia)
	}
	return ti
}

func newServerInput(sugerencia string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = sugerencia
	ti.Prompt = "> "
	// Width explícito: un textinput sin ancho dibuja solo el cursor y el operador
	// no ve lo que escribe ni el ejemplo de la sugerencia.
	ti.SetWidth(40)
	ti.CharLimit = 128
	ti.KeyMap.Paste = key.NewBinding(key.WithKeys("ctrl+v", "ctrl+alt+v"))
	ti.Focus()
	return ti
}

// sugerenciaAppDir es lo que ofrece el prompt de la carpeta de SIDC: el valor ya
// conocida (flag --app-dir, config actual o el repo deducido desde la posición del EXE)
// y, si no hay ninguna, un ejemplo para que el campo no quede mudo.
//
// El valor se precarga y el ejemplo no: un ejemplo precargado se guarda tal cual sin que
// nadie lo lea, y ahí SIDC puede estar en C:\SIDC, en C:\SIDC2014 o en otro disco. La
// carpeta del repo sí se puede precargar porque la posición del EXE la prueba: un
// aegis.exe en …\SIDC\AegisSetup\bin está dentro del repo, y el repo es la app de dev.
func (m Model) sugerenciaAppDir() (valor, ejemplo string) {
	if m.presetAppDir != "" {
		return m.presetAppDir, ""
	}
	if m.cfg.AppDir != "" {
		return m.cfg.AppDir, ""
	}
	if m.cfg.Env == "dev" {
		if d := carpetaDelRepo(); d != "" {
			return d, ""
		}
	}
	return "", `C:\SIDC`
}

// carpetaDelRepo deduce la raíz del repo desde la posición del propio EXE, y devuelve ""
// cuando el binario no está donde Aegis lo pone (un EXE suelto en el Escritorio o la
// carpeta temporal de go run): ahí no hay repo y no se puede inventar una carpeta.
func carpetaDelRepo() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	kit := filepath.Dir(filepath.Dir(exe)) // …\SIDC\AegisSetup\bin\aegis.exe -> …\SIDC\AegisSetup
	if !strings.EqualFold(filepath.Base(kit), "AegisSetup") {
		return ""
	}
	return filepath.Dir(kit)
}

func newAppDirInput(valor, ejemplo string) textinput.Model {
	ti := newServerInput(ejemplo)
	if valor != "" {
		ti.SetValue(valor)
	}
	return ti
}

func (m Model) updateServer(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc":
			// Vuelve a preguntar qué PC es: si venía del arranque, el perfil sigue sin
			// elegir y no se puede caer al menú.
			m.srvErr = ""
			if m.primeraVez {
				m.screen = screenPerfil
				return m, nil
			}
			m.screen = screenMenu
			return m, nil
		case "enter":
			server := strings.TrimSpace(m.srvInput.Value())
			if server == "" {
				m.srvErr = "Poné el nombre o IP del servidor (ej. 192.168.2.145, SIDC01 o CONTABILIDAD\\SQLEXPRESS)."
				return m, nil
			}
			host, port := setup.SepararHostPuerto(server)
			m.presetServer = host
			if port != "" {
				m.presetPort = port
			}
			m.portConfirmed = false
			return m.siguientePregunta("")
		}
	}
	var cmd tea.Cmd
	m.srvInput, cmd = m.srvInput.Update(msg)
	return m, cmd
}

func (m Model) updatePort(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.portErr = ""
			m.presetServer = ""
			m.presetPort = ""
			m.portConfirmed = false
			m.srvInput = newServerInput(m.sugerenciaServer())
			m.screen = screenServer
			return m, nil
		case "enter":
			port := strings.TrimSpace(m.portInput.Value())
			if port == "" {
				port = m.sugerenciaPort()
			} else {
				p, err := strconv.Atoi(port)
				if err != nil || p < 1 || p > 65535 {
					m.portErr = "Puerto inválido (debe ser un número entre 1 y 65535)."
					return m, nil
				}
			}
			m.presetPort = port
			m.portConfirmed = true
			return m.siguientePregunta("")
		}
	}
	var cmd tea.Cmd
	m.portInput, cmd = m.portInput.Update(msg)
	return m, cmd
}

var dbValidRe = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func (m Model) updateDatabase(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.dbErr = ""
			m.portConfirmed = false
			return m.siguientePregunta("")
		case "enter":
			db := strings.TrimSpace(m.dbInput.Value())
			if db == "" {
				db = "SIDC"
			}
			if !dbValidRe.MatchString(db) {
				m.dbErr = "Nombre de base de datos inválido (solo letras, números y guión bajo)."
				return m, nil
			}
			m.presetDB = db
			return m.siguientePregunta("")
		}
	}
	var cmd tea.Cmd
	m.dbInput, cmd = m.dbInput.Update(msg)
	return m, cmd
}

func (m Model) updateAuth(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.presetWinAuth = nil
		if m.perfilPendiente == actConfigManual {
			m.screen = screenDatabase
		} else {
			m.screen = screenServer
		}
		return m, nil
	case "up", "k":
		if m.authCursor > 0 {
			m.authCursor--
		}
		return m, nil
	case "down", "j":
		if m.authCursor < len(authOptions)-1 {
			m.authCursor++
		}
		return m, nil
	case "1":
		m.authCursor = 0
		return m.seleccionarAuth(true)
	case "2":
		m.authCursor = 1
		return m.seleccionarAuth(false)
	case "enter":
		return m.seleccionarAuth(authOptions[m.authCursor].winAuth)
	}
	return m, nil
}

func (m Model) seleccionarAuth(winAuth bool) (tea.Model, tea.Cmd) {
	m.presetWinAuth = &winAuth
	if winAuth {
		m.presetSQLUser = ""
	}
	return m.siguientePregunta("")
}

func (m Model) updateSQLUser(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.userErr = ""
			m.presetWinAuth = nil
			m.screen = screenAuth
			return m, nil
		case "enter":
			user := strings.TrimSpace(m.userInput.Value())
			if user == "" {
				user = m.sugerenciaSQLUser()
			}
			if user == "" {
				m.userErr = "El usuario no puede quedar vacío."
				return m, nil
			}
			m.presetSQLUser = user
			return m.siguientePregunta("")
		}
	}
	var cmd tea.Cmd
	m.userInput, cmd = m.userInput.Update(msg)
	return m, cmd
}

// updateAppDir pregunta dónde está la carpeta de SIDC. Valida antes de guardar: una ruta
// relativa se resolvería contra el directorio de trabajo del proceso, así que la misma
// config mediría carpetas distintas según desde dónde se lance Aegis.
func (m Model) updateAppDir(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc":
			// Igual que el nombre del servidor: si venía del arranque, el perfil sigue sin
			// elegir y caer al menú dejaría la config por defecto, que no es la de esta PC.
			m.dirErr = ""
			if m.perfilPendiente == actInstall {
				m.screen = screenConfirmInstall
				return m, nil
			}
			if m.perfilPendiente == actConfigManual {
				if m.presetWinAuth != nil && !*m.presetWinAuth {
					m.screen = screenSQLUser
					return m, nil
				}
				m.screen = screenAuth
				return m, nil
			}
			if m.perfilPendiente == actPresetProdServer {
				m.portConfirmed = false
				return m.siguientePregunta("")
			}
			if m.primeraVez {
				m.screen = screenPerfil
				return m, nil
			}
			m.screen = screenMenu
			return m, nil
		case "enter":
			dir := strings.TrimSpace(m.dirInput.Value())
			if dir == "" {
				m.dirErr = "Poné la carpeta donde está SIDC (ej. C:\\SIDC). Es la que tiene el .exe de SIDC y la carpeta Reportes."
				return m, nil
			}
			// La ruta relativa se rechaza acá y no en Validate: así el operador corrige en
			// la misma pantalla en vez de caer al error del perfil y tener que empezar de
			// nuevo. El motivo es el mismo: se resolvería contra el directorio de trabajo.
			if !config.RutaAbsoluta(dir) {
				m.dirErr = "Poné la ruta completa, con la letra del disco (ej. C:\\SIDC): " + dir + " se resolvería desde donde se lance Aegis."
				return m, nil
			}
			return m.siguientePregunta(dir)
		}
	}
	var cmd tea.Cmd
	m.dirInput, cmd = m.dirInput.Update(msg)
	return m, cmd
}

func (m Model) updateConfirmInstall(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "Q":
		m.screen = screenMenu
		m.cursor = 0
		return m, nil
	case "c", "C":
		m.screen = screenAdvanced
		m.cursor = 0
		return m, nil
	case "enter":
		if m.cfg.AppDir == "" {
			valor, ejemplo := m.sugerenciaAppDir()
			m.dirInput = newAppDirInput(valor, ejemplo)
			m.dirErr = ""
			m.screen = screenAppDir
			m.perfilPendiente = actInstall
			return m, nil
		}
		if need := secretNeeds(m.cfg, m.hasSecret); len(need) > 0 {
			_ = m.cfg.Save(m.cfgPath)
			return m.beginAsk(taskInstall, stepTitle(taskInstall), need)
		}
		_ = m.cfg.Save(m.cfgPath)
		return m.startTask(taskInstall, stepTitle(taskInstall))
	}
	return m, nil
}

func (m Model) updateAdvanced(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down", "j":
		if m.cursor < len(advancedItems)-1 {
			m.cursor++
		}
		return m, nil
	case "enter":
		return m.startItem(advancedItems[m.cursor].action)
	case "esc", "q", "Q":
		m.screen = screenMenu
		m.cursor = 0
		return m, nil
	default:
		for _, it := range advancedItems {
			if msg.String() == it.key {
				return m.startItem(it.action)
			}
		}
	}
	return m, nil
}

// aplicarPerfil guarda la config y RECALCULA el checklist. Lo segundo no es un
// adorno: el checklist viejo describe la máquina según el ambiente anterior, así que
// conservarlo dejaría al operador trabado por requisitos del ambiente que acaba de
// abandonar (Docker, por ejemplo, en una PC de producción).
func (m Model) aplicarPerfil(a action, server, appDir string) (tea.Model, tea.Cmd) {
	cfg, err := presetConfig(m.cfg, a, server, appDir)
	if err != nil {
		return m.errorDePerfil(err)
	}
	if m.presetDB != "" {
		cfg.Database = m.presetDB
	}
	if m.presetWinAuth != nil {
		cfg.UseWinAuth = *m.presetWinAuth
		if !*m.presetWinAuth {
			if m.presetSQLUser != "" {
				cfg.SQLUser = m.presetSQLUser
			} else if cfg.SQLUser == "" || cfg.SQLUser == "dev" {
				cfg.SQLUser = "sidc"
			}
		}
	}
	if err := cfg.Validate(); err != nil {
		return m.errorDePerfil(err)
	}
	if err := cfg.Save(m.cfgPath); err != nil {
		return m.errorDePerfil(err)
	}
	m.cfg = cfg
	m.presetServer = ""
	m.presetPort = ""
	m.portConfirmed = false
	m.presetDB = ""
	m.presetWinAuth = nil
	m.presetSQLUser = ""
	m.primeraVez = false
	m.srvErr = ""
	m.portErr = ""
	m.dbErr = ""
	m.userErr = ""
	m.dirErr = ""
	m.bloqueo = false
	m.taskErr = nil
	m.task = taskNone
	m.taskName = "PERFIL"
	m.screen = screenDone
	m.cursor = 0
	m.lines = []string{
		fmt.Sprintf("config guardada en %s", m.cfgPath),
		fmt.Sprintf("env=%s db_mode=%s server=%s db=%s auth=%s", cfg.Env, cfg.DbMode, cfg.Server, cfg.Database, authLabel(cfg)),
		fmt.Sprintf("app_dir=%s", cfg.AppDir),
		"",
		"Ya está midiendo esta config: mirá el checklist con [C].",
	}
	m = m.setupViewport(m.doneBody(), 4, 2)
	m.checks = nil
	m.verificando = true
	return m, m.runChecks()
}

func (m Model) errorDePerfil(err error) (tea.Model, tea.Cmd) {
	m.screen = screenDone
	m.task = taskNone
	m.taskName = "PERFIL"
	m.taskErr = err
	m.lines = nil
	m = m.setupViewport(m.doneBody(), 4, 2)
	return m, nil
}

func authLabel(cfg config.Config) string {
	if cfg.UseWinAuth {
		return "Windows Auth"
	}
	return "SQL Auth (" + cfg.SQLUser + ")"
}

// updateAsk maneja el prompt de claves: una por vez, sin eco, con validación
// antes de arrancar (así no se descubre al final que la clave no servía).
func (m Model) updateAsk(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.screen = screenMenu
			m.askQueue = nil
			m.askErr = ""
			return m, nil
		case "enter":
			name := m.askQueue[0]
			value := m.askInput.Value()
			if err := validateSecret(m.cfg, name, value); err != nil {
				m.askErr = err.Error()
				return m, nil
			}
			m.secrets[name] = value
			m.askQueue = m.askQueue[1:]
			m.askErr = ""
			if len(m.askQueue) == 0 {
				task := m.pendingTask
				taskName := m.pendingTaskName
				m.pendingTask = taskNone
				m.pendingTaskName = ""
				if task == taskNone {
					task = taskInstall
					taskName = "INSTALACIÓN COMPLETA"
				}
				return m.startTask(task, taskName)
			}
			m.askInput = newSecretInput()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.askInput, cmd = m.askInput.Update(msg)
	return m, cmd
}

func newSecretInput() textinput.Model {
	ti := textinput.New()
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'
	ti.Placeholder = "(no se muestra)"
	ti.KeyMap.Paste = key.NewBinding(key.WithKeys("ctrl+v", "ctrl+alt+v"))
	ti.Focus()
	return ti
}

// beginAsk pide las claves que faltan y despues corre la tarea pendiente.
func (m Model) beginAsk(task taskKind, name string, need []string) (tea.Model, tea.Cmd) {
	m.pendingTask = task
	m.pendingTaskName = name
	m.askQueue = need
	m.askInput = newSecretInput()
	m.askErr = ""
	m.screen = screenAsk
	return m, nil
}

// secret busca la clave primero en lo que se pidió en el prompt y si no en el
// entorno. Nunca en config.json.
func (m Model) secret(name string) string {
	if v, ok := m.secrets[name]; ok && v != "" {
		return v
	}
	return os.Getenv(name)
}

func (m Model) hasSecret(name string) bool { return m.secret(name) != "" }

func (m Model) startItem(a action) (tea.Model, tea.Cmd) {
	// La puerta se consulta acá y no al dibujar: si el operador aprieta el
	// número de una opción trabada, tiene que recibir qué le falta, no un
	// arranque que va a fallar a mitad de camino.
	if faltan := bloqueosDe(a, m.checks); len(faltan) > 0 {
		m.screen = screenDone
		m.bloqueo = true
		m.task = taskNone
		m.taskName = nombreDeAccion(a)
		m.taskErr = nil
		m.lines = append(arreglosDe(faltan), "", "Resolvé eso y volvé a intentar: el checklist se recalcula solo.")
		m = m.setupViewport(m.doneBody(), 4, 2)
		return m, nil
	}
	switch a {
	case actInstall:
		// Autodetectar carpeta si está vacía
		if m.cfg.AppDir == "" {
			if config.ExisteDir(`C:\SIDC`) {
				m.cfg.AppDir = `C:\SIDC`
			} else if d := carpetaDelRepo(); d != "" {
				m.cfg.AppDir = d
			}
		}
		// En Docker siempre es SQL Auth. En prod remoto, autodetectar si es WORKGROUP vs Dominio
		if m.cfg.DbMode != config.DbDocker && m.cfg.Server != "" {
			if !setup.IsServerLocal(m.cfg.Server) {
				bestAuth, _ := setup.DetectBestAuth(context.Background(), m.cfg.Server, m.cfg.Database)
				m.cfg.UseWinAuth = bestAuth
				if !bestAuth && (m.cfg.SQLUser == "" || m.cfg.SQLUser == "dev") {
					m.cfg.SQLUser = "sidc"
				}
			}
		}
		m.screen = screenConfirmInstall
		return m, nil
	case actRepair:
		if need := secretNeeds(m.cfg, m.hasSecret); len(need) > 0 {
			resolved := securestore.ResolvePassword("", func() string { return setup.DSNPassword(m.cfg.DsnName) })
			if resolved != "" {
				m.secrets[envAppPassword] = resolved
				return m.startTask(taskRepair, stepTitle(taskRepair))
			}
			return m.beginAsk(taskRepair, stepTitle(taskRepair), need)
		}
		return m.startTask(taskRepair, stepTitle(taskRepair))
	case actConfigAdvanced:
		m.screen = screenAdvanced
		m.cursor = 0
		return m, nil
	case actSetupDB:
		return m.startTask(taskSetupDB, stepTitle(taskSetupDB))
	case actSetupApp:
		if need := secretNeeds(m.cfg, m.hasSecret); len(need) > 0 {
			return m.beginAsk(taskSetupApp, stepTitle(taskSetupApp), need)
		}
		return m.startTask(taskSetupApp, stepTitle(taskSetupApp))
	case actCheck:
		if need := secretNeeds(m.cfg, m.hasSecret); len(need) > 0 {
			return m.beginAsk(taskCheck, stepTitle(taskCheck), need)
		}
		return m.startTask(taskCheck, stepTitle(taskCheck))
	case actRefreshLogos:
		return m.startTask(taskRefreshLogos, stepTitle(taskRefreshLogos))
	case actPresetDev, actPresetProdLocal, actPresetProdServer, actConfigManual:
		// Los presets preguntan lo que no se puede adivinar: el nombre del servidor en
		// prod server y, en cualquiera, dónde está la carpeta de SIDC. Un servidor
		// adivinado se ve igual de válido que uno real hasta que falla la conexión, y
		// una carpeta adivinada mide otra máquina (el default apuntaba al repo de dev).
		return m.aplicarPreset(a)
	}
	return m, nil
}

func (m Model) startTask(k taskKind, name string) (tea.Model, tea.Cmd) {
	m.screen = screenWorking
	m.task = k
	m.taskName = name
	m.lines = nil
	m.taskErr = nil
	m.bloqueo = false
	cfg, secret := m.cfg, m.secret
	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		var lines []string
		emit := func(s string) { lines = append(lines, s) }
		timeout := 15 * time.Minute
		if k == taskInstall {
			timeout = 45 * time.Minute
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		var err error
		if k == taskInstall {
			seq := installSeq()
			for i, step := range seq {
				if i > 0 {
					emit("")
				}
				emit(stepLabel(i, len(seq), step))
				if err = runStep(ctx, cfg, step, secret, emit); err != nil {
					// Se corta en el paso que falla, diciendo cual: dejar seguir
					// solo agrega ruido encima del error real.
					err = fmt.Errorf("%s: %w", stepTitle(step), err)
					break
				}
			}
		} else {
			err = runStep(ctx, cfg, k, secret, emit)
		}
		return taskFinishedMsg{kind: k, lines: lines, err: err}
	})
}

// runStep es la unica implementacion de cada paso. La comparten las entradas
// sueltas del menu y la instalacion completa: si se agrega un paso, se agrega
// una sola vez.
func runStep(ctx context.Context, cfg config.Config, k taskKind, secret func(string) string, emit func(string)) error {
	switch k {
	case taskSetupDB:
		// El kit de Docker va primero: sin el compose no hay motor que levantar, y el
		// restore que viene después lo necesita arriba.
		for _, f := range instalarCompose(cfg.DbMode, emit) {
			emit("DOCKER PENDIENTE: " + f)
		}
		bak, err := setup.FindNewestBakCfg(cfg)
		if err != nil {
			return err
		}
		emit("BAK: " + bak)
		return setup.SetupDB(ctx, cfg, bak, secret(envSAPassword), secret(envAppPassword), emit)

	case taskSetupApp:
		appPass := secret(envAppPassword)
		for _, miss := range setup.CheckAppFiles(cfg.AppDir) {
			emit("FALTA: " + miss)
		}
		savePWD := !cfg.UseWinAuth // solo dev/docker guarda PWD
		if err := escribirDSN(cfg, appPass, savePWD, emit); err != nil {
			return err
		}
		// Los controles de VB6 salen del propio binario: en una PC limpia no hay
		// carpeta de la PC vieja de dónde copiarlos.
		for _, f := range instalarOCX(cfg.LegacyDir, emit) {
			emit("OCX PENDIENTE: " + f)
		}
		// El runtime de Crystal sale del propio binario: en una PC limpia no hay
		// carpeta de instalación de Crystal Reports que copiar.
		instalarCrystal(emit)
		if err := setup.PatchAppLogos(cfg.AppDir, emit); err != nil {
			emit("AVISO LOGOS: " + err.Error())
		}
		if !cfg.UseWinAuth && appPass != "" {
			if err := setup.PatchCrystalODBCBridge(cfg.AppDir, cfg.SQLUser, appPass, emit); err != nil {
				emit("AVISO PUENTE CRYSTAL: " + err.Error())
			}
		}
		if cfg.DbMode == config.DbDocker && !cfg.UseWinAuth {
			if appPass == "" {
				return fmt.Errorf("falta %s para el parche _DOCKER", envAppPassword)
			}
			return parcheDocker(cfg.AppDir, cfg.SQLUser, appPass, emit)
		}
		return nil

	case taskCheck:
		fails := 0
		for _, r := range check.Run(ctx, cfg, secret(envAppPassword)) {
			mark := "OK  "
			if !r.OK {
				mark = "FAIL"
				fails++
			}
			emit(fmt.Sprintf("%s %-22s %s", mark, r.Name, r.Info))
		}
		if fails > 0 {
			// Antes esto no era error y el TUI mostraba "OK" con el check en
			// rojo: el operador se enteraba al reves.
			return fmt.Errorf("check: %d fallo(s)", fails)
		}
		return nil

	case taskRefreshLogos:
		if cfg.AppDir == "" {
			return fmt.Errorf("app_dir sin configurar (elegí perfil o pasá --app-dir)")
		}
		emit(fmt.Sprintf("Carpeta SIDC: %s", cfg.AppDir))
		if err := setup.PatchAppAndReports(cfg.AppDir, emit); err != nil {
			return err
		}
		if !cfg.UseWinAuth {
			dockerExe := filepath.Join(cfg.AppDir, setup.SIDCDockerExeName)
			if _, err := os.Stat(dockerExe); err != nil {
				appPass := secret(envAppPassword)
				if appPass == "" {
					appPass = os.Getenv(envAppPassword)
				}
				if appPass != "" {
					_ = parcheDocker(cfg.AppDir, cfg.SQLUser, appPass, emit)
				}
			}
		}
		return nil

	case taskRepair:
		emit("== REPARACIÓN DE ESTACIÓN ==")
		// 1. DSN
		valid, diffs := setup.ValidateDSN(cfg, secret(envAppPassword))
		if !valid {
			emit(fmt.Sprintf("DSN desalineado (%s). Reparando...", strings.Join(diffs, "; ")))
			if err := setup.RepairDSN(cfg, secret(envAppPassword), emit); err != nil {
				emit("AVISO DSN: " + err.Error())
			} else {
				emit("DSN SIDC_SQL reparado.")
			}
		} else {
			emit("DSN SIDC_SQL: OK")
		}

		// 2. Crystal
		if miss := setup.CheckCrystal(); len(miss) > 0 {
			emit(fmt.Sprintf("Crystal Reports incompleto (%d componentes faltantes). Instalando...", len(miss)))
			instalarCrystal(emit)
		} else {
			emit("Crystal Reports runtime: OK")
		}

		// 3. OCX
		emit("Verificando controles OCX...")
		for _, f := range instalarOCX(cfg.LegacyDir, emit) {
			emit("OCX: " + f)
		}

		// 4. Logos
		if cfg.AppDir != "" {
			if err := setup.PatchAppLogos(cfg.AppDir, emit); err != nil {
				emit("AVISO LOGOS: " + err.Error())
			} else {
				emit("Logos: OK")
			}
		}

		// 5. Check
		emit("")
		emit("== VERIFICACIÓN FINAL ==")
		return runStep(ctx, cfg, taskCheck, secret, emit)
	}
	return nil
}

// presetConfig calcula la config que deja un preset, sin tocar disco. El
// presetServer viene del flag --server y solo aplica al preset "prod server":
// deja elegir el servidor sin editar config.json a mano. Vacío = CONTABILIDAD
// si la config actual todavía apunta a localhost (dev), o la que ya hubiera.
//
// appDir es la carpeta de SIDC contestada por el operador (o el flag --app-dir): vacía
// significa "no la cambio", no "dejala vacía".
func presetConfig(cfg config.Config, a action, presetServer, appDir string) (config.Config, error) {
	switch a {
	case actPresetDev:
		cfg.Env, cfg.DbMode, cfg.Server = "dev", config.DbDocker, "localhost,14333"
		cfg.UseWinAuth, cfg.Driver = false, "ODBC Driver 17 for SQL Server"
	case actPresetProdLocal:
		cfg.Env, cfg.DbMode, cfg.Server = "prod", config.DbLocal, "localhost"
		cfg.UseWinAuth, cfg.Driver = true, "SQL Server"
	case actPresetProdServer:
		cfg.Env, cfg.DbMode = "prod", config.DbServer
		if presetServer != "" {
			cfg.Server = presetServer
		} else if cfg.Server == "localhost,14333" || cfg.Server == "localhost" {
			cfg.Server = "CONTABILIDAD"
		}
		cfg.UseWinAuth, cfg.Driver = true, "SQL Server"
	case actConfigManual:
		cfg.Env, cfg.DbMode = "prod", config.DbServer
		if presetServer != "" {
			cfg.Server = presetServer
		}
		cfg.UseWinAuth, cfg.Driver = true, "SQL Server"
	}
	if appDir != "" {
		cfg.AppDir = appDir
	}
	if err := cfg.Validate(); err != nil {
		return config.Config{}, err
	}
	return cfg, nil
}

func (m Model) View() tea.View {
	var content string
	switch m.screen {
	case screenLogin:
		content = m.login.view()
	case screenWorking:
		content = m.viewWorking()
	case screenDone:
		content = m.viewDone()
	case screenHelp:
		content = m.viewHelp()
	case screenChecklist:
		content = m.viewChecklist()
	case screenAsk:
		content = m.viewAsk()
	case screenPerfil:
		content = m.viewPerfil()
	case screenServer:
		content = m.viewServer()
	case screenPort:
		content = m.viewPort()
	case screenDatabase:
		content = m.viewDatabase()
	case screenAuth:
		content = m.viewAuth()
	case screenSQLUser:
		content = m.viewSQLUser()
	case screenAppDir:
		content = m.viewAppDir()
	case screenConfirmInstall:
		content = m.viewConfirmInstall()
	case screenAdvanced:
		content = m.viewAdvanced()
	default:
		content = m.viewMenu()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m Model) viewMenu() string {
	s := m.styles
	var b strings.Builder

	var badges []string
	badges = append(badges, s.BadgeInfo.Render("PERFIL: "+strings.ToUpper(m.cfg.Env)))
	if m.authenticated && m.operatorUser != "" {
		badges = append(badges, s.BadgeSuccess.Render("OPERADOR: "+m.operatorUser))
	} else {
		badges = append(badges, s.BadgeMuted.Render("OPERADOR: ANÓNIMO"))
	}
	b.WriteString(fmt.Sprintf("%s  %s  %s\n\n", s.AppTitle.Render("AEGIS SETUP"), strings.Join(badges, "  "), s.Subtitle.Render("v"+version.Current)))
	b.WriteString(s.Muted.Render(fmt.Sprintf("db_mode=%s · server=%s · db=%s · auth=%s", m.cfg.DbMode, m.cfg.Server, m.cfg.Database, authLabel(m.cfg))) + "\n")
	b.WriteString(m.checklistLine() + "\n\n")
	b.WriteString(s.SectionHeader.Render("FLUJO PRINCIPAL") + "\n")
	for i, it := range menuItems {
		cur := "  "
		if m.cursor == i {
			cur = s.Cursor.Render("> ")
		}
		trabada := bloqueosDe(it.action, m.checks)
		nombre := s.Value.Render(it.name)
		if len(trabada) > 0 {
			nombre = s.Muted.Render(it.name)
		}
		linea := fmt.Sprintf("%s%s %s %s", cur, s.Key.Render("["+it.key+"]"), nombre, s.Muted.Render("- "+it.desc))
		if len(trabada) > 0 {
			linea += "  " + s.Error.Render("(falta: "+titulosDe(trabada)+")")
		}
		b.WriteString(linea + "\n")
	}
	labelL := " operador   "
	if m.authenticated {
		labelL = " cerrar sesión   "
	}
	b.WriteString("\n" + s.HelpBar.Render(
		s.Key.Render("[↑↓/Enter]")+s.Desc.Render(" elegir   ")+
			s.Key.Render("[C]")+s.Desc.Render(" checklist   ")+
			s.Key.Render("[E]")+s.Desc.Render(" permisos   ")+
			s.Key.Render("[L]")+s.Desc.Render(labelL)+
			s.Key.Render("[H]")+s.Desc.Render(" ayuda   ")+
			s.Key.Render("[Q]")+s.Desc.Render(" salir")))
	return s.Box.Render(b.String())
}

// checklistLine pinta la línea de estado del checklist. El color es la
// información: verde si nada traba, rojo si algo sí.
func (m Model) checklistLine() string {
	s := m.styles
	txt := resumenChecklist(m.checks, m.verificando)
	switch {
	case m.verificando || len(m.checks) == 0:
		return s.Muted.Render(txt)
	case len(precheck.Faltantes(m.checks)) == 0:
		return s.Success.Render(txt)
	default:
		return s.Error.Render(txt)
	}
}

func (m Model) checklistBody() string {
	s := m.styles
	var b strings.Builder
	for _, r := range m.checks {
		marca := s.Success.Render(r.Estado.Marca())
		switch r.Estado {
		case precheck.EstadoFalta:
			marca = s.Error.Render(r.Estado.Marca())
		case precheck.EstadoAviso:
			marca = s.Warning.Render(r.Estado.Marca())
		}
		b.WriteString(fmt.Sprintf("%s %s\n", marca, s.Value.Render(r.Titulo)))
		b.WriteString("    " + s.Muted.Render(r.Detalle) + "\n")
		if r.Estado != precheck.EstadoOK {
			b.WriteString("    " + s.Muted.Render("por qué: ") + s.Desc.Render(r.Motivo) + "\n")
			b.WriteString("    " + s.Muted.Render("arreglo: ") + s.Value.Render(r.Arreglo) + "\n")
			if deps := accionesQueDependenDe(r.ID); len(deps) > 0 {
				b.WriteString("    " + s.Muted.Render("traba: "+strings.Join(deps, ", ")) + "\n")
			}
		}
		b.WriteString("\n")
	}

	if bloquean := precheck.Bloqueantes(m.checks); len(bloquean) > 0 {
		b.WriteString(s.Error.Render(resumenFaltantes(len(bloquean))) + "\n\n")
	} else if faltan := precheck.Faltantes(m.checks); len(faltan) > 0 {
		b.WriteString(s.Warning.Render(fmt.Sprintf("%d requisitos sin cumplir, y ninguno traba: son resultado del flujo (la base y el DSN), no condición para empezar.", len(faltan))) + "\n\n")
	} else {
		b.WriteString(s.Success.Render("Todo en orden: el flujo completo está disponible.") + "\n\n")
	}
	if m.verificando {
		b.WriteString(s.Muted.Render("Recalculando...") + "\n\n")
	}
	return b.String()
}

// viewChecklist es el detalle: qué falta, por qué importa, qué hacer y qué
// opción del menú queda trabada. Es el plan de trabajo de la PC.
func (m Model) viewChecklist() string {
	s := m.styles
	var header strings.Builder
	header.WriteString(s.AppTitle.Render("CHECKLIST DE LA MAQUINA") + "\n")
	header.WriteString(s.Subtitle.Render(fmt.Sprintf("env=%s db_mode=%s server=%s db=%s", m.cfg.Env, m.cfg.DbMode, m.cfg.Server, m.cfg.Database)) + "\n\n")

	if m.verificando || len(m.checks) == 0 {
		header.WriteString(fmt.Sprintf("%s %s\n", m.spinner.View(), s.Info.Render("Verificando la máquina... consulta el motor SQL, puede tardar unos segundos.")))
		header.WriteString("\n" + s.HelpBar.Render(s.Key.Render("[Esc]")+s.Desc.Render(" volver al menú")))
		return s.Box.Render(header.String())
	}

	footer := s.HelpBar.Render(
		s.Key.Render("[↑↓/PgUp/PgDn]") + s.Desc.Render(" desplazar   ") +
			s.Key.Render("[Esc/Enter]") + s.Desc.Render(" volver al menú   ") +
			s.Key.Render("[C]") + s.Desc.Render(" recalcular   ") +
			s.Key.Render("[E]") + s.Desc.Render(" permisos"))

	if m.height <= 0 {
		return s.Box.Render(header.String() + m.checklistBody() + "\n" + footer)
	}
	return s.Box.Render(header.String() + m.viewport.View() + "\n\n" + footer)
}

// viewPerfil es la primera pantalla en una PC sin config. Explica la consecuencia
// de cada opción, no la jerga: el operador sabe si esta PC es de pruebas o la de
// producción; "db_mode=local" no le dice nada.
func (m Model) viewPerfil() string {
	s := m.styles
	var b strings.Builder
	b.WriteString(s.AppTitle.Render("AEGIS SETUP") + "\n")
	b.WriteString(s.Subtitle.Render("Todavía no hay config en esta PC: decime en qué PC estamos.") + "\n\n")
	b.WriteString(s.SectionHeader.Render("ESTA PC ES...") + "\n")
	for i, p := range perfiles {
		cur := "  "
		if m.cursor == i {
			cur = s.Cursor.Render("> ")
		}
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", cur, s.Key.Render("["+p.key+"]"),
			s.Value.Render(p.name), s.Muted.Render("- "+p.desc)))
	}
	b.WriteString("\n" + s.Muted.Render("Se guarda en ") + s.Value.Render(m.cfgPath) + s.Muted.Render(" y se puede cambiar después (presets 4-6).") + "\n")
	b.WriteString("\n" + s.Muted.Render("En prod nunca se guardan claves: SIDC usa Windows Auth.") + "\n")
	b.WriteString("\n" + s.HelpBar.Render(
		s.Key.Render("[↑↓/Enter]")+s.Desc.Render(" elegir   ")+
			s.Key.Render("[Q]"+s.Desc.Render(" salir"))))
	return s.Box.Render(b.String())
}

// viewServer pide el nombre del servidor. El pedido trae el error típico a la
// vista: acá es donde el operador escribe una instancia con nombre o un puerto y
// conviene que sepa antes que el checklist no puede probarlos por TCP.
func (m Model) viewServer() string {
	s := m.styles
	var b strings.Builder
	b.WriteString(s.AppTitle.Render("SERVIDOR DE PRODUCCIÓN") + "\n\n")
	// Value y no Label: Label tiene Width(16) fijo y parte la pregunta en dos renglones.
	b.WriteString(s.Value.Render("¿Nombre o IP del servidor?") + "\n\n")
	b.WriteString("  " + m.srvInput.View() + "\n")
	if m.srvErr != "" {
		b.WriteString("\n" + s.Error.Render("✖ ") + s.Value.Render(m.srvErr) + "\n")
	}
	b.WriteString("\n" + s.SectionHeader.Render("EJEMPLOS") + "\n")
	for _, e := range [][2]string{
		{"SIDC01", "nombre de la PC donde está SQL Server"},
		{"192.168.1.50", "por IP, si no resuelve por nombre"},
		{`SIDC01\SQLEXPRESS`, "instancia con nombre (SQL Express)"},
		{"SIDC01,1433", "puerto explícito, si no es el 1433"},
	} {
		b.WriteString("  " + s.Value.Render(e[0]) + "  " + s.Muted.Render(e[1]) + "\n")
	}
	b.WriteString("\n" + s.HelpBar.Render(
		s.Key.Render("[Enter]")+s.Desc.Render(" aceptar   ")+
			s.Key.Render("[Esc]")+s.Desc.Render(" volver")))
	return s.Box.Render(b.String())
}

// viewPort pide el puerto TCP de SQL Server.
func (m Model) viewPort() string {
	s := m.styles
	var b strings.Builder
	b.WriteString(s.AppTitle.Render("PUERTO DE SQL SERVER") + "\n\n")
	b.WriteString(s.Value.Render("¿Puerto TCP del motor?") + "\n\n")
	b.WriteString("  " + m.portInput.View() + "\n")
	if m.portErr != "" {
		b.WriteString("\n" + s.Error.Render("✖ ") + s.Value.Render(m.portErr) + "\n")
	}
	b.WriteString("\n" + s.SectionHeader.Render("EJEMPLOS") + "\n")
	for _, e := range [][2]string{
		{"1433", "puerto estándar de SQL Server"},
		{"54721", "puerto alternativo si se configuró estático"},
		{"(vacío)", "dejar vacío para instancias con nombre (SQL Browser)"},
	} {
		b.WriteString("  " + s.Value.Render(e[0]) + "  " + s.Muted.Render(e[1]) + "\n")
	}
	b.WriteString("\n" + s.HelpBar.Render(
		s.Key.Render("[Enter]")+s.Desc.Render(" aceptar   ")+
			s.Key.Render("[Esc]")+s.Desc.Render(" volver")))
	return s.Box.Render(b.String())
}

// viewDatabase pide el nombre de la base de datos para apuntar SIDC a cualquier base.
func (m Model) viewDatabase() string {
	s := m.styles
	var b strings.Builder
	b.WriteString(s.AppTitle.Render("BASE DE DATOS DE SIDC") + "\n\n")
	b.WriteString(s.Value.Render("¿Nombre de la base de datos?") + "\n\n")
	b.WriteString("  " + m.dbInput.View() + "\n")
	if m.dbErr != "" {
		b.WriteString("\n" + s.Error.Render("✖ ") + s.Value.Render(m.dbErr) + "\n")
	}
	b.WriteString("\n" + s.SectionHeader.Render("EJEMPLOS") + "\n")
	for _, e := range [][2]string{
		{"SIDC", "nombre estándar de la base en producción"},
		{"SIDC_PRUEBAS", "base de pruebas o desarrollo"},
		{"SIDC2014", "versión histórica"},
	} {
		b.WriteString("  " + s.Value.Render(e[0]) + "  " + s.Muted.Render(e[1]) + "\n")
	}
	b.WriteString("\n" + s.HelpBar.Render(
		s.Key.Render("[Enter]")+s.Desc.Render(" aceptar   ")+
			s.Key.Render("[Esc]")+s.Desc.Render(" volver")))
	return s.Box.Render(b.String())
}

// viewAuth pregunta el método de autenticación contra SQL Server.
func (m Model) viewAuth() string {
	s := m.styles
	var b strings.Builder
	b.WriteString(s.AppTitle.Render("AUTENTICACIÓN DE SQL SERVER") + "\n")
	b.WriteString(s.Subtitle.Render("¿Cómo se autentica SIDC contra la base de datos?") + "\n\n")
	b.WriteString(s.SectionHeader.Render("MÉTODO DE ACCESO") + "\n")
	for i, opt := range authOptions {
		cur := "  "
		if m.authCursor == i {
			cur = s.Cursor.Render("> ")
		}
		b.WriteString(fmt.Sprintf("%s%s %s\n    %s\n\n",
			cur,
			s.Key.Render("["+opt.key+"]"),
			s.Value.Render(opt.name),
			s.Muted.Render(opt.desc),
		))
	}
	b.WriteString(s.HelpBar.Render(
		s.Key.Render("[↑↓/Enter]") + s.Desc.Render(" elegir   ") +
			s.Key.Render("[Esc]") + s.Desc.Render(" volver")))
	return s.Box.Render(b.String())
}

// viewSQLUser pide el usuario de SQL Server cuando se eligió SQL Auth.
func (m Model) viewSQLUser() string {
	s := m.styles
	var b strings.Builder
	b.WriteString(s.AppTitle.Render("USUARIO DE SQL SERVER") + "\n\n")
	b.WriteString(s.Value.Render("¿Usuario de SQL Server para SIDC?") + "\n\n")
	b.WriteString("  " + m.userInput.View() + "\n")
	if m.userErr != "" {
		b.WriteString("\n" + s.Error.Render("✖ ") + s.Value.Render(m.userErr) + "\n")
	}
	b.WriteString("\n" + s.SectionHeader.Render("EJEMPLOS") + "\n")
	b.WriteString("  " + s.Value.Render("sidc") + "  " + s.Muted.Render("usuario nativo SQL estándar con permisos de db_owner") + "\n")
	b.WriteString("  " + s.Value.Render("sa") + "    " + s.Muted.Render("administrador de la instancia SQL Server") + "\n")
	b.WriteString("\n" + s.HelpBar.Render(
		s.Key.Render("[Enter]")+s.Desc.Render(" aceptar   ")+
			s.Key.Render("[Esc]")+s.Desc.Render(" volver")))
	return s.Box.Render(b.String())
}

// viewAppDir pide la carpeta de SIDC. El pedido dice qué se espera encontrar adentro,
// porque el operador tiene que reconocer la carpeta correcta y no una parecida: si
// apunta a la carpeta equivocada, el checklist mide otra cosa y todo lo que siga sale mal.
func (m Model) viewAppDir() string {
	s := m.styles
	var b strings.Builder
	b.WriteString(s.AppTitle.Render("CARPETA DE SIDC") + "\n\n")
	b.WriteString(s.Value.Render("¿Dónde está la carpeta de SIDC?") + "\n\n")
	b.WriteString("  " + m.dirInput.View() + "\n")
	if m.dirErr != "" {
		b.WriteString("\n" + s.Error.Render("✖ ") + s.Value.Render(m.dirErr) + "\n")
	}
	b.WriteString("\n" + s.SectionHeader.Render("QUÉ TIENE QUE TENER ESA CARPETA") + "\n")
	for _, e := range [][2]string{
		{setup.SIDCExeName, "el ejecutable de SIDC"},
		{"Reportes\\", "unos 57 archivos .rpt"},
		{"Fotos\\Principal.jpg", "la foto de la pantalla principal"},
	} {
		b.WriteString("  " + s.Value.Render(e[0]) + "  " + s.Muted.Render(e[1]) + "\n")
	}
	b.WriteString("\n" + s.SectionHeader.Render("EJEMPLOS") + "\n")
	b.WriteString("  " + s.Value.Render(`C:\SIDC`) + "  " + s.Muted.Render("lo más común") + "\n")
	b.WriteString("  " + s.Value.Render(`C:\SIDC2014`) + "  " + s.Muted.Render("si el nombre delata la versión del motor") + "\n")
	b.WriteString("  " + s.Value.Render(`D:\SIDC`) + "  " + s.Muted.Render("cuando la app vive en otro disco") + "\n")
	b.WriteString("\n" + s.Muted.Render("Se guarda en config.json como app_dir y queda para las próximas corridas.") + "\n")
	b.WriteString("\n" + s.HelpBar.Render(
		s.Key.Render("[Enter]")+s.Desc.Render(" aceptar   ")+
			s.Key.Render("[Esc]")+s.Desc.Render(" volver")))
	return s.Box.Render(b.String())
}

func (m Model) viewConfirmInstall() string {
	s := m.styles
	var b strings.Builder
	b.WriteString(s.AppTitle.Render("AEGIS SETUP — CONFIRMACIÓN DE INSTALACIÓN") + "\n")
	b.WriteString(s.Subtitle.Render("Parámetros detectados y listos para aprovisionar esta PC.") + "\n\n")

	b.WriteString(s.SectionHeader.Render("PARÁMETROS DE CONFIGURACIÓN") + "\n")
	b.WriteString(fmt.Sprintf("  %s %s\n", s.Label.Render("Servidor:      "), s.Value.Render(m.cfg.Server)))
	b.WriteString(fmt.Sprintf("  %s %s\n", s.Label.Render("Base de datos: "), s.Value.Render(m.cfg.Database)))
	b.WriteString(fmt.Sprintf("  %s %s\n", s.Label.Render("Autenticación: "), s.Value.Render(m.cfg.AuthLabel())))
	appDir := m.cfg.AppDir
	if appDir == "" {
		appDir = "(se solicitará al confirmar)"
	}
	b.WriteString(fmt.Sprintf("  %s %s\n\n", s.Label.Render("Carpeta SIDC:  "), s.Value.Render(appDir)))

	b.WriteString(s.SectionHeader.Render("ACCIONES AUTOMÁTICAS A EJECUTAR") + "\n")
	b.WriteString("  " + s.Success.Render("[✔]") + " " + s.Value.Render("DSN SIDC_SQL") + "       " + s.Muted.Render("System DSN 32-bit (HKLM WOW6432Node) con verificación byte a byte") + "\n")
	b.WriteString("  " + s.Success.Render("[✔]") + " " + s.Value.Render("Crystal Reports") + "    " + s.Muted.Render("Copia runtime embebido a SysWOW64 y registra COM de 32-bit") + "\n")
	b.WriteString("  " + s.Success.Render("[✔]") + " " + s.Value.Render("Controles OCX") + "      " + s.Muted.Render("Instala y registra controles ActiveX legacy de VB6") + "\n")
	b.WriteString("  " + s.Success.Render("[✔]") + " " + s.Value.Render("Parche de logos") + "    " + s.Muted.Render("Actualiza ejecutables y reportes (.rpt) desde Fotos/Principal.jpg") + "\n")
	b.WriteString("  " + s.Success.Render("[✔]") + " " + s.Value.Render("Verificación") + "       " + s.Muted.Render("Diagnóstico integral: TCP, SQL, DSN, Crystal y archivos de app") + "\n\n")

	b.WriteString(s.HelpBar.Render(
		s.Key.Render("[Enter]") + s.Desc.Render(" confirmar e instalar   ") +
			s.Key.Render("[C]") + s.Desc.Render(" configuración avanzada   ") +
			s.Key.Render("[Esc]") + s.Desc.Render(" volver al menú")))
	return s.Box.Render(b.String())
}

func (m Model) viewAdvanced() string {
	s := m.styles
	var b strings.Builder
	b.WriteString(s.AppTitle.Render("CONFIGURACIÓN AVANZADA Y AMBIENTES") + "\n")
	b.WriteString(s.Subtitle.Render("Opciones de configuración manual, presets de red y mantenimiento.") + "\n\n")

	for i, it := range advancedItems {
		cur := "  "
		if m.cursor == i {
			cur = s.Cursor.Render("> ")
		}
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", cur, s.Key.Render("["+it.key+"]"), s.Value.Render(it.name), s.Muted.Render("- "+it.desc)))
	}

	b.WriteString("\n" + s.HelpBar.Render(
		s.Key.Render("[↑↓/Enter]")+s.Desc.Render(" elegir   ")+
			s.Key.Render("[Esc/Q]")+s.Desc.Render(" volver al menú principal")))
	return s.Box.Render(b.String())
}

func (m Model) viewAsk() string {
	s := m.styles
	if len(m.askQueue) == 0 {
		// No deberia pasar (la pantalla se activa solo con claves faltantes),
		// pero un index out of range aca corta la instalacion en la cara del
		// operador, a mitad de camino. Mejor mostrar el menú.
		return m.viewMenu()
	}
	name := m.askQueue[0]
	var b strings.Builder
	b.WriteString(s.AppTitle.Render(stepTitle(taskInstall)) + "\n\n")
	b.WriteString(s.SectionHeader.Render(fmt.Sprintf("Falta %s  (%d por pedir)", name, len(m.askQueue))) + "\n\n")
	b.WriteString(s.Label.Render("Clave: ") + m.askInput.View() + "\n")
	if m.askErr != "" {
		b.WriteString("\n" + s.Error.Render("✖ ") + s.Value.Render(m.askErr) + "\n")
	}
	if hint := secretHint(m.cfg, name); hint != "" {
		b.WriteString("\n" + s.Muted.Render(hint) + "\n")
	}
	b.WriteString("\n" + s.Muted.Render("Se usa solo en esta corrida: no se escribe a disco.") + "\n")
	b.WriteString("\n" + s.HelpBar.Render(
		s.Key.Render("[Enter]")+s.Desc.Render(" aceptar   ")+
			s.Key.Render("[Esc]")+s.Desc.Render(" cancelar")))
	return s.Box.Render(b.String())
}

func (m Model) viewWorking() string {
	s := m.styles
	var b strings.Builder
	b.WriteString(s.AppTitle.Render(m.taskName) + "\n\n")
	b.WriteString(fmt.Sprintf("%s %s\n\n", m.spinner.View(), s.Info.Render("Ejecutando... no cierres (puede tardar minutos en RESTORE).")))
	b.WriteString(s.Muted.Render("Claves por env AEGIS_SA_PASSWORD / AEGIS_SQL_PASSWORD, o pedidas al inicio.") + "\n")
	return s.Box.Render(b.String())
}

func (m Model) setupViewport(content string, headerLines, footerLines int) Model {
	w := m.width - 6
	if w < 40 {
		w = 78
	}
	h := m.height - headerLines - footerLines - 4
	if h < 4 {
		h = 15
	}
	m.viewport.SetWidth(w)
	m.viewport.SetHeight(h)
	m.viewport.SetContent(content)
	m.viewport.GotoTop()
	return m
}

func (m Model) refreshViewportContent(content string, headerLines, footerLines int) Model {
	w := m.width - 6
	if w < 40 {
		w = 78
	}
	h := m.height - headerLines - footerLines - 4
	if h < 4 {
		h = 15
	}
	m.viewport.SetWidth(w)
	m.viewport.SetHeight(h)
	m.viewport.SetContent(content)
	return m
}

func (m Model) doneBody() string {
	var b strings.Builder
	for i, l := range m.lines {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("  " + l)
	}
	if m.readyToOpen && m.taskErr == nil && !m.bloqueo {
		if len(m.lines) > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("  SIDC está listo. Presioná [Enter] para abrirlo y cerrar Aegis (A también funciona).")
	}
	return b.String()
}

func (m Model) viewDone() string {
	s := m.styles
	var header strings.Builder
	header.WriteString(s.AppTitle.Render(m.taskName) + "\n\n")
	switch {
	case m.bloqueo:
		// No es una falla: es la puerta haciendo su trabajo. Se dice qué
		// hacer, no "error".
		header.WriteString(s.Warning.Render("! FALTA UN REQUISITO") + "\n\n")
	case m.taskErr != nil:
		header.WriteString(s.Error.Render("✖ ERROR: ") + s.Value.Render(m.taskErr.Error()) + "\n\n")
	default:
		header.WriteString(s.Success.Render("✔ OK") + "\n\n")
	}

	footerText := s.Key.Render("[↑↓/PgUp/PgDn]") + s.Desc.Render(" desplazar   ")
	if m.readyToOpen && m.taskErr == nil && !m.bloqueo {
		footerText += "   " + s.Key.Render("[Enter/A]") + s.Desc.Render(" abrir SIDC y cerrar Aegis") +
			"   " + s.Key.Render("[Esc]") + s.Desc.Render(" volver al menú")
	} else {
		footerText += "   " + s.Key.Render("[Esc/Enter]") + s.Desc.Render(" volver al menú")
	}
	footer := s.HelpBar.Render(footerText)

	if m.height <= 0 {
		return s.Box.Render(header.String() + m.doneBody() + "\n\n" + footer)
	}
	return s.Box.Render(header.String() + m.viewport.View() + "\n\n" + footer)
}

func (m Model) viewHelp() string {
	s := m.styles
	footer := s.HelpBar.Render(
		s.Key.Render("[↑↓/PgUp/PgDn]") + s.Desc.Render(" desplazar   ") +
			s.Key.Render("[Esc/Enter/Q]") + s.Desc.Render(" volver al menú"))

	if m.height <= 0 {
		return s.Box.Render(m.helpHTML + "\n\n" + footer)
	}
	return s.Box.Render(m.viewport.View() + "\n\n" + footer)
}

func renderHelp(s Styles) string {
	md := "# AEGIS ayuda\n\n" +
		"Flujo por PC: 0 Instalación completa (db -> app -> check), o 1, 2 y 3 por separado.\n\n" +
		"- dev docker: SQL Auth con AEGIS_SQL_PASSWORD. El TUI guarda PWD y genera el ejecutable Docker versionado.\n" +
		"- prod local/server: Windows Auth como CONTABILIDAD. Nunca guarda PWD ni pide claves.\n" +
		"- Preset prod server: si el server no viene por --server, el TUI pregunta el nombre.\n" +
		"- Claves por entorno o pedidas al inicio, jamás en config.json.\n\n" +
		"Primera vez:\n\n" +
		"- Sin config.json el TUI pregunta en qué PC estamos: Pruebas (Docker), Producción en esta PC o Producción en un servidor.\n" +
		"- Después pregunta dónde está la carpeta de SIDC (app_dir): en una PC limpia ese dato no se puede adivinar.\n" +
		"- Elige una vez y queda guardada; los presets 4-6 la cambian después.\n" +
		"- Cambiar de perfil recalcula el checklist: los requisitos de Docker no aplican en la PC de producción.\n\n" +
		"Drops manuales (tu los pones):\n\n" +
		"- El .bak de SIDC (SQL 2014) en la carpeta de backups; el checklist dice la ruta exacta de esta máquina.\n" +
		"- assets/oldpc/NOTAS.txt -> DSN, collation, usuarios app.\n\n" +
		"Ya no son drops manuales:\n\n" +
		"- El runtime de Crystal (43 archivos, 24 MB) viaja dentro del EXE: Setup App lo copia a SysWOW64 y registra los 4 componentes COM.\n" +
		"- Los 11 controles OCX de VB6 y sus 11 dependencias también viajan dentro del EXE.\n\n" +
		"Permisos de administrador:\n\n" +
		"- [E] pide permisos y relanza Aegis elevado; check, checklist y dashboard nunca los piden.\n" +
		"- Desde la consola, setup-db y setup-app se elevan solos (el padre espera y devuelve el mismo código).\n" +
		"- AEGIS_NO_ELEVAR=1 corta el reintento en el proceso ya elevado.\n\n" +
		"Teclas: 0-7 ejecutan, flechas+Enter eligen, C checklist, E permisos, H ayuda, Q salir.\n\n" +
		"Checklist y desbloqueo:\n\n" +
		"- C muestra los requisitos de la PC (admin, SysWOW64, Docker, motor, driver ODBC, .bak, base, DSN, Crystal, OCX, archivos de SIDC).\n" +
		"- Cada uno dice para qué sirve, qué hacer si falta y qué opción del menú traba.\n" +
		"- Las opciones trabadas se ven en gris con el requisito que falta, y se desbloquean solas al resolverlo.\n" +
		"- Check y los presets nunca se bloquean: son el diagnóstico y la configuración.\n\n" +
		"- Cuando Instalación completa termina bien, la pantalla final ofrece [Enter] Abrir SIDC y cerrar Aegis ([A] también funciona).\n"
	out, err := glamour.Render(md, "dark")
	if err != nil {
		return md
	}
	return out
}
