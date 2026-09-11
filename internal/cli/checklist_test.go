// © Antony Monge López — Costa Rica — Céd. 604700548
package cli

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"aegis-setup/internal/config"
	"aegis-setup/internal/precheck"
)

// sondasCLI corre el checklist sobre una máquina imaginaria. La CLI recibe las
// sondas inyectadas justo para esto: probar la salida sin ser admin, sin Docker y
// sin SysWOW64.
func sondasCLI(roto map[precheck.ID]bool) func() precheck.Sondas {
	return func() precheck.Sondas {
		s := precheck.Sondas{
			Admin:    func() (bool, string) { return true, "ok" },
			Maquina:  func() (bool, string) { return true, "ok" },
			Docker:   func() (bool, string) { return true, "ok" },
			Motor:    motorCLI(precheck.EstadoOK, "ok"),
			ODBC:     func(string) (bool, string) { return true, "ok" },
			Base:     func(config.Config, string) (bool, string) { return true, "ok" },
			DSN:      func(string) (bool, string) { return true, "ok" },
			Crystal:  func() []string { return nil },
			OCX:      func(config.Config) precheck.EstadoOCX { return precheck.EstadoOCX{} },
			Backup:   func(config.Config) (string, error) { return "SIDC.bak", nil },
			AppFiles: func(string) []string { return nil },
		}
		if roto[precheck.ReqAdmin] {
			s.Admin = func() (bool, string) { return false, "sesión sin elevar" }
		}
		if roto[precheck.ReqDocker] {
			s.Docker = func() (bool, string) { return false, "docker no responde" }
		}
		if roto[precheck.ReqMotor] {
			s.Motor = motorCLI(precheck.EstadoFalta, "connection refused")
		}
		if roto[precheck.ReqBackup] {
			s.Backup = func(config.Config) (string, error) { return "", errors.New("no hay .bak en ninguna carpeta") }
		}
		if roto[precheck.ReqBase] {
			s.Base = func(config.Config, string) (bool, string) { return false, "login failed" }
		}
		if roto[precheck.ReqDSN] {
			s.DSN = func(string) (bool, string) { return false, "no existe el DSN" }
		}
		if roto[precheck.ReqApp] {
			s.AppFiles = func(string) []string { return []string{"exe: falta"} }
		}
		return s
	}
}

// salidaDeChecklist corre el comando de verdad y devuelve lo que escribió.
func salidaDeChecklist(t *testing.T, modo config.DbMode, roto map[precheck.ID]bool) (string, error) {
	t.Helper()
	cfg := config.Default()
	cfg.DbMode = modo

	var buf bytes.Buffer
	err := escribirChecklist(&buf, cfg, precheck.Run(cfg, "", sondasCLI(roto)()))
	return buf.String(), err
}

func TestChecklistMuestraLosTresEstados(t *testing.T) {
	// Motor y respaldo faltan; la base falta y no traba nada.
	roto := map[precheck.ID]bool{precheck.ReqMotor: true, precheck.ReqBackup: true, precheck.ReqBase: true}
	out, err := salidaDeChecklist(t, config.DbDocker, roto)

	if !errors.Is(err, ErrCheckFail) {
		t.Fatalf("err = %v, quiero ErrCheckFail (traba la instalación)", err)
	}
	for _, esperado := range []string{
		"OK   ",               // permiso, Docker, driver, Crystal, app
		"FALTA  4. Motor SQL", // el rojo que traba
		"connection refused",  // el detalle crudo de la sonda
		"por qué: ",           // para qué sirve
		"arreglo: ",           // qué hacer
		"traba:   Setup DB, Instalación completa",
		"2 requisitos traban la instalación",
	} {
		if !strings.Contains(out, esperado) {
			t.Errorf("no aparece %q en:\n%s", esperado, out)
		}
	}
	// La base falta, pero no traba: no puede figurar entre los bloqueantes.
	if strings.Contains(out, "Base SIDC en el motor, ") {
		t.Errorf("la base contó como bloqueante:\n%s", out)
	}
}

