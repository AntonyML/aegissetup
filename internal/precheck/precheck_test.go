// © Antony Monge López — Costa Rica — Céd. 604700548
package precheck

import (
	"errors"
	"strings"
	"testing"

	"aegis-setup/internal/config"
)

// sondasOK pasa todo. Cada test rompe la sonda que le importa: así el caso de
// prueba se lee de un vistazo en vez de armar once sondas cada vez.
func sondasOK() Sondas {
	return Sondas{
		Admin:    func() (bool, string) { return true, "ok" },
		Maquina:  func() (bool, string) { return true, "ok" },
		Docker:   func() (bool, string) { return true, "ok" },
		Motor:    motor(EstadoOK, "ok"),
		ODBC:     func(string) (bool, string) { return true, "ok" },
		Base:     func(config.Config, string) (bool, string) { return true, "ok" },
		DSN:      func(string) (bool, string) { return true, "ok" },
		Crystal:  func() []string { return nil },
		OCX:      func(config.Config) EstadoOCX { return EstadoOCX{} },
		Backup:   func(config.Config) (string, error) { return "SIDC.bak", nil },
		AppFiles: func(string) []string { return nil },
	}
}

func ids(rs []Requisito) []ID {
	out := make([]ID, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.ID)
	}
	return out
}

func buscar(t *testing.T, rs []Requisito, id ID) Requisito {
	t.Helper()
	r, ok := Buscar(rs, id)
	if !ok {
		t.Fatalf("no está %s en el checklist: %v", id, ids(rs))
	}
	return r
}

