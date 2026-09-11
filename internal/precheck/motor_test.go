// © Antony Monge López — Costa Rica — Céd. 604700548
package precheck

import (
	"strings"
	"testing"

	"aegis-setup/internal/config"
)

// Regresión del bug que salió corriendo el binario contra un perfil prod: la sonda
// dialaba "localhost" crudo y net.Dial fallaba con "missing port in address". El
// resultado era una máquina sana en rojo, y la instalación trabada por un puerto
// que la sonda nunca se molestó en completar.
func TestMotorAgregaElPuertoQueFalta(t *testing.T) {
	e, det := motorAlcanzable("localhost")
	if strings.Contains(det, "missing port") {
		t.Fatalf("dialó sin puerto: %q", det)
	}
	if e == EstadoOK {
		t.Skipf("hay SQL escuchando en localhost:1433, no se puede probar el fallo: %q", det)
	}
	if e != EstadoFalta {
		t.Errorf("esperaba Falta con puerto cerrado, got %v (%q)", e, det)
	}
	if !strings.Contains(det, "1433") {
		t.Errorf("el detalle tiene que decir a qué puerto se conectó: %q", det)
	}
}

// Una instancia con nombre (el default de Express, que es el motor de prod) no se
// puede sondear por TCP: SQL Browser negocia el puerto. Si esto devolviera fallo,
// la máquina de producción quedaría en rojo por una limitación de la sonda, no por
// un problema real — y encima trabaría la instalación.
func TestInstanciaConNombreNoEsFallo(t *testing.T) {
	for _, srv := range []string{`localhost\SQLEXPRESS`, `.\SQLEXPRESS`, `CONTABILIDAD\SQLEXPRESS`} {
		e, det := motorAlcanzable(srv)
		if e == EstadoFalta {
			t.Errorf("motorAlcanzable(%q) = Falta (%q); una instancia con nombre no se puede sondear así", srv, det)
		}
		if e != EstadoAviso {
			t.Errorf("motorAlcanzable(%q) = %v, quiero Aviso", srv, e)
		}
		if !strings.Contains(det, "SQL Browser") {
			t.Errorf("el detalle tiene que explicar por qué no se puede verificar: %q", det)
		}
	}
}

// El aviso no puede trabar nada: es una limitación de nuestra medición, no un
// requisito incumplido.
func TestAvisoDeInstanciaConNombreNoTraba(t *testing.T) {
	s := sondasOK()
	s.Motor = motorAlcanzable
	cfg := config.Default()
	cfg.DbMode = config.DbLocal
	cfg.Server = `localhost\SQLEXPRESS`

	r := buscar(t, Run(cfg, "", s), ReqMotor)
	if r.Estado != EstadoAviso {
		t.Fatalf("estado = %v (%q), quiero Aviso", r.Estado, r.Detalle)
	}
	for _, x := range Bloqueantes(Run(cfg, "", s)) {
		if x.ID == ReqMotor {
			t.Fatal("un aviso no puede contar como bloqueante")
		}
	}
}