// El caso que va a pasar más seguido: PC limpia, nada instalado. El checklist
// tiene que seguir diciendo que el operador puede arrancar.
func TestChecklistConTodoRotoSoloCuentaLoQueTraba(t *testing.T) {
	roto := map[precheck.ID]bool{
		precheck.ReqAdmin: true, precheck.ReqDocker: true, precheck.ReqMotor: true,
		precheck.ReqBackup: true, precheck.ReqBase: true, precheck.ReqDSN: true,
		precheck.ReqApp: true,
	}
	out, err := salidaDeChecklist(t, config.DbDocker, roto)
	if !errors.Is(err, ErrCheckFail) {
		t.Fatalf("err = %v, quiero ErrCheckFail", err)
	}

	// 5 traban (permisos, Docker, motor, respaldo, app) y la base y el DSN no.
	if !strings.Contains(out, "5 requisitos traban la instalación") {
		t.Errorf("contó mal los bloqueantes:\n%s", out)
	}
	if !strings.Contains(out, "Resolvelos en el orden de la lista") {
		t.Errorf("no dice en qué orden resolverlos:\n%s", out)
	}
	// El orden de la lista tiene que ser el de resolución, no el del código.
	uno := strings.Index(out, "1. Permisos de administrador")
	dos := strings.Index(out, "2. Windows de 64 bits")
	if uno < 0 || dos < 0 || uno > dos {
		t.Errorf("el orden de la lista no es el de resolución (1 en %d, 2 en %d):\n%s", uno, dos, out)
	}
}

func TestChecklistLimpioNoFalla(t *testing.T) {
	out, err := salidaDeChecklist(t, config.DbLocal, nil)
	if err != nil {
		t.Fatalf("un checklist limpio no puede fallar: %v", err)
	}
	if !strings.Contains(out, "Todo en orden") {
		t.Errorf("no dijo que está todo bien:\n%s", out)
	}
	if strings.Contains(out, "FALTA") {
		t.Errorf("hay un FALTA en un checklist limpio:\n%s", out)
	}
}

// En una PC recién formateada la base y el DSN faltan y no traban nada. Si el
// comando fallara por eso, un script de instalación no podría arrancar nunca.
func TestChecklistConSoloLaBaseFaltanteNoFalla(t *testing.T) {
	roto := map[precheck.ID]bool{precheck.ReqBase: true, precheck.ReqDSN: true}
	out, err := salidaDeChecklist(t, config.DbLocal, roto)
	if err != nil {
		t.Fatalf("la base y el DSN no traban: %v", err)
	}
	if !strings.Contains(out, "Nada traba la instalación") {
		t.Errorf("no explicó por qué se puede seguir:\n%s", out)
	}
	if !strings.Contains(out, "FALTA") {
		t.Errorf("igual tenía que mostrar los rojos:\n%s", out)
	}
}

// Docker no aplica en prod local: el checklist no puede pedirlo ahí.
func TestChecklistNoPideDockerFueraDePruebas(t *testing.T) {
	out, _ := salidaDeChecklist(t, config.DbLocal, nil)
	if strings.Contains(out, "Docker") {
		t.Errorf("pidió Docker en un perfil que no lo usa:\n%s", out)
	}
}

