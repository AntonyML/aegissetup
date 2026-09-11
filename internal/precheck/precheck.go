// © Antony Monge López — Costa Rica — Céd. 604700548
//
// Package precheck arma la lista ordenada de requisitos que una PC necesita
// para instalar SIDC. Es diagnóstico puro: no modifica nada.
//
// El orden de la lista es el orden en que conviene resolverlos, no un orden
// alfabético ni de importancia: primero lo que desbloquea todo lo demás
// (permisos, máquina) y al final lo que depende de los pasos previos.
//
// Las sondas se inyectan a propósito. Sin eso habría que ser administrador,
// tener Docker, tener SysWOW64 y un SQL Server vivo para poder correr un test.
package precheck

import (
	"strconv"
	"strings"

	"aegis-setup/internal/config"
)

// ID identifica cada requisito. Es estable: el TUI decide con esto qué acción
// desbloquea cada uno.
type ID string

const (
	ReqAdmin   ID = "admin"
	ReqMaquina ID = "maquina"
	ReqDocker  ID = "docker"
	ReqMotor   ID = "motor"
	ReqODBC    ID = "odbc"
	ReqBackup  ID = "backup"
	ReqBase    ID = "base"
	ReqDSN     ID = "dsn"
	ReqCrystal ID = "crystal"
	ReqOCX     ID = "ocx"
	ReqApp     ID = "app"
)

// Etapa es un paso del flujo de instalación. El precheck no conoce el menú del
// TUI a propósito: el TUI traduce sus acciones a estas etapas y la CLI imprime
// las mismas, así los dos dicen lo mismo de la misma máquina.
type Etapa string

const (
	EtapaSetupDB  Etapa = "Setup DB"
	EtapaSetupApp Etapa = "Setup App"
	EtapaInstall  Etapa = "Instalación completa"
)

// ordenDeResolucion es el orden en que se muestran los requisitos y en el que
// conviene resolverlos. Es una lista y no un mapa para que el orden sea igual en
// cada corrida: un checklist que se reordena solo no se puede seguir.
var ordenDeResolucion = []ID{
	ReqAdmin, ReqMaquina, ReqDocker, ReqMotor, ReqODBC,
	ReqBackup, ReqBase, ReqDSN, ReqCrystal, ReqOCX, ReqApp,
}

// trabaPorRequisito dice qué etapas quedan trabadas por cada requisito. Es la
// fuente única de la puerta: la leen la puerta del TUI y el checklist de la CLI,
// así no pueden discrepar sobre qué desbloquea qué.
//
// Tres decisiones que no son obvias:
//
//   - La base y el DSN no traban nada: son el resultado del flujo, no su
//     entrada. Exigirlos antes sería circular ("no puedo crear la base porque
//     todavía no existe la base").
//   - Los OCX tampoco: los instala el propio Setup App, así que trabarlos
//     dejaría al operador sin ninguna acción que los resuelva.
//   - Crystal sí traba la instalación completa pero no Setup App, que es quien
//     lo instala.
var trabaPorRequisito = map[ID][]Etapa{
	ReqAdmin:   {EtapaSetupDB, EtapaSetupApp, EtapaInstall},
	ReqMaquina: {EtapaSetupDB, EtapaSetupApp, EtapaInstall},
	ReqDocker:  {EtapaSetupDB, EtapaInstall},
	ReqMotor:   {EtapaSetupDB, EtapaInstall},
	ReqODBC:    {EtapaInstall},
	ReqBackup:  {EtapaSetupDB, EtapaInstall},
	ReqCrystal: {EtapaInstall},
	ReqApp:     {EtapaSetupApp, EtapaInstall},
	ReqOCX:     nil,
	ReqBase:    nil,
	ReqDSN:     nil,
}

// Traba dice qué etapas quedan trabadas por un requisito.
func Traba(id ID) []Etapa { return trabaPorRequisito[id] }

// BloqueantesDe devuelve los requisitos que impiden empezar esa etapa, en orden
// de resolución.
func BloqueantesDe(e Etapa) []ID {
	var out []ID
	for _, id := range ordenDeResolucion {
		for _, t := range trabaPorRequisito[id] {
			if t == e {
				out = append(out, id)
				break
			}
		}
	}
	return out
}

