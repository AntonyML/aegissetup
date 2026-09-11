// © Antony Monge López — Costa Rica — Céd. 604700548
package ui

import (
	"strings"
	"testing"

	"aegis-setup/internal/config"
)

// cfgSinAppDir es una PC recién instalada: no hay config.json y nadie dijo todavía dónde
// está la carpeta de SIDC.
func cfgSinAppDir() config.Config {
	cfg := config.Default()
	cfg.AppDir = ""
	return cfg
}

// La carpeta de SIDC se pregunta en los tres perfiles, no solo en prod. Antes venía un
// default (C:\DEV\SIDC) y el instalador medía el repo del desarrollador: en una PC limpia
// ese camino no existe, y si existe le dice al operador que SIDC está bien instalado
// cuando en realidad está mirando otra máquina.
func TestLaCarpetaDeSIDCSePreguntaEnLosTresPerfiles(t *testing.T) {
	for _, p := range perfiles {
		m, path := nuevoEnPrimeraVez(t, cfgSinAppDir())
		m = pulsar(m, p.key)
		if m.screen == screenServer {
			m = escribir(m, "SIDC01")
		}
		if m.screen != screenAppDir {
			t.Errorf("perfil %q -> pantalla %v, quiero screenAppDir", p.name, m.screen)
			continue
		}
		if m.dirInput.Value() != "" {
			t.Errorf("perfil %q: el campo viene precargado con %q; en prod nadie puede adivinar esa carpeta",
				p.name, m.dirInput.Value())
		}
		if m.dirInput.Placeholder == "" {
			t.Errorf("perfil %q: el campo no muestra ningún ejemplo", p.name)
		}
		if hayArchivo(path) {
			t.Errorf("perfil %q: escribió un config sin saber dónde está SIDC", p.name)
		}
	}
}

// Elegir perfil no alcanza para guardar: la config se escribe cuando la carpeta está
// contestada. Guardarla antes dejaría en disco un app_dir vacío que después traba Setup App
// con un mensaje que no tiene nada que ver con lo que hizo el operador.
func TestSinCarpetaNoSeAplicaElPerfil(t *testing.T) {
	m, path := nuevoEnPrimeraVez(t, cfgSinAppDir())
	m = pulsar(m, "2")

	if m.screen != screenAppDir {
		t.Fatalf("pantalla = %v, quiero screenAppDir", m.screen)
	}
	if m.taskErr != nil {
		t.Fatalf("error inesperado: %v", m.taskErr)
	}
	if hayArchivo(path) {
		t.Fatal("guardó el perfil con la carpeta sin contestar")
	}
}

// La carpeta contestada tiene que llegar al disco y a la memoria: la corrida siguiente y
// el checklist en pantalla tienen que medir la misma carpeta que el operador escribió.
func TestLaCarpetaContestadaSeGuardaYSeMide(t *testing.T) {
	m, path := nuevoEnPrimeraVez(t, cfgSinAppDir())
	m = pulsar(m, "2")
	m = escribir(m, `D:\SIDC`) // escribir ya confirma con Enter

	if m.taskErr != nil {
		t.Fatalf("error inesperado: %v", m.taskErr)
	}
	if m.screen != screenDone {
		t.Fatalf("pantalla = %v, quiero la confirmación", m.screen)
	}
	if m.cfg.AppDir != `D:\SIDC` {
		t.Errorf("cfg en memoria app_dir = %q, quiero D:\\SIDC", m.cfg.AppDir)
	}
	guardada, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if guardada.AppDir != `D:\SIDC` {
		t.Errorf("disco app_dir = %q, quiero D:\\SIDC", guardada.AppDir)
	}
	if !strings.Contains(strings.Join(m.lines, "\n"), `D:\SIDC`) {
		t.Errorf("la confirmación no dice dónde quedó la carpeta de SIDC:\n%s", strings.Join(m.lines, "\n"))
	}
	// El checklist viejo medía otra carpeta (o el default del programa): se recalcula.
	if !m.verificando || m.checks != nil {
		t.Errorf("verificando=%v checks=%d, quiero recalcular el checklist", m.verificando, len(m.checks))
	}
}

// Enter en el campo vacío no guarda una carpeta vacía: es el caso de la PC limpia, donde
// la respuesta correcta todavía no está dicha y el operador puede creer que alcanza.
func TestCarpetaVaciaNoSeAcepta(t *testing.T) {
	m, path := nuevoEnPrimeraVez(t, cfgSinAppDir())
	m = pulsar(m, "2")
	m.dirInput.SetValue("")
	m = pulsar(m, "enter")

	if m.screen != screenAppDir {
		t.Fatalf("pantalla = %v, se fue con la carpeta vacía", m.screen)
	}
	if m.dirErr == "" {
		t.Error("no explicó por qué no acepta vacío")
	}
	if hayArchivo(path) {
		t.Error("escribió un config con app_dir vacío sin que el operador lo decidiera")
	}
}

