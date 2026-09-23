// © Antony Monge López — Costa Rica — Céd. 604700548
package ui

import (
	"errors"
	"strings"
	"testing"

	"aegis-setup/internal/config"
	"aegis-setup/internal/precheck"
)

// checklistFalso corre el checklist de verdad con sondas de mentira. Se usa el
// Run real a propósito: así estos tests también verifican que los IDs que pide
// cada puerta son IDs que el precheck de verdad reporta.
func checklistFalso(modo config.DbMode, faltan ...precheck.ID) []precheck.Requisito {
	cfg := config.Default()
	cfg.DbMode = modo
	s := precheck.Sondas{
		Admin:    func() (bool, string) { return true, "ok" },
		Maquina:  func() (bool, string) { return true, "ok" },
		Docker:   func() (bool, string) { return true, "ok" },
		Motor:    motorUI(precheck.EstadoOK, "ok"),
		ODBC:     func(string) (bool, string) { return true, "ok" },
		Base:     func(config.Config, string) (bool, string) { return true, "ok" },
		DSN:      func(string) (bool, string) { return true, "ok" },
		Crystal:  func() []string { return nil },
		OCX:      func(config.Config) precheck.EstadoOCX { return precheck.EstadoOCX{} },
		Backup:   func(config.Config) (string, error) { return "SIDC.bak", nil },
		AppFiles: func(string) []string { return nil },
	}
	roto := map[precheck.ID]bool{}
	for _, id := range faltan {
		roto[id] = true
	}
	if roto[precheck.ReqAdmin] {
		s.Admin = func() (bool, string) { return false, "sesión sin elevar" }
	}
	if roto[precheck.ReqMaquina] {
		s.Maquina = func() (bool, string) { return false, "no se encontró SysWOW64" }
	}
	if roto[precheck.ReqDocker] {
		s.Docker = func() (bool, string) { return false, "docker no responde" }
	}
	if roto[precheck.ReqMotor] {
		s.Motor = motorUI(precheck.EstadoFalta, "connection refused")
	}
	if roto[precheck.ReqODBC] {
		s.ODBC = func(string) (bool, string) { return false, "no instalado" }
	}
	if roto[precheck.ReqBase] {
		s.Base = func(config.Config, string) (bool, string) { return false, "login failed" }
	}
	if roto[precheck.ReqDSN] {
		s.DSN = func(string) (bool, string) { return false, "no existe el DSN" }
	}
	if roto[precheck.ReqCrystal] {
		s.Crystal = func() []string { return []string{"crpe32.dll"} }
	}
	if roto[precheck.ReqBackup] {
		s.Backup = func(config.Config) (string, error) { return "", errors.New("no hay .bak en ninguna carpeta") }
	}
	if roto[precheck.ReqApp] {
		s.AppFiles = func(string) []string { return []string{"exe: falta"} }
	}
	return precheck.Run(cfg, "", s)
}