// Estado es el veredicto de un requisito.
type Estado int

const (
	// EstadoOK: cumple.
	EstadoOK Estado = iota
	// EstadoFalta: no cumple y hay que resolverlo antes de lo que dependa de él.
	EstadoFalta
	// EstadoAviso: no cumple, pero no bloquea nada: se resuelve solo en un paso
	// posterior o es una degradación tolerable.
	EstadoAviso
)

// Marca es el símbolo de una línea del checklist.
func (e Estado) Marca() string {
	switch e {
	case EstadoOK:
		return "✓"
	case EstadoAviso:
		return "!"
	}
	return "✗"
}

// Requisito es una línea del checklist.
type Requisito struct {
	ID     ID
	Titulo string
	// Motivo explica por qué importa: qué se rompe si falta. El operador no
	// tiene por qué saber que el DSN de VB6 es de 32 bits.
	Motivo  string
	Estado  Estado
	Detalle string
	// Arreglo dice exactamente qué hacer. Vacío cuando Estado es EstadoOK.
	Arreglo string
	// Traba lista las etapas del flujo que no se pueden empezar sin este
	// requisito. Queda vacío en los que no traban nada.
	Traba []Etapa
}

// OK indica si el requisito está cumplido.
func (r Requisito) OK() bool { return r.Estado == EstadoOK }

// EstadoOCX separa "faltan en SysWOW64" de "no hay de dónde copiarlos". La
// diferencia decide el veredicto: si están en el origen, el faltante se
// resuelve solo al correr Setup App; si no, el operador tiene que traerlos.
type EstadoOCX struct {
	FaltanEnSysWOW64 []string
	FaltanEnOrigen   []string
}

// Sondas son las verificaciones contra el sistema. Cada una devuelve el
// veredicto y un detalle corto para mostrar.
type Sondas struct {
	Admin    func() (bool, string)
	Maquina  func() (bool, string)
	Docker   func() (bool, string)
	Motor    func(server string) (Estado, string)
	ODBC     func(driver string) (bool, string)
	Base     func(cfg config.Config, appPass string) (bool, string)
	DSN      func(name string) (bool, string)
	Crystal  func() []string
	OCX      func(cfg config.Config) EstadoOCX
	Backup   func(cfg config.Config) (string, error)
	AppFiles func(dir string) []string
}

