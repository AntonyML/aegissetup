// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"strings"
	"testing"

	"aegis-setup/internal/config"
)

// Esta función nació de un bug real: el checklist reportaba "dial tcp: address
// localhost: missing port in address" en el perfil prod local, con la máquina
// sana. El checklist leía eso como "el motor no está" y trababa la instalación.
func TestHostPuerto(t *testing.T) {
	casos := []struct{ in, want, por string }{
		{"localhost,14333", "localhost:14333", "ODBC usa coma, la red usa dos puntos"},
		{"localhost:14333", "localhost:14333", "ya viene normalizado"},
		{"localhost", "localhost:1433", "el default de SQL Server"},
		{"CONTABILIDAD", "CONTABILIDAD:1433", "instancia por defecto sin puerto"},
		{`localhost\SQLEXPRESS`, `localhost\SQLEXPRESS`, "instancia con nombre: el puerto lo negocia SQL Browser, forzar 1433 la rompe"},
		{`.\SQLEXPRESS`, `.\SQLEXPRESS`, "ídem, con el atajo de máquina local"},
		{`CONTABILIDAD\SQLEXPRESS,1433`, `CONTABILIDAD\SQLEXPRESS:1433`, "puerto explícito + instancia con nombre"},
		{"MI_SERVIDOR,11433", "MI_SERVIDOR:11433", "puerto no estándar"},
		{"[::1]", "[::1]:1433", "IPv6 sin puerto"},
		{"[::1]:1433", "[::1]:1433", "IPv6 con puerto"},
	}
	for _, c := range casos {
		if got := HostPuerto(c.in); got != c.want {
			t.Errorf("HostPuerto(%q) = %q, want %q — %s", c.in, got, c.want, c.por)
		}
	}
}

// Sin puerto no se puede dialar: la sonda de motor tiene que poder contar con que
// HostPuerto ya lo puso, sea cual sea el server que escribió el operador.
func TestHostPuertoSiempreDialable(t *testing.T) {
	for _, in := range []string{"localhost", "CONTABILIDAD", "localhost,14333", "MI_SERVIDOR:11433"} {
		got := HostPuerto(in)
		if !tienePuerto(got) {
			t.Errorf("HostPuerto(%q) = %q y sigue sin puerto", in, got)
		}
	}
}

func TestURLHost(t *testing.T) {
	casos := []struct{ in, want string }{
		{`192.168.2.145\SQLEXPRESS`, `192.168.2.145/SQLEXPRESS`},
		{`localhost\SQLEXPRESS`, `localhost/SQLEXPRESS`},
		{`192.168.2.145,54721`, `192.168.2.145:54721`},
		{"localhost", "localhost:1433"},
	}
	for _, c := range casos {
		if got := URLHost(c.in); got != c.want {
			t.Errorf("URLHost(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestAppDSNNamedInstance(t *testing.T) {
	cfg := config.Default()
	cfg.Server = `192.168.2.145\SQLEXPRESS`
	cfg.Database = "SIDC"
	cfg.UseWinAuth = true

	dsn := AppDSN(cfg, "")
	if strings.Contains(dsn, `\`) {
		t.Errorf("AppDSN no debe contener barras invertidas que rompan url.Parse: %q", dsn)
	}
	if !strings.Contains(dsn, "192.168.2.145/SQLEXPRESS") {
		t.Errorf("AppDSN debe contener 192.168.2.145/SQLEXPRESS: %q", dsn)
	}
}

