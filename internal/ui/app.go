// © Antony Monge López — Costa Rica — Céd. 604700548
package ui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"aegis-setup/internal/check"
	"aegis-setup/internal/config"
	"aegis-setup/internal/precheck"
	"aegis-setup/internal/setup"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
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
)

type taskKind int

const (
	taskNone taskKind = iota
	taskSetupDB
	taskSetupApp
	taskCheck
	taskInstall
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
	// secrets son las claves pedidas en el prompt, solo en memoria y solo para
	// esta corrida. No se escriben a disco: prod nunca guarda claves.
	secrets  map[string]string
	askQueue []string
	askInput textinput.Model
	askErr   string
	// presetServer es el server del preset "prod server" vía flag --server.
	// Vacío = comportamiento por defecto (CONTABILIDAD si la config actual
	// apunta a localhost, o la config que ya hubiera).
	presetServer string
	// checks es el último checklist corrido. Vacío = todavía no corrió y por
	// eso no se bloquea nada: una sonda que no respondió no puede encerrar al
	// operador.
	checks      []precheck.Requisito
	verificando bool
	// bloqueo distingue "el paso falló" de "no te dejo empezar": el operador
	// reacciona distinto a cada uno y la pantalla no debería decir "error"
	// cuando la puerta hizo su trabajo.
	bloqueo bool
	// primeraVez es "no hay config.json todavía". Mientras sea true no se mide
	// nada: sin perfil elegido, el checklist mide el ambiente por defecto, que no
	// es el de esta PC.
	primeraVez bool
	// srvInput y srvErr son el prompt del nombre del servidor en prod server.
	// Campo propio y no el de las claves: uno se muestra y el otro no, y eso
	// tiene que ser propiedad del campo, no de por dónde pasó el flujo.
	srvInput textinput.Model
	srvErr   string
}

var menuItems = []menuEntry{
	{"0", "Instalación completa", "setup-db -> setup-app -> check, de un tirón", actInstall},
	{"1", "Setup DB", "restaura .bak -> SIDC (compat, logins)", actSetupDB},
	{"2", "Setup App", "DSN 32-bit + OCX + verifica app (+parche dev)", actSetupApp},
	{"3", "Check", "verifica que App y DB se hablan", actCheck},
	{"4", "Preset dev", "config dev docker localhost,14333", actPresetDev},
	{"5", "Preset prod local", "config prod misma PC, Windows Auth", actPresetProdLocal},
	{"6", "Preset prod server", "config prod servidor xxxx, Windows Auth", actPresetProdServer},
}

// perfiles es la primera pregunta en una PC sin config: en qué PC estamos. Son las
// MISMAS acciones que los presets del menú (4/5/6) a propósito. Es la misma decisión
// tomada en dos momentos distintos, y con listas separadas el día que cambie una
// regla de ambiente el arranque y el menú dejarían configs distintas.
var perfiles = []menuEntry{
	{"1", "Pruebas", "SQL Server 2019 en Docker, en esta misma PC", actPresetDev},
	{"2", "Producción en esta PC", "SIDC y SQL Server locales, Windows Auth", actPresetProdLocal},
	{"3", "Producción en un servidor", "SQL Server en otra PC de la red", actPresetProdServer},
}

// NewModel crea el modelo TUI con la config cargada.
func NewModel(cfg config.Config, cfgPath string) Model {
	styles := DefaultStyles()
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styles.Spinner
	return Model{cfg: cfg, cfgPath: cfgPath, styles: styles, spinner: sp, secrets: map[string]string{}}
}

// SetPresetServer fija el server del preset "prod server" (flag --server del
// CLI). Devuelve una copia con el valor puesto; no muta el original.
func (m Model) SetPresetServer(s string) Model {
	m.presetServer = s
	return m
}

// SetPrimeraVez marca que no hay config todavía, así que lo primero es preguntar en
// qué PC estamos. Lo decide el CLI, que es quien sabe si el archivo existe: el
// modelo no toca el disco.
func (m Model) SetPrimeraVez(v bool) Model {
	m.primeraVez = v
	if v {
		m.screen = screenPerfil
		m.cursor = 0
	}
	return m
}

type checksMsg struct{ rs []precheck.Requisito }