func TestRunOrdenDeResolucion(t *testing.T) {
	// El orden es la promesa de la pantalla: primero lo que desbloquea el resto.
	// Se compara contra ordenDeResolucion y no contra una lista escrita acá, para
	// que agregar un requisito y olvidarse de la tabla de trabas rompa este test
	// en vez de romper la puerta en silencio.
	got := ids(Run(config.Default(), "", sondasOK()))
	if len(got) != len(ordenDeResolucion) {
		t.Fatalf("cantidad de requisitos: got %v, want %v", got, ordenDeResolucion)
	}
	for i := range ordenDeResolucion {
		if got[i] != ordenDeResolucion[i] {
			t.Errorf("posición %d: got %s, want %s", i, got[i], ordenDeResolucion[i])
		}
	}

	// En el perfil sin Docker el requisito de Docker no aplica: no tiene sentido
	// pedir Docker para una instalación contra un SQL Server local.
	cfg := config.Default()
	cfg.DbMode = config.DbLocal
	got = ids(Run(cfg, "", sondasOK()))
	want := make([]ID, 0, len(ordenDeResolucion))
	for _, id := range ordenDeResolucion {
		if id != ReqDocker {
			want = append(want, id)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("perfil local: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("perfil local, posición %d: got %s, want %s", i, got[i], want[i])
		}
	}
}

// La distinción que sostiene el mensaje del menú: en una PC limpia la base y el
// DSN están en rojo y no traban nada. Si se contaran como bloqueos, el menú diría
// que no se puede instalar cuando en realidad sí se puede.
func TestBloqueantesNoEsFaltantes(t *testing.T) {
	// PC limpia: todo lo de la máquina falta, y la base y el DSN también porque
	// todavía no se corrió nada.
	s := sondasOK()
	s.Admin = func() (bool, string) { return false, "sin elevar" }
	s.Docker = func() (bool, string) { return false, "docker no responde" }
	s.Motor = motor(EstadoFalta, "connection refused")
	s.ODBC = func(string) (bool, string) { return false, "no instalado" }
	s.Base = func(config.Config, string) (bool, string) { return false, "login failed" }
	s.DSN = func(string) (bool, string) { return false, "no existe" }
	s.Crystal = func() []string { return []string{"crpe32.dll"} }
	s.Backup = func(config.Config) (string, error) { return "", errors.New("no hay .bak") }
	s.AppFiles = func(string) []string { return []string{"exe: falta"} }

	rs := Run(config.Default(), "", s)
	faltan := Faltantes(rs)
	bloquean := Bloqueantes(rs)

	if len(bloquean) >= len(faltan) {
		t.Fatalf("bloqueantes=%d faltantes=%d: la base y el DSN deberían faltar sin trabar", len(bloquean), len(faltan))
	}
	for _, id := range []ID{ReqBase, ReqDSN} {
		if r := buscar(t, rs, id); r.Estado != EstadoFalta {
			t.Errorf("%s debería estar en falta en una PC limpia, está en %d", id, r.Estado)
		}
		for _, r := range bloquean {
			if r.ID == id {
				t.Errorf("%s contó como bloqueante y no traba nada", id)
			}
		}
	}
	// Y todo bloqueante tiene que decir a quién traba: es la diferencia entre una
	// lista de rojos y un plan.
	for _, r := range bloquean {
		if len(r.Traba) == 0 {
			t.Errorf("%s está en bloqueantes sin etapas", r.ID)
		}
	}
}

// Etapas arma el texto del "traba:" del checklist.
func TestEtapasFormatea(t *testing.T) {
	if got := Etapas(Traba(ReqBackup)); got != "Setup DB, Instalación completa" {
		t.Errorf("got %q", got)
	}
	if got := Etapas(nil); got != "" {
		t.Errorf("got %q, quiero vacío", got)
	}
}

// Cada requisito que Run emite tiene que tener una entrada en la tabla de trabas,
// aunque sea vacía. Un ID sin entrada es un requisito que nunca bloquea nada: si
// alguien lo agrega y se olvida de la tabla, la puerta no se abre nunca y nadie
// se entera.
func TestTodoRequisitoDeRunTieneEntradaEnLaTabla(t *testing.T) {
	for _, id := range ids(Run(config.Default(), "", sondasOK())) {
		if _, ok := trabaPorRequisito[id]; !ok {
			t.Errorf("%s no está en trabaPorRequisito: nunca va a trabar nada", id)
		}
	}
	for _, id := range ordenDeResolucion {
		if _, ok := trabaPorRequisito[id]; !ok {
			t.Errorf("%s está en ordenDeResolucion y no en trabaPorRequisito", id)
		}
	}
}

// La instalación completa no puede exigir menos que cada paso suelto: si no,
// alguien pasaría la puerta de un paso y se estrellaría con la grande.
func TestInstallEsSuperconjuntoDeCadaPaso(t *testing.T) {
	install := BloqueantesDe(EtapaInstall)
	tiene := func(ids []ID, id ID) bool {
		for _, x := range ids {
			if x == id {
				return true
			}
		}
		return false
	}
	for _, e := range []Etapa{EtapaSetupDB, EtapaSetupApp} {
		for _, id := range BloqueantesDe(e) {
			if !tiene(install, id) {
				t.Errorf("%s traba %s y no traba la instalación completa", id, e)
			}
		}
	}
}

// Las decisiones de diseño que sostienen todo el desbloqueo progresivo.
func TestLoQueNoTrabaNada(t *testing.T) {
	// La base y el DSN son el resultado del flujo, no su entrada. Exigirlos antes
	// sería circular: "no puedo crear la base porque no existe la base".
	for _, id := range []ID{ReqBase, ReqDSN} {
		if len(Traba(id)) != 0 {
			t.Errorf("%s traba %v: es salida del flujo, no requisito previo", id, Traba(id))
		}
	}

	// Crystal lo instala Setup App: exigirlo antes dejaría al operador sin ninguna
	// acción que pueda resolverlo.
	for _, e := range Traba(ReqCrystal) {
		if e == EtapaSetupApp {
			t.Error("Crystal traba Setup App, que es justamente quien lo instala")
		}
	}

	// Los OCX los instala Setup App, y además se avisa (nunca se marca falta).
	if len(Traba(ReqOCX)) != 0 {
		t.Errorf("los OCX traban %v", Traba(ReqOCX))
	}

	// Sin permisos no se puede nada de lo que escribe en la máquina.
	if got := len(BloqueantesDe(EtapaSetupDB)); got < 4 {
		t.Errorf("Setup DB solo tiene %d bloqueantes: %v", got, BloqueantesDe(EtapaSetupDB))
	}
}

// El orden de los bloqueantes tiene que seguir el orden del checklist: es lo que
// hace que "resolvelos de arriba hacia abajo" sea cierto también en el mensaje.
func TestBloqueantesSiguenElOrdenDeResolucion(t *testing.T) {
	pos := map[ID]int{}
	for i, id := range ordenDeResolucion {
		pos[id] = i
	}
	for _, e := range []Etapa{EtapaSetupDB, EtapaSetupApp, EtapaInstall} {
		bl := BloqueantesDe(e)
		for i := 1; i < len(bl); i++ {
			if pos[bl[i-1]] >= pos[bl[i]] {
				t.Errorf("%s: %s aparece antes que %s y va después en el checklist", e, bl[i-1], bl[i])
			}
		}
	}
}

// Etapa desconocida no puede devolver bloqueantes de casualidad.
func TestEtapaDesconocidaNoBloquea(t *testing.T) {
	if got := BloqueantesDe(Etapa("inventada")); len(got) != 0 {
		t.Errorf("una etapa inventada trajo bloqueantes: %v", got)
	}
}

func TestDockerSoloAplicaAlPerfilDePruebas(t *testing.T) {
	// En prod la base no vive en un contenedor: pedir Docker ahí sería mandar al
	// operador a instalar algo que no necesita.
	for _, modo := range []config.DbMode{config.DbLocal, config.DbServer} {
		cfg := config.Default()
		cfg.DbMode = modo
		for _, r := range Run(cfg, "", sondasOK()) {
			if r.ID == ReqDocker {
				t.Errorf("db_mode=%s: no debería pedir Docker", modo)
			}
		}
	}
}

func TestDockerAplicaEnPerfilDePruebas(t *testing.T) {
	cfg := config.Default()
	if cfg.DbMode != config.DbDocker {
		t.Fatalf("Default() debería ser el perfil de pruebas, es %s", cfg.DbMode)
	}
	if _, ok := Buscar(Run(cfg, "", sondasOK()), ReqDocker); !ok {
		t.Error("db_mode=docker: debería pedir Docker")
	}
}

func TestCadaRequisitoApareceUnaVez(t *testing.T) {
	vistos := map[ID]bool{}
	for _, r := range Run(config.Default(), "", sondasOK()) {
		if vistos[r.ID] {
			t.Errorf("%s aparece dos veces", r.ID)
		}
		vistos[r.ID] = true
	}
}

func TestNadaRojoCuandoTodoPasa(t *testing.T) {
	rs := Run(config.Default(), "", sondasOK())
	if f := Faltantes(rs); len(f) != 0 {
		t.Fatalf("esperaba 0 faltantes, hay %d: %v", len(f), ids(f))
	}
	for _, r := range rs {
		if r.Arreglo != "" {
			t.Errorf("%s está OK pero igual trae arreglo: %s", r.ID, r.Arreglo)
		}
	}
}

func TestCadaRojoDiceQueHacer(t *testing.T) {
	// El peor caso es una PC limpia, y es justo el caso donde el checklist tiene
	// que servir. Si un rojo no dice qué hacer, el operador queda mirando la
	// pantalla.
	cfg := config.Default()
	s := Sondas{
		Admin:   func() (bool, string) { return false, "sesión sin elevar" },
		Maquina: func() (bool, string) { return false, "sin SysWOW64" },
		Docker:  func() (bool, string) { return false, "docker no responde" },
		Motor:   motor(EstadoFalta, "connection refused"),
		ODBC:    func(string) (bool, string) { return false, "no instalado" },
		Base:    func(config.Config, string) (bool, string) { return false, "login failed" },
		DSN:     func(string) (bool, string) { return false, "no existe el DSN" },
		Crystal: func() []string { return []string{"crpe32.dll", "craxDrt.dll"} },
		OCX: func(config.Config) EstadoOCX {
			return EstadoOCX{FaltanEnSysWOW64: []string{"MSCOMCTL.OCX"}}
		},
		Backup:   func(config.Config) (string, error) { return "", errors.New("no hay .bak en ninguna carpeta") },
		AppFiles: func(string) []string { return []string{"exe: C:\\SIDC\\Sistema.exe"} },
	}

	rs := Run(cfg, "", s)
	if got, want := len(Faltantes(rs)), 10; got != want {
		t.Fatalf("faltantes: got %d, want %d (%v)", got, want, ids(rs))
	}
	for _, r := range rs {
		if r.Estado == EstadoOK {
			t.Errorf("%s no puede estar OK con todas las sondas fallando", r.ID)
		}
		for campo, v := range map[string]string{"motivo": r.Motivo, "detalle": r.Detalle, "arreglo": r.Arreglo} {
			if strings.TrimSpace(v) == "" {
				t.Errorf("%s: %s vacío", r.ID, campo)
			}
		}
	}
}

func TestArregloDelMotorCambiaPorAmbiente(t *testing.T) {
	// El motor de pruebas se levanta con compose; el de prod es un servicio que
	// debería estar corriendo. Un texto único para los dos manda a alguien a
	// hacer lo que no corresponde.
	s := sondasOK()
	s.Motor = motor(EstadoFalta, "connection refused")

	dev := config.Default() // docker
	if got := buscar(t, Run(dev, "", s), ReqMotor).Arreglo; !strings.Contains(got, "docker compose") {
		t.Errorf("dev: el arreglo debería decir cómo levantar el contenedor, dice: %s", got)
	}

	prod := config.Default()
	prod.DbMode = config.DbServer
	if got := buscar(t, Run(prod, "", s), ReqMotor).Arreglo; strings.Contains(got, "docker compose") {
		t.Errorf("prod: el arreglo no debería hablar de Docker, dice: %s", got)
	}
}

// El arreglo del motor de pruebas tiene que nombrar el compose que ESTA PC tiene: el paso 1
// lo deja en docker_dir. Antes decía "docker/docker-compose.yml", que es la ruta del repo y en
// la PC destino no existe, así que el operador copiaba un comando que no podía correr.
func TestElArregloDelMotorNombraElComposeInstalado(t *testing.T) {
	s := sondasOK()
	s.Motor = motor(EstadoFalta, "connection refused")

	dev := config.Default() // docker
	dev.DockerDir = `C:\ProgramData\AegisSetup\docker`
	got := buscar(t, Run(dev, "", s), ReqMotor).Arreglo

	if !strings.Contains(got, `C:\ProgramData\AegisSetup\docker\docker-compose.yml`) {
		t.Errorf("el arreglo no dice dónde está el compose de esta PC, dice: %s", got)
	}
	if strings.Contains(got, "-f docker/docker-compose.yml") {
		t.Errorf("el arreglo sigue mandando a la carpeta del repo, dice: %s", got)
	}
	// Ese archivo lo deja Aegis al correr el paso 1: si el arreglo no lo dice, un archivo que
	// todavía no está se lee como una instalación rota.
	if !strings.Contains(got, "Setup DB") {
		t.Errorf("el arreglo no aclara de dónde sale el compose, dice: %s", got)
	}
}

func TestOCXNuncaBloquea(t *testing.T) {
	// Los OCX son la única falta que se arregla desde el propio menú. Si
	// bloquearan Setup App, el operador no tendría ninguna acción para salir
	// del rojo.
	casos := []struct {
		nombre  string
		origen  []string
		espera  string
		prohibe string
	}{
		{
			nombre:  "faltan en SysWOW64 pero están en el origen",
			origen:  nil,
			espera:  "Setup App",
			prohibe: "PC vieja",
		},
		{
			// El kit viaja dentro del binario: este caso es un EXE mal armado, no una PC
			// sin la carpeta de la PC vieja. El arreglo tiene que apuntar al binario.
			nombre:  "el binario no trae el kit",
			origen:  []string{"MSCOMCTL.OCX"},
			espera:  "compilación vieja",
			prohibe: "Se resuelve solo",
		},
	}
	for _, c := range casos {
		s := sondasOK()
		origen := c.origen
		s.OCX = func(config.Config) EstadoOCX {
			return EstadoOCX{FaltanEnSysWOW64: []string{"MSCOMCTL.OCX"}, FaltanEnOrigen: origen}
		}
		r := buscar(t, Run(config.Default(), "", s), ReqOCX)
		if r.Estado == EstadoOK {
			t.Errorf("%s: con OCX faltantes no puede estar OK", c.nombre)
		}
		if r.Estado == EstadoFalta {
			t.Errorf("%s: no debe bloquear", c.nombre)
		}
		if !strings.Contains(r.Arreglo, c.espera) {
			t.Errorf("%s: el arreglo debería mencionar %q, dice: %s", c.nombre, c.espera, r.Arreglo)
		}
		if strings.Contains(r.Arreglo, c.prohibe) {
			t.Errorf("%s: el arreglo no debería decir %q, dice: %s", c.nombre, c.prohibe, r.Arreglo)
		}
	}

	if r := buscar(t, Run(config.Default(), "", sondasOK()), ReqOCX); !r.OK() {
		t.Errorf("sin OCX faltantes debería estar OK, está %v", r.Estado)
	}
}

func TestCumpleExigeTodos(t *testing.T) {
	s := sondasOK()
	s.Motor = motor(EstadoFalta, "connection refused")
	rs := Run(config.Default(), "", s)

	if !Cumple(rs, ReqAdmin) {
		t.Error("admin está OK, Cumple(admin) debería dar true")
	}
	if Cumple(rs, ReqAdmin, ReqMotor) {
		t.Error("el motor falta, Cumple(admin, motor) debería dar false")
	}
	if Cumple(rs, ID("sonda-que-no-corrio")) {
		t.Error("un ID que no está en la lista no puede darse por cumplido")
	}
	if !Cumple(rs) {
		t.Error("sin IDs no hay nada que exigir")
	}
}

func TestResumenListaAcorta(t *testing.T) {
	// El runtime de Crystal son 41 DLLs: listarlas todas tapa el resto del
	// checklist.
	if got := resumenLista([]string{"a", "b"}); got != "a, b" {
		t.Errorf("lista corta: got %q", got)
	}
	got := resumenLista([]string{"a", "b", "c", "d", "e", "f"})
	if got != "a, b, c, d (y 2 más)" {
		t.Errorf("lista larga: got %q", got)
	}
	if r := resumenLista([]string{"a", "b", "c", "d"}); strings.Contains(r, "más") {
		t.Errorf("justo en el tope no debería acortar: %q", r)
	}
}

func TestMarcaPorEstado(t *testing.T) {
	casos := []struct {
		e    Estado
		want string
	}{
		{EstadoOK, "✓"},
		{EstadoAviso, "!"},
		{EstadoFalta, "✗"},
	}
	for _, c := range casos {
		if got := c.e.Marca(); got != c.want {
			t.Errorf("Estado(%d).Marca(): got %q, want %q", c.e, got, c.want)
		}
	}
}

// motor existe porque la sonda de motor devuelve tres estados (OK / falla / no se
// pudo verificar) y casi todos los tests solo necesitan decir "anda" o "no anda".
func motor(e Estado, detalle string) func(string) (Estado, string) {
	return func(string) (Estado, string) { return e, detalle }
}

// Con app_dir sin configurar, el requisito tiene que pedir la carpeta. Un título que
// diga "Archivos de SIDC en " y un arreglo que mande a copiar a una carpeta vacía dejan
// al operador sin saber qué hacer: la carpeta de SIDC la tiene que decir él.
func TestAppDirVacioPideLaCarpeta(t *testing.T) {
	cfg := config.Default()
	cfg.AppDir = ""
	// La sonda real devuelve "app_dir sin configurar" cuando la ruta está vacía.
	s := sondasOK()
	s.AppFiles = func(dir string) []string {
		if dir == "" {
			return []string{"app_dir sin configurar"}
		}
		return nil
	}

	r := buscar(t, Run(cfg, "", s), ReqApp)
	if r.Estado == EstadoOK {
		t.Fatal("sin app_dir no puede estar OK")
	}
	if !strings.Contains(r.Titulo, "sin configurar") {
		t.Errorf("el título tiene que decir que falta configurar la carpeta, dice: %s", r.Titulo)
	}
	for _, quiero := range []string{"--app-dir", "perfil"} {
		if !strings.Contains(r.Arreglo, quiero) {
			t.Errorf("el arreglo tiene que nombrar %q, dice: %s", quiero, r.Arreglo)
		}
	}
	// app_dir vacío traba Setup App: sin los archivos de SIDC no hay qué configurar.
	var trabaSetupApp bool
	for _, id := range BloqueantesDe(EtapaSetupApp) {
		if id == ReqApp {
			trabaSetupApp = true
		}
	}
	if !trabaSetupApp {
		t.Error("app_dir sin configurar tiene que trabar Setup App, no solo avisar")
	}
}

// Con app_dir configurado, el requisito nombra la carpeta real: es la única forma de que
// el operador vea que Aegis está midiendo la carpeta que él quiso.
func TestAppDirConfiguradoNombraLaCarpeta(t *testing.T) {
	cfg := config.Default()
	cfg.AppDir = `D:\SIDC`
	s := sondasOK()
	s.AppFiles = func(dir string) []string { return []string{"exe: " + dir + `\falta.exe`} }

	r := buscar(t, Run(cfg, "", s), ReqApp)
	if !strings.Contains(r.Titulo, `D:\SIDC`) {
		t.Errorf("el título tiene que nombrar la carpeta, dice: %s", r.Titulo)
	}
	if !strings.Contains(r.Detalle, `D:\SIDC`) {
		t.Errorf("el detalle tiene que mostrar qué falta en la carpeta, dice: %s", r.Detalle)
	}
	if !strings.Contains(r.Arreglo, `D:\SIDC`) {
		t.Errorf("el arreglo tiene que decir a dónde copiar, dice: %s", r.Arreglo)
	}
}

// F7d: el respaldo dejo de ser un archivo que se copia "de la PC vieja" sin mas. Ahora
// viaja como asset del Release, y el arreglo tiene que decir como conseguirlo o el
// operador lee donde dejarlo pero no de donde sacarlo.
func TestElArregloDelRespaldoDiceDeDondeBajarlo(t *testing.T) {
	cfg := config.Default()
	s := sondasOK()
	s.Backup = func(config.Config) (string, error) { return "", errors.New("no hay .bak") }

	r := buscar(t, Run(cfg, "", s), ReqBackup)
	if r.Estado != EstadoFalta {
		t.Fatalf("estado = %v, quiero EstadoFalta", r.Estado)
	}
	for _, esperado := range []string{"aegis bak", config.DirBackups()} {
		if !strings.Contains(r.Arreglo, esperado) {
			t.Errorf("el arreglo no nombra %q: %s", esperado, r.Arreglo)
		}
	}
}
