// © Antony Monge López — Costa Rica — Céd. 604700548
package precheck

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"aegis-setup/assets"
	"aegis-setup/internal/config"
	"aegis-setup/internal/setup"

	_ "github.com/microsoft/go-mssqldb"
)

// SondasReales son las verificaciones contra la máquina de verdad. Lo que
// depende del sistema operativo (permisos, SysWOW64, driver ODBC) está en
// probes_windows.go / probes_other.go para que esto compile en Linux, donde
// corre parte del desarrollo.
func SondasReales() Sondas {
	return Sondas{
		Admin:   esAdmin,
		Maquina: maquinaApta,
		Docker:  dockerEnMarcha,
		Motor:   motorAlcanzable,
		ODBC:    driverODBC,
		Base:    basePresente,
		DSN:     dsnPresente,
		Crystal: setup.CheckCrystal,
		OCX: func(cfg config.Config) EstadoOCX {
			// Los controles viajan dentro del propio binario, así que "no hay de dónde
			// copiarlos" pasó a ser un caso de emergencia: un EXE mal armado, sin el
			// kit embebido.
			origen := setup.OrigenOCX{Binario: assets.OCX(), Carpeta: cfg.LegacyDir}
			return EstadoOCX{
				FaltanEnSysWOW64: setup.CheckOCX(),
				FaltanEnOrigen:   setup.FaltanOCXEn(origen),
			}
		},
		Backup:   setup.FindNewestBakCfg,
		AppFiles: setup.CheckAppFiles,
	}
}

// motorAlcanzable abre el puerto del motor. Es solo TCP a propósito: si acá se
// intentara conectar con credenciales, un fallo de login se leería como "el
// motor no está", que manda al operador a arreglar lo que no está roto.
//
// Devuelve tres estados porque hay un caso que no es ni OK ni fallo: una instancia
// con nombre ("localhost\SQLEXPRESS", el default de Express) negocia el puerto por
// SQL Browser, así que abrir un puerto fijo no prueba nada. Ahí se avisa y quien
// decide de verdad es la sonda de la base, que sí se conecta con el driver.
func motorAlcanzable(server string) (Estado, string) {
	noVerificable := "instancia con nombre: el puerto lo negocia SQL Browser, no se puede sondear por TCP"
	if strings.Contains(server, "\\") {
		return EstadoAviso, noVerificable
	}
	host := setup.HostPuerto(server)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	var d net.Dialer
	c, err := d.DialContext(ctx, "tcp", host)
	if err != nil {
		return EstadoFalta, err.Error()
	}
	defer c.Close()
	return EstadoOK, "puerto abierto en " + host
}

// dockerEnMarcha pregunta por la versión del servidor, no por la del cliente:
// docker instalado pero apagado es el caso normal y hay que distinguirlo.
func dockerEnMarcha() (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Version}}").Output()
	if err != nil {
		return false, "docker no responde (¿está instalado y el motor encendido?)"
	}
	return true, "servidor " + strings.TrimSpace(string(out))
}

// basePresente se conecta a la base de la app y lee su nivel de compatibilidad.
// Usa la misma cadena que el check, para que el checklist y la verificación no
// puedan decir cosas distintas de la misma base.
func basePresente(cfg config.Config, appPass string) (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := sql.Open("sqlserver", setup.AppDSN(cfg, appPass))
	if err != nil {
		return false, err.Error()
	}
	defer db.Close()
	var name string
	var compat int
	err = db.QueryRowContext(ctx,
		"SELECT DB_NAME(), compatibility_level FROM sys.databases WHERE name = @p1", cfg.Database).Scan(&name, &compat)
	if err != nil {
		return false, "no se pudo consultar: " + err.Error()
	}
	return true, fmt.Sprintf("%s, compatibilidad %d", name, compat)
}

func dsnPresente(name string) (bool, string) {
	m, err := setup.ReadDSN(name)
	if err != nil {
		return false, "no existe el DSN " + name
	}
	server, database := m["Server"], m["Database"]
	if server == "" || database == "" {
		return false, "el DSN existe pero está incompleto"
	}
	return true, fmt.Sprintf("Server=%s Database=%s", server, database)
}