func tieneID(ids []precheck.ID, id precheck.ID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// El operador tiene que poder diagnosticar justo cuando todo está roto. Si el
// check se bloqueara, perdería la herramienta que le dice qué arreglar.
func TestGateNoTrancaElDiagnosticoNiLosPresets(t *testing.T) {
	todos := []precheck.ID{
		precheck.ReqAdmin, precheck.ReqMaquina, precheck.ReqDocker, precheck.ReqMotor,
		precheck.ReqODBC, precheck.ReqBackup, precheck.ReqBase, precheck.ReqDSN,
		precheck.ReqCrystal, precheck.ReqApp,
	}
	roto := checklistFalso(config.DbDocker, todos...)
	if len(precheck.Faltantes(roto)) < 8 {
		t.Fatalf("el escenario de prueba no rompió lo suficiente: %v", roto)
	}

	for _, a := range []action{actCheck, actPresetDev, actPresetProdLocal, actPresetProdServer} {
		if g := gate(a); len(g) != 0 {
			t.Errorf("%v tiene puerta %v y no debería tener ninguna", a, g)
		}
		if b := bloqueosDe(a, roto); len(b) != 0 {
			t.Errorf("%v quedó trabada con todo roto: %v", a, b)
		}
	}
}

// Una sonda que todavía no corrió no puede encerrar al operador: si el checklist
// está vacío, se deja pasar y el error sale del paso mismo.
func TestSinChecklistNoSeTrabaNada(t *testing.T) {
	for _, it := range menuItems {
		if b := bloqueosDe(it.action, nil); len(b) != 0 {
			t.Errorf("%q se trabó sin checklist: %v", it.name, b)
		}
	}
}

// Los avisos son cosas que se resuelven solas en un paso posterior (los OCX): no
// pueden trabar nada.
func TestAvisoNoTraba(t *testing.T) {
	rs := []precheck.Requisito{
		{ID: precheck.ReqAdmin, Titulo: "Permisos", Estado: precheck.EstadoAviso},
		{ID: precheck.ReqBackup, Titulo: "Respaldo", Estado: precheck.EstadoOK},
	}
	if b := bloqueosDe(actSetupDB, rs); len(b) != 0 {
		t.Errorf("un aviso trabó Setup DB: %v", b)
	}
}

func TestFaltaTrabaYQuedaEnLaLista(t *testing.T) {
	rs := checklistFalso(config.DbDocker, precheck.ReqBackup, precheck.ReqMotor)
	b := bloqueosDe(actSetupDB, rs)
	if len(b) != 2 {
		t.Fatalf("bloqueos = %d, quiero 2 (%v)", len(b), b)
	}
	if !tieneID([]precheck.ID{b[0].ID, b[1].ID}, precheck.ReqBackup) {
		t.Errorf("el respaldo falta y no quedó en la lista: %v", b)
	}
	// El orden de los bloqueos sigue el de la puerta, que es el de resolución.
	if b[0].ID != precheck.ReqMotor {
		t.Errorf("el motor va antes que el respaldo, salió %s primero", b[0].ID)
	}
}

// La instalación de terminal (actInstall) exige todo lo que exige Setup App.
func TestLaPuertaDeLaInstalacionCubreSetupApp(t *testing.T) {
	grande := gate(actInstall)
	for _, id := range gate(actSetupApp) {
		if !tieneID(grande, id) {
			t.Errorf("la instalación de terminal no exige %q, que sí exige %q", id, nombreDeAccion(actSetupApp))
		}
	}
}

// Un ID mal escrito en una puerta nunca bloquearía nada: el bloqueo se apagaría
// solo y nadie se enteraría hasta que la instalación fallara a mitad de camino.
func TestTodoIDDePuertaExisteEnElChecklist(t *testing.T) {
	vistos := map[precheck.ID]bool{}
	for _, r := range checklistFalso(config.DbDocker) {
		vistos[r.ID] = true
	}
	for _, it := range menuItems {
		for _, id := range gate(it.action) {
			if !vistos[id] {
				t.Errorf("%q pide %q y el checklist nunca lo reporta", it.name, id)
			}
		}
	}
}

// La base, el DSN y el backup no traban ninguna acción: la base y el DSN son el
// resultado del flujo, y los backups corresponden a backup-agent.
func TestBaseYDSNNoTrabasNingunaAccion(t *testing.T) {
	for _, id := range []precheck.ID{precheck.ReqBase, precheck.ReqDSN, precheck.ReqBackup} {
		if deps := accionesQueDependenDe(id); len(deps) != 0 {
			t.Errorf("%q traba %v: son el resultado del flujo, no un requisito previo", id, deps)
		}
	}
}

func TestDependenciasDeUnRequisito(t *testing.T) {
	// Crystal lo instala Setup App: exigirlo antes sería dejar al operador sin
	// ninguna acción que lo resuelva.
	if deps := strings.Join(accionesQueDependenDe(precheck.ReqCrystal), " | "); strings.Contains(deps, "Setup App") {
		t.Errorf("Crystal no puede trabar Setup App, y traba: %q", deps)
	}

	if deps := accionesQueDependenDe(precheck.ReqAdmin); len(deps) < 2 {
		t.Errorf("sin permisos no se puede nada, y solo traba: %v", deps)
	}
}

func TestResumenDelChecklist(t *testing.T) {
	if got := resumenChecklist(nil, false); !strings.Contains(got, "verificando") {
		t.Errorf("sin datos debería decir que está verificando: %q", got)
	}
	if got := resumenChecklist(nil, true); !strings.Contains(got, "verificando") {
		t.Errorf("mientras corre debería decir que está verificando: %q", got)
	}
	if got := resumenChecklist(checklistFalso(config.DbDocker), false); !strings.Contains(got, "ninguno bloquea") {
		t.Errorf("con todo OK: %q", got)
	}

	uno := resumenChecklist(checklistFalso(config.DbDocker, precheck.ReqAdmin), false)
	if !strings.Contains(uno, "1 bloquea") || strings.Contains(uno, "1 bloquean") {
		t.Errorf("singular mal formado: %q", uno)
	}
	if !strings.Contains(uno, "[C]") {
		t.Errorf("no dice cómo ver el detalle: %q", uno)
	}

	muchos := resumenChecklist(checklistFalso(config.DbDocker, precheck.ReqAdmin, precheck.ReqApp), false)
	if !strings.Contains(muchos, "2 bloquean") {
		t.Errorf("plural mal formado: %q", muchos)
	}
}

func TestResumenFaltantes(t *testing.T) {
	if got := resumenFaltantes(1); !strings.Contains(got, "Falta 1") || strings.Contains(got, "Faltan") {
		t.Errorf("singular mal formado: %q", got)
	}
	if got := resumenFaltantes(3); !strings.Contains(got, "Faltan 3") {
		t.Errorf("plural mal formado: %q", got)
	}
}

// El caso que importa: apretar una opción trabada tiene que devolver qué falta y
// qué hacer, no arrancar un paso que va a fallar al final.
func TestOpcionTrabadaExplicaEnVezDeArrancar(t *testing.T) {
	t.Setenv(envSAPassword, "")
	t.Setenv(envAppPassword, "")

	m := NewModel(devCfg(), "")
	m.checks = checklistFalso(config.DbDocker, precheck.ReqAdmin)
	m = pulsar(m, "2") // Reparar

	if m.screen != screenDone {
		t.Fatalf("pantalla = %v, quiero la explicación en screenDone", m.screen)
	}
	if !m.bloqueo {
		t.Error("no quedó marcado como bloqueo")
	}
	if m.taskErr != nil {
		t.Errorf("un bloqueo no es un error del paso: %v", m.taskErr)
	}
	if m.task != taskNone {
		t.Errorf("arrancó la tarea igual: %v", m.task)
	}
	if m.taskName != "Reparar" {
		t.Errorf("no dijo qué opción se bloqueó: %q", m.taskName)
	}
	junto := strings.Join(m.lines, "\n")
	for _, esperado := range []string{"Permisos de administrador"} {
		if !strings.Contains(junto, esperado) {
			t.Errorf("no dijo qué hacer (%q). Texto:\n%s", esperado, junto)
		}
	}
}

// Y con la puerta abierta, la opción arranca como siempre.
func TestOpcionDesbloqueadaArranca(t *testing.T) {
	t.Setenv(envSAPassword, "Sa*2026*Dev")
	t.Setenv(envAppPassword, "Dv*123")

	m := NewModel(devCfg(), "")
	m.checks = checklistFalso(config.DbDocker)
	m = pulsar(m, "2")

	if m.screen != screenWorking || m.task != taskRepair {
		t.Fatalf("pantalla=%v tarea=%v, quiero que arranque Reparar", m.screen, m.task)
	}
	if m.bloqueo {
		t.Error("quedó marcado como bloqueo con el checklist limpio")
	}
}

func TestTeclaCMuestraElChecklist(t *testing.T) {
	m := pulsar(NewModel(devCfg(), ""), "c")
	if m.screen != screenChecklist {
		t.Fatalf("pantalla = %v, quiero screenChecklist", m.screen)
	}
	m = pulsar(m, "esc")
	if m.screen != screenMenu {
		t.Errorf("esc no volvió al menú: %v", m.screen)
	}
}

// Al terminar un paso el checklist se recalcula solo: si no, el operador seguiría
// viendo trabada la opción que acaba de destrabar.
func TestAlTerminarUnPasoSeRecalculaElChecklist(t *testing.T) {
	m := NewModel(devCfg(), "")
	out, cmd := m.Update(taskFinishedMsg{kind: taskSetupDB})
	m = out.(Model)
	if cmd == nil {
		t.Fatal("no disparó el recálculo del checklist")
	}
	if !m.verificando {
		t.Error("no quedó marcado como verificando")
	}
	if m.bloqueo {
		t.Error("quedó marcado como bloqueo después de correr un paso")
	}

	out, _ = m.Update(checksMsg{rs: checklistFalso(config.DbDocker)})
	m = out.(Model)
	if m.verificando {
		t.Error("siguió en verificando después de recibir el checklist")
	}
	if len(m.checks) == 0 {
		t.Error("no guardó el checklist recibido")
	}
}

func TestChecklistEnPantallaMuestraArregloYTraba(t *testing.T) {
	m := NewModel(devCfg(), "")
	m.checks = checklistFalso(config.DbDocker, precheck.ReqAdmin)
	m.screen = screenChecklist

	c := m.View().Content
	for _, esperado := range []string{
		"Permisos de administrador", // el requisito
		"sesión sin elevar",
		"por qué:", "arreglo:", "traba:",
		"Falta 1",
	} {
		if !strings.Contains(c, esperado) {
			t.Errorf("el checklist no muestra %q. Pantalla:\n%s", esperado, c)
		}
	}
}

func TestChecklistMientrasVerificaNoMiente(t *testing.T) {
	m := NewModel(devCfg(), "")
	m.screen = screenChecklist
	m.verificando = true
	c := m.View().Content
	if !strings.Contains(c, "Verificando") {
		t.Errorf("mientras verifica debería decirlo:\n%s", c)
	}
	if strings.Contains(c, "Falta") || strings.Contains(c, "sin bloqueos") {
		t.Errorf("mientras verifica no puede afirmar nada del resultado:\n%s", c)
	}
}

// El menú tiene que mostrar la puerta antes de que el operador la choque.
func TestMenuMuestraLaPuerta(t *testing.T) {
	m := NewModel(devCfg(), "")
	m.checks = checklistFalso(config.DbDocker, precheck.ReqAdmin)
	c := m.View().Content

	if !strings.Contains(c, "CHECKLIST: 1 bloquea") {
		t.Errorf("el menú no resume el bloqueo:\n%s", c)
	}
	if !strings.Contains(c, "falta: Permisos de administrador") {
		t.Errorf("el menú no dice qué opción está trabada y por qué:\n%s", c)
	}
	// Permisos de administrador traba Instalación completa y Setup App
	if strings.Count(c, "(falta:") != 2 {
		t.Errorf("esperaba 2 opciones trabadas (Instalación completa y Setup App):\n%s", c)
	}
}

// La sonda de motor devuelve tres estados; acá casi siempre queremos decir "anda".
// El tercero ("no se pudo verificar") tiene su propio test en precheck.
func motorUI(e precheck.Estado, detalle string) func(string) (precheck.Estado, string) {
	return func(string) (precheck.Estado, string) { return e, detalle }
}
