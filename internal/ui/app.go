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
	"aegis-setup/internal/setup"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
)

type screen int

const (
	screenMenu screen = iota
	screenWorking
	screenDone
	screenHelp
)

type taskKind int

const (
	taskNone taskKind = iota
	taskSetupDB
	taskSetupApp
	taskCheck
)

type taskFinishedMsg struct {
	kind  taskKind
	lines []string
	err   error
}

// Model es el TUI de Aegis: menú que EJECUTA el flujo, no solo lo dice.
type Model struct {
	cfg     config.Config
	cfgPath string
	styles  Styles
	screen  screen
	cursor  int
	spinner spinner.Model
	task    taskKind
	taskName string
	lines   []string
	taskErr error
	width   int
	height  int
	helpHTML string
}

var menuItems = []struct {
	key  string
	name string
	desc string
}{
	{"1", "Setup DB", "restaura .bak -> SIDC (compat, logins)"},
	{"2", "Setup App", "DSN 32-bit + OCX + verifica app (+parche dev)"},
	{"3", "Check", "verifica que App y DB se hablan"},
	{"4", "Preset dev", "config dev docker localhost,14333"},
	{"5", "Preset prod local", "config prod misma PC, Windows Auth"},
	{"6", "Preset prod server", "config prod servidor xxxx, Windows Auth"},
}

// NewModel crea el modelo TUI con la config cargada.
func NewModel(cfg config.Config, cfgPath string) Model {
	styles := DefaultStyles()
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styles.Spinner
	return Model{cfg: cfg, cfgPath: cfgPath, styles: styles, spinner: sp}
}

func (m Model) Init() tea.Cmd { return nil }

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
				return m.startItem(m.cursor)
			case "q", "Q":
				return m, tea.Quit
			case "h", "H", "?":
				m.screen = screenHelp
				m.helpHTML = renderHelp(m.styles)
				return m, nil
			default:
				for i, it := range menuItems {
					if msg.String() == it.key {
						return m.startItem(i)
					}
				}
			}
			return m, nil
		case screenDone, screenHelp:
			if msg.String() == "esc" || msg.String() == "enter" || msg.String() == "q" {
				m.screen = screenMenu
				m.lines = nil
				m.taskErr = nil
				return m, nil
			}
			return m, nil
		case screenWorking:
			return m, nil
		}
	}
	return m, nil
}

func (m Model) startItem(i int) (tea.Model, tea.Cmd) {
	switch i {
	case 0:
		return m.startTask(taskSetupDB, "SETUP DB")
	case 1:
		return m.startTask(taskSetupApp, "SETUP APP")
	case 2:
		return m.startTask(taskCheck, "CHECK")
	case 3, 4, 5:
		lines, err := m.applyPreset(i)
		m.screen = screenDone
		m.lines = lines
		m.taskErr = err
		m.task = taskNone
		m.taskName = "PRESET"
		return m, nil
	}
	return m, nil
}

func (m Model) startTask(k taskKind, name string) (tea.Model, tea.Cmd) {
	m.screen = screenWorking
	m.task = k
	m.taskName = name
	m.lines = nil
	m.taskErr = nil
	cfg := m.cfg
	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		var lines []string
		emit := func(s string) { lines = append(lines, s) }
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		var err error
		switch k {
		case taskSetupDB:
			bak, ferr := setup.FindNewestBak(cfg.BackupDir)
			if ferr != nil {
				err = ferr
				break
			}
			emit("BAK: " + bak)
			sa := os.Getenv("AEGIS_SA_PASSWORD")
			ap := os.Getenv("AEGIS_SQL_PASSWORD")
			err = setup.SetupDB(ctx, cfg, bak, sa, ap, emit)
		case taskSetupApp:
			ap := os.Getenv("AEGIS_SQL_PASSWORD")
			for _, miss := range setup.CheckAppFiles(cfg.AppDir) {
				emit("FALTA: " + miss)
			}
			savePWD := !cfg.UseWinAuth // solo dev/docker guarda PWD
			if werr := setup.WriteDSN(cfg, ap, savePWD, emit); werr != nil {
				err = werr
				break
			}
			for _, f := range setup.InstallOCX(cfg.LegacyDir, emit) {
				emit("OCX PENDIENTE: " + f)
			}
			if !cfg.UseWinAuth {
				if ap == "" {
					err = fmt.Errorf("falta AEGIS_SQL_PASSWORD para el parche _DOCKER")
					break
				}
				err = setup.PatchDockerExe(cfg.AppDir, cfg.SQLUser, ap, emit)
			}
		case taskCheck:
			ap := os.Getenv("AEGIS_SQL_PASSWORD")
			for _, r := range check.Run(ctx, cfg, ap) {
				mark := "OK  "
				if !r.OK {
					mark = "FAIL"
				}
				emit(fmt.Sprintf("%s %-22s %s", mark, r.Name, r.Info))
			}
		}
		return taskFinishedMsg{kind: k, lines: lines, err: err}
	})
}