// Run evalúa todos los requisitos en orden de resolución.
func Run(cfg config.Config, appPass string, s Sondas) []Requisito {
	var out []Requisito

	ok, det := s.Admin()
	out = append(out, veredicto(ReqAdmin, "Permisos de administrador",
		"Sin elevación no se registran los OCX, ni se escribe el DSN, ni se restaura la base.",
		ok, det,
		"Apretá [E] en el menú para pedir permisos y relanzar Aegis elevado, o cerrá Aegis y abrilo con click derecho → Ejecutar como administrador."))

	ok, det = s.Maquina()
	out = append(out, veredicto(ReqMaquina, "Windows de 64 bits con SysWOW64",
		"SIDC es VB6 de 32 bits: los OCX y el DSN viven en el subsistema de 32 bits.",
		ok, det,
		"Esta PC no tiene SysWOW64, así que no puede correr SIDC. Verificá que sea Windows de 64 bits."))

	// Docker solo aplica al perfil de pruebas: en prod la base corre en el
	// motor del servidor, no en un contenedor.
	if cfg.DbMode == config.DbDocker {
		ok, det = s.Docker()
		out = append(out, veredicto(ReqDocker, "Docker en marcha",
			"El perfil de pruebas corre SQL Server 2019 en un contenedor.",
			ok, det,
			"Instalá Docker Desktop y levantá el motor: docker compose -f docker/docker-compose.yml up -d"))
	}

	estadoMotor, detMotor := s.Motor(cfg.Server)
	out = append(out, porEstado(ReqMotor, "Motor SQL alcanzable en "+cfg.Server,
		"Sin conexión al motor no se puede restaurar la base ni consultarla.",
		estadoMotor, detMotor, arregloMotor(cfg)))

	ok, det = s.ODBC(cfg.Driver)
	out = append(out, veredicto(ReqODBC, "Driver ODBC: "+cfg.Driver,
		"El DSN SIDC_SQL apunta a este driver; sin él la app no abre la base.",
		ok, det, arregloODBC(cfg.Driver)))

	bak, errBak := s.Backup(cfg)
	out = append(out, veredicto(ReqBackup, "Respaldo .bak disponible",
		"Setup DB restaura este respaldo para crear la base "+cfg.Database+".",
		errBak == nil, detalleDeError(bak, errBak),
		"Dejá el .bak de SIDC en "+config.DirBackups()+" (o al lado del ejecutable)."))

	ok, det = s.Base(cfg, appPass)
	out = append(out, veredicto(ReqBase, "Base "+cfg.Database+" en el motor",
		"Es la base que consulta la app. Si todavía no existe, falta correr Setup DB.",
		ok, det,
		"Corré Setup DB (menú [1]) para restaurarla desde el .bak."))

	ok, det = s.DSN(cfg.DsnName)
	out = append(out, veredicto(ReqDSN, "DSN 32-bit "+cfg.DsnName,
		"SIDC busca la base por nombre de DSN, no por cadena de conexión.",
		ok, det,
		"Corré Setup App (menú [2]): crea el System DSN de 32 bits."))

	if faltan := s.Crystal(); len(faltan) > 0 {
		out = append(out, veredicto(ReqCrystal, "Runtime de Crystal Reports",
			"Los 57 reportes de SIDC se generan con este runtime.",
			false, "faltan en SysWOW64: "+resumenLista(faltan),
			"Corré Setup App (menú [2]) para instalarlo desde el runtime embebido (43 archivos, 24 MB, viajan dentro de Aegis.exe)."))
	} else {
		out = append(out, veredicto(ReqCrystal, "Runtime de Crystal Reports",
			"Los 57 reportes de SIDC se generan con este runtime.",
			true, "el runtime embebido ya está en SysWOW64", ""))
	}

	// Los OCX no bloquean nada: si faltan en SysWOW64 pero están en el origen,
	// el propio Setup App los copia y registra. Bloquear acá dejaría al operador
	// sin forma de arreglarlo desde el menú.
	ocx := s.OCX(cfg)
	switch {
	case len(ocx.FaltanEnSysWOW64) == 0:
		out = append(out, veredicto(ReqOCX, "Controles OCX de VB6",
			"Sin los OCX registrados la app falla con error 339 al abrir pantallas.",
			true, "los 11 controles están en SysWOW64", ""))
	case len(ocx.FaltanEnOrigen) == 0:
		out = append(out, aviso(ReqOCX, "Controles OCX de VB6",
			"Sin los OCX registrados la app falla con error 339 al abrir pantallas.",
			"faltan registrar en SysWOW64: "+resumenLista(ocx.FaltanEnSysWOW64),
			"Se resuelve solo: corré Setup App (menú [2]) y los copia y registra."))
	default:
		// Faltan y no hay origen: no se bloquea nada a propósito. Bloquear dejaría
		// al operador sin ninguna acción del menú que lo arregle.
		out = append(out, aviso(ReqOCX, "Controles OCX de VB6",
			"Sin los OCX registrados la app falla con error 339 al abrir pantallas.",
			"faltan en SysWOW64 y tampoco están en el origen: "+resumenLista(ocx.FaltanEnOrigen),
			"Copiá los OCX de la PC vieja a "+cfg.LegacyDir+" y corré Setup App (menú [2])."))
	}

	if faltan := s.AppFiles(cfg.AppDir); len(faltan) > 0 {
		out = append(out, veredicto(ReqApp, "Archivos de SIDC en "+cfg.AppDir,
			"Es el sistema que se está instalando: sin sus archivos no hay nada que configurar.",
			false, "falta: "+resumenLista(faltan),
			"Copiá la carpeta de SIDC desde la PC vieja a "+cfg.AppDir+"."))
	} else {
		out = append(out, veredicto(ReqApp, "Archivos de SIDC en "+cfg.AppDir,
			"Es el sistema que se está instalando: sin sus archivos no hay nada que configurar.",
			true, "exe + reportes + fotos presentes", ""))
	}

	return out
}

// Faltantes devuelve los requisitos que no se cumplen, traben o no.
func Faltantes(rs []Requisito) []Requisito {
	var out []Requisito
	for _, r := range rs {
		if r.Estado == EstadoFalta {
			out = append(out, r)
		}
	}
	return out
}