// Una ruta relativa se resolvería contra el directorio de trabajo de quien lance Aegis, así
// que la misma config mediría carpetas distintas. Se rechaza en la pantalla, no después.
func TestCarpetaRelativaNoSeAcepta(t *testing.T) {
	m, path := nuevoEnPrimeraVez(t, cfgSinAppDir())
	m = pulsar(m, "2")
	m = escribir(m, `SIDC`)
	m = pulsar(m, "enter")

	if m.screen != screenAppDir {
		t.Fatalf("pantalla = %v, se fue con una ruta relativa", m.screen)
	}
	if m.dirErr == "" {
		t.Error("no explicó por qué no acepta una ruta relativa")
	}
	if hayArchivo(path) {
		t.Error("escribió un config con app_dir relativo")
	}
}

// Con --app-dir no se pregunta: es el camino desatendido (scripts, instalación sin nadie
// frente a la pantalla) y ahí no hay quién tipee.
func TestConFlagAppDirNoPregunta(t *testing.T) {
	m, path := nuevoEnPrimeraVez(t, cfgSinAppDir())
	m = m.SetPresetAppDir(`E:\SIDC`)
	m = pulsar(m, "2")

	if m.screen == screenAppDir {
		t.Fatal("preguntó la carpeta teniendo --app-dir")
	}
	guardada, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if guardada.AppDir != `E:\SIDC` || guardada.Env != "prod" {
		t.Errorf("disco = app_dir %q env %q, quiero E:\\SIDC prod", guardada.AppDir, guardada.Env)
	}
}

// Cuando la carpeta ya se conoce (config anterior o el repo deducido por la posición del
// EXE), el campo viene con ese valor: el operador confirma con Enter o lo corrige, y ve
// qué carpeta se va a medir sin tener que abrir el config.json.
func TestLaCarpetaConocidaSePrecarga(t *testing.T) {
	m, _ := nuevoEnPrimeraVez(t, devCfg())
	m = pulsar(m, "2")

	if m.screen != screenAppDir {
		t.Fatalf("pantalla = %v, quiero screenAppDir", m.screen)
	}
	if m.dirInput.Value() != `C:\SIDC` {
		t.Errorf("campo = %q, quiero la carpeta de la config (C:\\SIDC)", m.dirInput.Value())
	}
}

// Esc en la carpeta de SIDC vuelve a preguntar el perfil cuando venía del arranque: caer al
// menú dejaría la config por defecto, que no es la de esta PC (el bug de F4).
func TestEscEnLaCarpetaVuelveAlPerfil(t *testing.T) {
	m, path := nuevoEnPrimeraVez(t, cfgSinAppDir())
	m = pulsar(m, "2")
	m = pulsar(m, "esc")

	if m.screen != screenPerfil {
		t.Fatalf("pantalla = %v, quiero screenPerfil", m.screen)
	}
	if hayArchivo(path) {
		t.Error("escribió un config después de cancelar")
	}
}

// carpetaDelRepo deduce la raíz del repo desde la posición del EXE y devuelve "" cuando el
// binario no está donde Aegis lo pone. Con el binario de las pruebas (una carpeta temporal)
// no hay repo: precargar ahí sería inventar una carpeta.
func TestLaCarpetaDelRepoSeDeduceSoloConElLayoutReal(t *testing.T) {
	if got := carpetaDelRepo(); got != "" {
		t.Errorf("con el binario de las pruebas carpetaDelRepo() = %q, quiero vacío", got)
	}
	m, _ := nuevoEnPrimeraVez(t, config.Default())
	_, ejemplo := m.sugerenciaAppDir()
	if ejemplo == "" {
		t.Error("sin carpeta conocida el prompt tiene que mostrar un ejemplo igual")
	}
}

// Volver con Esc desde el menú deja al operador donde estaba. Si cayera a la pantalla del
// perfil, el menú quedaría inalcanzable y parecería que Aegis se reinició.
func TestEscEnLaCarpetaDesdeElMenuVuelveAlMenu(t *testing.T) {
	m := NewModel(devCfg(), "")
	m = pulsar(m, "4") // preset dev desde el menú
	if m.screen != screenAppDir {
		t.Fatalf("pantalla = %v, quiero screenAppDir", m.screen)
	}
	m = pulsar(m, "esc")
	if m.screen != screenMenu {
		t.Fatalf("pantalla = %v, quiero screenMenu", m.screen)
	}
}