func (m Model) applyPreset(i int) ([]string, error) {
	cfg := m.cfg
	switch i {
	case 3:
		cfg.Env, cfg.DbMode, cfg.Server = "dev", config.DbDocker, "localhost,14333"
		cfg.UseWinAuth, cfg.Driver = false, "ODBC Driver 17 for SQL Server"
	case 4:
		cfg.Env, cfg.DbMode, cfg.Server = "prod", config.DbLocal, "localhost"
		cfg.UseWinAuth, cfg.Driver = true, "SQL Server"
	case 5:
		cfg.Env, cfg.DbMode = "prod", config.DbServer
		if cfg.Server == "localhost,14333" || cfg.Server == "localhost" {
			cfg.Server = "CONTABILIDAD"
		}
		cfg.UseWinAuth, cfg.Driver = true, "SQL Server"
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := cfg.Save(m.cfgPath); err != nil {
		return nil, err
	}
	m.cfg = cfg
	return []string{fmt.Sprintf("config guardada: env=%s db_mode=%s server=%s", cfg.Env, cfg.DbMode, cfg.Server)}, nil
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
	b.WriteString(s.Subtitle.Render(fmt.Sprintf("env=%s db_mode=%s server=%s db=%s", m.cfg.Env, m.cfg.DbMode, m.cfg.Server, m.cfg.Database)) + "\n\n")
	b.WriteString(s.SectionHeader.Render("FLUJO (se ejecuta solo, sin chorrear comandos)") + "\n")
	for i, it := range menuItems {
		cur := "  "
		if m.cursor == i {
			cur = s.Cursor.Render("> ")
		}
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", cur, s.Key.Render("["+it.key+"]"), s.Value.Render(it.name), s.Muted.Render("- "+it.desc)))
	}
	b.WriteString("\n" + s.HelpBar.Render(
		s.Key.Render("[↑↓/Enter]")+s.Desc.Render(" elegir   ")+
			s.Key.Render("[H]")+s.Desc.Render(" ayuda   ")+
			s.Key.Render("[Q]")+s.Desc.Render(" salir")))
	return s.Box.Render(b.String())
}

func (m Model) viewWorking() string {
	s := m.styles
	var b strings.Builder
	b.WriteString(s.AppTitle.Render(m.taskName) + "\n\n")
	b.WriteString(fmt.Sprintf("%s %s\n\n", m.spinner.View(), s.Info.Render("Ejecutando... no cierres (puede tardar minutos en RESTORE).")))
	b.WriteString(s.Muted.Render("Claves por env AEGIS_SA_PASSWORD / AEGIS_SQL_PASSWORD.") + "\n")
	return s.Box.Render(b.String())
}

func (m Model) viewDone() string {
	s := m.styles
	var b strings.Builder
	b.WriteString(s.AppTitle.Render(m.taskName) + "\n\n")
	if m.taskErr != nil {
		b.WriteString(s.Error.Render("✖ ERROR: ") + s.Value.Render(m.taskErr.Error()) + "\n\n")
	} else {
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
		"Flujo por PC: 1 Setup DB -> 2 Setup App -> 3 Check.\n\n" +
		"- dev docker: SQL Auth con AEGIS_SQL_PASSWORD. El TUI guarda PWD y genera _DOCKER.exe solo.\n" +
		"- prod local/server: Windows Auth como CONTABILIDAD. Nunca guarda PWD.\n" +
		"- Claves por entorno, jamas en config.json.\n\n" +
		"Drops manuales (tu los pones):\n\n" +
		"- assets/backups/sqlserver2014/ -> el .bak de SIDC (SQL 2014).\n" +
		"- assets/legacy/ocx/ -> los 11 OCX de la PC vieja.\n" +
		"- assets/oldpc/NOTAS.txt -> DSN, collation, usuarios app.\n\n" +
		"Teclas: 1-6 ejecutan, flechas+Enter eligen, H ayuda, Q salir.\n"
	out, err := glamour.Render(md, "dark")
	if err != nil {
		return md
	}
	return out
}