func (m Model) Init() tea.Cmd {
	// Sin perfil elegido no hay nada que medir: el checklist del ambiente por
	// defecto diría "Docker en marcha" en la PC de FEMUCARIBE, que no usa Docker.
	if m.primeraVez {
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
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case taskFinishedMsg:
		m.screen = screenDone
		m.lines = msg.lines
		m.taskErr = msg.err
		m.bloqueo = false
		// Se recalcula el checklist: al terminar un paso se desbloquea el
		// siguiente, y el operador lo tiene que ver sin pedirlo.
		m.verificando = true
		return m, m.runChecks()
	case checksMsg:
		m.checks = msg.rs
		m.verificando = false
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
		case screenPerfil:
			return m.updatePerfil(msg)
		case screenServer:
			return m.updateServer(msg)
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
				return m, nil
			case "c", "C":
				m.screen = screenChecklist
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
			if msg.String() == "esc" || msg.String() == "enter" || msg.String() == "q" || msg.String() == "c" {
				m.lines = nil
				m.taskErr = nil
				m.bloqueo = false
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
			return m, nil
		case screenWorking:
			return m, nil
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

// elegirPerfil aplica el perfil elegido. Prod server primero pregunta el nombre: un
// nombre inventado manda al operador a un "motor no alcanzable" que no describe su
// problema, y encima queda escrito en el config.
func (m Model) elegirPerfil(p menuEntry) (tea.Model, tea.Cmd) {
	if p.action == actPresetProdServer && m.presetServer == "" {
		m.srvInput = newServerInput(m.sugerenciaServer())
		m.srvErr = ""
		m.screen = screenServer
		return m, nil
	}
	// presetServer es el flag --server: cuando viene, el nombre ya está dicho y no
	// hay nada que preguntar (camino no interactivo).
	return m.aplicarPerfil(p.action, m.presetServer)
}

// sugerenciaServer es lo único que podemos ofrecer como ejemplo. No se precarga en
// el campo a propósito: un valor precargado se guarda tal cual sin que el operador
// lo lea, que es exactamente cómo se cuela un CONTABILIDAD equivocado.
func (m Model) sugerenciaServer() string {
	if m.presetServer != "" {
		return m.presetServer
	}
	s := m.cfg.Server
	if s == "" || strings.HasPrefix(s, "localhost") || strings.HasPrefix(s, ".") || s == "127.0.0.1" {
		return "CONTABILIDAD"
	}
	return s
}

func newServerInput(sugerencia string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = sugerencia
	ti.Prompt = "> "
	// Width explícito: un textinput sin ancho dibuja solo el cursor y el operador
	// no ve lo que escribe ni el ejemplo de la sugerencia.
	ti.SetWidth(40)
	ti.CharLimit = 128
	ti.Focus()
	return ti
}

func (m Model) updateServer(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
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
			m.srvErr = "Poné el nombre del servidor (ej. CONTABILIDAD, SIDC01 o CONTABILIDAD\\SQLEXPRESS)."
			return m, nil
		}
		return m.aplicarPerfil(actPresetProdServer, server)
	}
	var cmd tea.Cmd
	m.srvInput, cmd = m.srvInput.Update(msg)
	return m, cmd
}

// aplicarPerfil guarda la config y RECALCULA el checklist. Lo segundo no es un
// adorno: el checklist viejo describe la máquina según el ambiente anterior, así que
// conservarlo dejaría al operador trabado por requisitos del ambiente que acaba de
// abandonar (Docker, por ejemplo, en una PC de producción).
func (m Model) aplicarPerfil(a action, server string) (tea.Model, tea.Cmd) {
	cfg, err := presetConfig(m.cfg, a, server)
	if err != nil {
		return m.errorDePerfil(err)
	}
	if err := cfg.Save(m.cfgPath); err != nil {
		return m.errorDePerfil(err)
	}
	m.cfg = cfg
	m.primeraVez = false
	m.srvErr = ""
	m.bloqueo = false
	m.taskErr = nil
	m.task = taskNone
	m.taskName = "PERFIL"
	m.screen = screenDone
	m.cursor = 0
	m.lines = []string{
		fmt.Sprintf("config guardada en %s", m.cfgPath),
		fmt.Sprintf("env=%s db_mode=%s server=%s auth=%s", cfg.Env, cfg.DbMode, cfg.Server, authLabel(cfg)),
		"",
		"Ya está midiendo esta config: mirá el checklist con [C].",
	}
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
func (m Model) updateAsk(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
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
			return m.startTask(taskInstall, "INSTALACIÓN COMPLETA")
		}
		m.askInput = newSecretInput()
		return m, nil
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
	ti.Focus()
	return ti
}

// beginAsk pide las claves que faltan y despues corre la instalacion completa.
func (m Model) beginAsk(need []string) (tea.Model, tea.Cmd) {
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
		return m, nil
	}
	switch a {
	case actInstall:
		if need := secretNeeds(m.cfg, m.hasSecret); len(need) > 0 {
			return m.beginAsk(need)
		}
		return m.startTask(taskInstall, stepTitle(taskInstall))
	case actSetupDB:
		return m.startTask(taskSetupDB, stepTitle(taskSetupDB))
	case actSetupApp:
		return m.startTask(taskSetupApp, stepTitle(taskSetupApp))
	case actCheck:
		return m.startTask(taskCheck, stepTitle(taskCheck))
	case actPresetDev, actPresetProdLocal, actPresetProdServer:
		// Prod server sin --server pregunta el nombre en vez de escribir
		// CONTABILIDAD: un servidor adivinado se ve igual de válido que uno real
		// hasta que falla la conexión.
		if a == actPresetProdServer && m.presetServer == "" {
			m.srvInput = newServerInput(m.sugerenciaServer())
			m.srvErr = ""
			m.screen = screenServer
			return m, nil
		}
		return m.aplicarPerfil(a, m.presetServer)
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
		if err := setup.WriteDSN(cfg, appPass, savePWD, emit); err != nil {
			return err
		}
		for _, f := range setup.InstallOCX(cfg.LegacyDir, emit) {
			emit("OCX PENDIENTE: " + f)
		}
		if !cfg.UseWinAuth {
			if appPass == "" {
				return fmt.Errorf("falta %s para el parche _DOCKER", envAppPassword)
			}
			return setup.PatchDockerExe(cfg.AppDir, cfg.SQLUser, appPass, emit)
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
	}
	return nil
}

// presetConfig calcula la config que deja un preset, sin tocar disco. El
// presetServer viene del flag --server y solo aplica al preset "prod server":
// deja elegir el servidor sin editar config.json a mano. Vacío = CONTABILIDAD
// si la config actual todavía apunta a localhost (dev), o la que ya hubiera.
func presetConfig(cfg config.Config, a action, presetServer string) (config.Config, error) {
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
	}
	if err := cfg.Validate(); err != nil {
		return config.Config{}, err
	}
	return cfg, nil
}