// El "1 requisitos" se lee como un bug del programa, no como un dato. Salió de
// correr el binario contra un perfil prod real, así que queda cubierto.
func TestPieBienFormadoEnSingularYPlural(t *testing.T) {
	uno, err := salidaDeChecklist(t, config.DbLocal, map[precheck.ID]bool{precheck.ReqMotor: true})
	if !errors.Is(err, ErrCheckFail) {
		t.Fatalf("err = %v, quiero ErrCheckFail", err)
	}
	if !strings.Contains(uno, "== 1 requisito traba la instalación") {
		t.Errorf("singular mal formado:\n%s", uno)
	}
	if strings.Contains(uno, "1 requisitos") {
		t.Errorf("quedó el plural pegado al 1:\n%s", uno)
	}
	if !errors.Is(err, ErrCheckFail) || !strings.Contains(err.Error(), "1 requisito traba") {
		t.Errorf("el mensaje de error no acompaña al pie: %v", err)
	}

	dos, _ := salidaDeChecklist(t, config.DbLocal, map[precheck.ID]bool{precheck.ReqMotor: true, precheck.ReqBackup: true})
	if !strings.Contains(dos, "== 2 requisitos traban la instalación") {
		t.Errorf("plural mal formado:\n%s", dos)
	}
}

// El error que devuelve tiene que decir lo mismo que el reporte: si el mensaje
// del código de salida y el pie del reporte se contradicen, un script y un
// humano leen dos verdades distintas de la misma corrida.
func TestElErrorDiceLoMismoQueElPie(t *testing.T) {
	cfg := config.Default()
	rs := precheck.Run(cfg, "", sondasCLI(map[precheck.ID]bool{precheck.ReqMotor: true})())

	var buf bytes.Buffer
	err := escribirChecklist(&buf, cfg, rs)
	if err == nil {
		t.Fatal("tenía que devolver ErrCheckFail")
	}
	// El error lleva el prefijo del centinela ("la máquina no está lista: "); lo
	// que tiene que coincidir con el reporte es el veredicto.
	pie := strings.TrimPrefix(err.Error(), ErrCheckFail.Error()+": ")
	if pie == err.Error() {
		t.Fatalf("el error no envuelve ErrCheckFail: %v", err)
	}
	if !strings.Contains(buf.String(), pie) {
		t.Errorf("el error %q no aparece tal cual en el reporte:\n%s", pie, buf.String())
	}
}

// El número del pie tiene que ser exactamente la cantidad de bloqueantes: si la
// CLI contara por un lado y la puerta del TUI por otro, el operador vería dos
// verdades sobre la misma máquina.
func TestElPieCuentaExactamenteLosBloqueantes(t *testing.T) {
	roto := map[precheck.ID]bool{
		precheck.ReqAdmin: true, precheck.ReqDocker: true, precheck.ReqMotor: true,
		precheck.ReqBackup: true, precheck.ReqBase: true, precheck.ReqDSN: true,
		precheck.ReqApp: true,
	}
	cfg := config.Default()
	rs := precheck.Run(cfg, "", sondasCLI(roto)())

	var buf bytes.Buffer
	_ = escribirChecklist(&buf, cfg, rs)
	salida := buf.String()

	titulo := fmt.Sprintf("%d requisitos traban la instalación", len(precheck.Bloqueantes(rs)))
	i := strings.Index(salida, "== "+titulo)
	if i < 0 {
		t.Fatalf("el pie no dice %q:\n%s", titulo, salida)
	}
	pie := salida[i:]

	// Todo bloqueante tiene que estar nombrado en el pie: es la lista de tareas.
	for _, r := range precheck.Bloqueantes(rs) {
		if !strings.Contains(pie, r.Titulo) {
			t.Errorf("el bloqueante %q no aparece en el pie:\n%s", r.Titulo, pie)
		}
	}
	// Y ningún no-bloqueante puede figurar ahí: es el error que este test evita.
	for _, r := range precheck.Faltantes(rs) {
		if len(precheck.Traba(r.ID)) == 0 && strings.Contains(pie, r.Titulo) {
			t.Errorf("%q no traba nada y está en la lista de lo que traba:\n%s", r.Titulo, pie)
		}
	}
}

// La sonda de motor devuelve tres estados (OK / falla / no se pudo verificar).
func motorCLI(e precheck.Estado, detalle string) func(string) (precheck.Estado, string) {
	return func(string) (precheck.Estado, string) { return e, detalle }
}