// Bloqueantes devuelve los requisitos que hoy impiden empezar alguna etapa.
//
// No es lo mismo que Faltantes: en una PC limpia la base y el DSN faltan, y eso
// no traba nada porque son el resultado del flujo y no su entrada. Contarlos como
// bloqueos diría "2 bloquean" cuando en realidad el operador puede arrancar.
func Bloqueantes(rs []Requisito) []Requisito {
	var out []Requisito
	for _, r := range rs {
		if r.Estado == EstadoFalta && len(r.Traba) > 0 {
			out = append(out, r)
		}
	}
	return out
}

// Etapas lista las etapas para los mensajes.
func Etapas(es []Etapa) string {
	xs := make([]string, 0, len(es))
	for _, e := range es {
		xs = append(xs, string(e))
	}
	return strings.Join(xs, ", ")
}

// Buscar devuelve el requisito con ese ID.
func Buscar(rs []Requisito, id ID) (Requisito, bool) {
	for _, r := range rs {
		if r.ID == id {
			return r, true
		}
	}
	return Requisito{}, false
}

// Cumple indica si todos los IDs pedidos están OK. Un ID que no está en la
// lista se considera no cumplido: si la sonda no corrió, no se asume que está bien.
func Cumple(rs []Requisito, ids ...ID) bool {
	for _, id := range ids {
		r, ok := Buscar(rs, id)
		if !ok || !r.OK() {
			return false
		}
	}
	return true
}

func veredicto(id ID, titulo, motivo string, ok bool, detalle, arreglo string) Requisito {
	r := Requisito{ID: id, Titulo: titulo, Motivo: motivo, Detalle: detalle, Traba: Traba(id)}
	if ok {
		r.Estado = EstadoOK
		return r
	}
	r.Estado = EstadoFalta
	r.Arreglo = arreglo
	return r
}

func aviso(id ID, titulo, motivo, detalle, arreglo string) Requisito {
	return Requisito{ID: id, Titulo: titulo, Motivo: motivo, Estado: EstadoAviso, Detalle: detalle, Arreglo: arreglo, Traba: Traba(id)}
}

// porEstado arma un requisito a partir de una sonda de tres valores. Existe porque
// algunas sondas distinguen "no se pudo verificar" de "falló", y esa diferencia no
// se puede meter en un bool: un aviso explica y no traba, un fallo traba.
func porEstado(id ID, titulo, motivo string, e Estado, detalle, arreglo string) Requisito {
	switch e {
	case EstadoOK:
		return veredicto(id, titulo, motivo, true, detalle, "")
	case EstadoAviso:
		return aviso(id, titulo, motivo, detalle, arreglo)
	default:
		return veredicto(id, titulo, motivo, false, detalle, arreglo)
	}
}

// arregloMotor cambia según el ambiente: el motor de pruebas vive en un
// contenedor que hay que levantar, el de prod es un servicio que ya debería estar.
func arregloMotor(cfg config.Config) string {
	if cfg.DbMode == config.DbDocker {
		return "Levantá el motor de pruebas: docker compose -f docker/docker-compose.yml up -d"
	}
	return "En prod revisá que el servicio SQL Server esté corriendo y que el nombre del servidor sea el correcto."
}

// arregloODBC distingue el driver de dev (ODBC Driver 17, instalable desde
// Microsoft) del de prod (SQL Server, el legacy que ya trae Windows o el instalador).
func arregloODBC(driver string) string {
	if driver == "SQL Server" {
		return "El driver legacy 'SQL Server' viene con Windows o con el instalador de SIDC. Verificá en odbcad32.exe (32 bits) → Drives."
	}
	return "Instalá " + driver + " (msodbcsql.msi) y verificá que aparezca en odbcad32.exe de 32 bits."
}

// resumenLista acorta listas largas: 41 DLLs no se leen en pantalla.
func resumenLista(xs []string) string {
	const tope = 4
	if len(xs) <= tope {
		return strings.Join(xs, ", ")
	}
	return strings.Join(xs[:tope], ", ") + " (y " + strconv.Itoa(len(xs)-tope) + " más)"
}

func detalleDeError(detalle string, err error) string {
	if err != nil {
		return err.Error()
	}
	return detalle
}