func (m Model) View() tea.View {
	var content string
	switch m.screen {
	case screenWorking:
		content = m.viewWorking()
	case screenDone:
		content = m.viewDone()
	case screenHelp:
		content = m.helpHTML
	case screenChecklist:
		content = m.viewChecklist()
	case screenAsk:
		content = m.viewAsk()
	case screenPerfil:
		content = m.viewPerfil()
	case screenServer:
		content = m.viewServer()
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
	b.WriteString(s.AppTitle.Render("AEGIS SETUP") + "\n")
	b.WriteString(s.Subtitle.Render(fmt.Sprintf("env=%s db_mode=%s server=%s db=%s", m.cfg.Env, m.cfg.DbMode, m.cfg.Server, m.cfg.Database)) + "\n")
	b.WriteString(m.checklistLine() + "\n\n")
	b.WriteString(s.SectionHeader.Render("FLUJO (se ejecuta solo, sin chorrear comandos)") + "\n")
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
	b.WriteString("\n" + s.HelpBar.Render(
		s.Key.Render("[↑↓/Enter]")+s.Desc.Render(" elegir   ")+
			s.Key.Render("[C]")+s.Desc.Render(" checklist   ")+
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

// viewChecklist es el detalle: qué falta, por qué importa, qué hacer y qué
// opción del menú queda trabada. Es el plan de trabajo de la PC.
func (m Model) viewChecklist() string {
	s := m.styles
	var b strings.Builder
	b.WriteString(s.AppTitle.Render("CHECKLIST DE LA MAQUINA") + "\n")
	b.WriteString(s.Subtitle.Render(fmt.Sprintf("env=%s db_mode=%s server=%s db=%s", m.cfg.Env, m.cfg.DbMode, m.cfg.Server, m.cfg.Database)) + "\n\n")

	if m.verificando || len(m.checks) == 0 {
		b.WriteString(fmt.Sprintf("%s %s\n", m.spinner.View(), s.Info.Render("Verificando la máquina... consulta el motor SQL, puede tardar unos segundos.")))
		b.WriteString("\n" + s.HelpBar.Render(s.Key.Render("[Esc]")+s.Desc.Render(" volver al menú")))
		return s.Box.Render(b.String())
	}

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

	// Se distinguen los dos casos porque se ven iguales en pantalla y no lo son:
	// en una PC limpia la base y el DSN están en rojo y no traban nada.
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
	b.WriteString(s.HelpBar.Render(
		s.Key.Render("[Esc/Enter]") + s.Desc.Render(" volver al menú   ") +
			s.Key.Render("[C]") + s.Desc.Render(" recalcular")))
	return s.Box.Render(b.String())
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

func (m Model) viewDone() string {
	s := m.styles
	var b strings.Builder
	b.WriteString(s.AppTitle.Render(m.taskName) + "\n\n")
	switch {
	case m.bloqueo:
		// No es una falla: es la puerta haciendo su trabajo. Se dice qué
		// hacer, no "error".
		b.WriteString(s.Warning.Render("! FALTA UN REQUISITO") + "\n\n")
	case m.taskErr != nil:
		b.WriteString(s.Error.Render("✖ ERROR: ") + s.Value.Render(m.taskErr.Error()) + "\n\n")
	default:
		b.WriteString(s.Success.Render("✔ OK") + "\n\n")
	}
	for _, l := range m.lines {
		b.WriteString("  " + l + "\n")
	}
	b.WriteString("\n" + s.HelpBar.Render(s.Key.Render("[Esc/Enter]")+s.Desc.Render(" volver al menú")))
	return s.Box.Render(b.String())
}

func renderHelp(s Styles) string {
	md := "# AEGIS ayuda\n\n" +
		"Flujo por PC: 0 Instalación completa (db -> app -> check), o 1, 2 y 3 por separado.\n\n" +
		"- dev docker: SQL Auth con AEGIS_SQL_PASSWORD. El TUI guarda PWD y genera _DOCKER.exe solo.\n" +
		"- prod local/server: Windows Auth como CONTABILIDAD. Nunca guarda PWD ni pide claves.\n" +
		"- Preset prod server: si el server no viene por --server, el TUI pregunta el nombre.\n" +
		"- Claves por entorno o pedidas al inicio, jamás en config.json.\n\n" +
		"Primera vez:\n\n" +
		"- Sin config.json el TUI pregunta en qué PC estamos: Pruebas (Docker), Producción en esta PC o Producción en un servidor.\n" +
		"- Elige una vez y queda guardada; los presets 4-6 la cambian después.\n" +
		"- Cambiar de perfil recalcula el checklist: los requisitos de Docker no aplican en la PC de producción.\n\n" +
		"Drops manuales (tu los pones):\n\n" +
		"- assets/backups/sqlserver2014/ -> el .bak de SIDC (SQL 2014).\n" +
		"- assets/legacy/ocx/ -> los 11 OCX de la PC vieja.\n" +
		"- assets/oldpc/NOTAS.txt -> DSN, collation, usuarios app.\n\n" +
		"Teclas: 0-6 ejecutan, flechas+Enter eligen, C checklist, H ayuda, Q salir.\n\n" +
		"Checklist y desbloqueo:\n\n" +
		"- C muestra los requisitos de la PC (admin, SysWOW64, Docker, motor, driver ODBC, .bak, base, DSN, Crystal, OCX, archivos de SIDC).\n" +
		"- Cada uno dice para qué sirve, qué hacer si falta y qué opción del menú traba.\n" +
		"- Las opciones trabadas se ven en gris con el requisito que falta, y se desbloquean solas al resolverlo.\n" +
		"- Check y los presets nunca se bloquean: son el diagnóstico y la configuración.\n\n"
	out, err := glamour.Render(md, "dark")
	if err != nil {
		return md
	}
	return out
}
