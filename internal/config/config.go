// © Antony Monge López — Costa Rica — Céd. 604700548
// Package config define toda la parametrización de Aegis Setup.
// Convención: dev usa Docker (SQL Auth), prod usa lo que diga DbMode
// (local=SQL en la misma PC con Windows Auth, docker=Docker por red con
// SQL Auth, server=instancia remota xxxx con Windows o SQL Auth).
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// DbMode dice dónde vive la base en cada ambiente.
type DbMode string

const (
	DbDocker DbMode = "docker" // Docker por TCP (dev y opcional en prod)
	DbLocal  DbMode = "local"  // SQL en la misma PC, Windows Auth (prod clásico)
	DbServer DbMode = "server" // Servidor remoto xxxx (prod futuro)
)

// Config es el config.json de Aegis. Un json parcial hace override de defaults.
type Config struct {
	Env        string `json:"env"`          // dev | prod
	DbMode     DbMode `json:"db_mode"`      // docker | local | server
	Server     string `json:"server"`       // ej dev: localhost,14333 | prod local: localhost | prod server: CONTABILIDAD o MI_SERVIDOR
	Database   string `json:"database"`     // siempre SIDC (el exe lo trae hardcodeado)
	DsnName    string `json:"dsn_name"`     // siempre SIDC_SQL
	Driver     string `json:"driver"`       // prod: SQL Server (legacy) | dev docker: ODBC Driver 17 for SQL Server o SQL Server
	UseWinAuth bool   `json:"use_win_auth"` // true en prod local/server con AD, false en docker
	SQLUser    string `json:"sql_user"`     // solo cuando UseWinAuth=false (dev/docker)
	// SQLPassword va por flag/env AEGIS_SQL_PASSWORD, nunca en el json en claro.
	Collation string `json:"collation"`  // ej Modern_Spanish_CI_AS (debe igualar prod)
	Compat    int    `json:"compat"`     // 120 = SQL 2014 como CONTABILIDAD
	AppDir    string `json:"app_dir"`    // donde vive SIDC, ej C:\DEV\SIDC
	DockerDir string `json:"docker_dir"` // donde Aegis deja el compose del SQL de pruebas
	BackupDir string `json:"backup_dir"` // donde dejas el .bak, ej C:\DEV\SIDC\AegisSetup\assets\backups\sqlserver2014
	LegacyDir string `json:"legacy_dir"` // donde dejas los OCX de la PC vieja
}

// Default devuelve parámetros que replican prod CONTABILIDAD + dev Docker.
func Default() Config {
	return Config{
		Env:        "dev",
		DbMode:     DbDocker,
		Server:     "localhost,14333",
		Database:   "SIDC",
		DsnName:    "SIDC_SQL",
		Driver:     "ODBC Driver 17 for SQL Server",
		UseWinAuth: false,
		SQLUser:    "dev",
		Collation:  "Modern_Spanish_CI_AS",
		Compat:     120,
		AppDir:     "",
		DockerDir:  DirDocker(),
		// El respaldo vive en el árbol de máquina (F3): en una PC limpia no hay
		// repo del que sacarlo. En dev se sigue encontrando por BackupDirs().
		BackupDir: DirBackups(),
		// legacy_dir vacío = los OCX salen del propio binario. Es una carpeta extra
		// para el control que no venga adentro, no un requisito de la instalación.
		LegacyDir: "",
	}
}

var dbNameRe = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// Load lee config.json. Si no existe devuelve Default() sin error.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return Config{}, fmt.Errorf("config: leer %s: %w", path, err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("config: %s corrupto: %w", path, err)
	}
	merged, err := json.Marshal(cfg)
	if err != nil {
		return Config{}, fmt.Errorf("config: interno: %w", err)
	}
	var base map[string]json.RawMessage
	if err := json.Unmarshal(merged, &base); err != nil {
		return Config{}, fmt.Errorf("config: interno: %w", err)
	}
	for k, v := range raw {
		base[k] = v
	}
	merged, err = json.Marshal(base)
	if err != nil {
		return Config{}, fmt.Errorf("config: interno: %w", err)
	}
	if err := json.Unmarshal(merged, &cfg); err != nil {
		return Config{}, fmt.Errorf("config: %s inválido: %w", path, err)
	}
	// docker_dir la administra Aegis: el compose se extrae del propio EXE, así que la
	// carpeta tiene que ser una que exista de verdad en esta PC. Un config.json viejo puede
	// traer la carpeta del repo de quien programa Aegis —que en la PC destino no existe, y
	// con ella el arreglo del motor mandaría a correr un comando que no puede funcionar— o
	// una ruta relativa, que se resolvería contra el directorio de trabajo. En esos casos se
	// usa la carpeta del producto. Una carpeta propia que sí existe se respeta.
	if cfg.DockerDir == "" || !RutaAbsoluta(cfg.DockerDir) || !existeDir(cfg.DockerDir) {
		cfg.DockerDir = DirDocker()
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Save escribe config.json pretty.
func (c Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// AuthLabel devuelve una representación legible del método de autenticación configurado.
func (c Config) AuthLabel() string {
	if c.UseWinAuth {
		return "Windows Auth"
	}
	user := c.SQLUser
	if user == "" {
		user = "sidc"
	}
	return "SQL Auth (" + user + ")"
}

// Sanitize limpia espacios en blanco y saltos de línea de todos los campos de texto.
func (c *Config) Sanitize() {
	c.Env = strings.TrimSpace(c.Env)
	c.Server = strings.TrimSpace(c.Server)
	c.Database = strings.TrimSpace(c.Database)
	c.DsnName = strings.TrimSpace(c.DsnName)
	c.Driver = strings.TrimSpace(c.Driver)
	c.SQLUser = strings.TrimSpace(c.SQLUser)
	c.Collation = strings.TrimSpace(c.Collation)
	c.AppDir = strings.TrimSpace(c.AppDir)
	c.DockerDir = strings.TrimSpace(c.DockerDir)
	c.BackupDir = strings.TrimSpace(c.BackupDir)
	c.LegacyDir = strings.TrimSpace(c.LegacyDir)
}

// Validate chequea sentido antes de tocar SQL, registro o disco.
func (c *Config) Validate() error {
	c.Sanitize()
	if c.Env != "dev" && c.Env != "prod" {
		return fmt.Errorf("config: env debe ser dev|prod, recibí %q", c.Env)
	}
	switch c.DbMode {
	case DbDocker, DbLocal, DbServer:
	default:
		return fmt.Errorf("config: db_mode debe ser docker|local|server, recibí %q", c.DbMode)
	}
	if c.Server == "" {
		return fmt.Errorf("config: server vacío")
	}
	if !dbNameRe.MatchString(c.Database) {
		return fmt.Errorf("config: database %q inválido", c.Database)
	}
	if c.DsnName == "" {
		return fmt.Errorf("config: dsn_name vacío (el exe exige SIDC_SQL)")
	}
	if c.Driver == "" {
		return fmt.Errorf("config: driver vacío")
	}
	if !c.UseWinAuth && c.SQLUser == "" {
		return fmt.Errorf("config: sql_user vacío con use_win_auth=false")
	}
	if c.Compat != 110 && c.Compat != 120 && c.Compat != 130 && c.Compat != 140 && c.Compat != 150 && c.Compat != 160 {
		return fmt.Errorf("config: compat %d no válido (110|120|130|140|150|160)", c.Compat)
	}
	// backup_dir es opcional: el aprovisionamiento de terminales no depende de .bak
	// (la gestión de respaldos corresponde a backup-agent).
	// app_dir puede estar vacío: es la PC recién instalada donde todavía nadie dijo
	// dónde está SIDC, y quien lo pide es el perfil del TUI (o --app-dir). Pero si
	// viene, tiene que ser absoluta: una ruta relativa se resuelve contra el directorio
	// de trabajo del proceso, así que la misma config mediría carpetas distintas según
	// desde dónde se lance Aegis.
	if c.AppDir != "" && !RutaAbsoluta(c.AppDir) {
		return fmt.Errorf("config: app_dir %q tiene que ser una ruta absoluta (ej. C:\\SIDC)", c.AppDir)
	}
	return nil
}

// ExisteDir dice si la ruta está en disco y es una carpeta.
func ExisteDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func existeDir(p string) bool { return ExisteDir(p) }

// RutaAbsoluta reconoce las rutas de Windows (C:\, C:/, \\\\servidor\recurso) sin
// depender del sistema donde corre el proceso: el instalador es de Windows, pero parte
// del desarrollo y de las pruebas corre en Linux, y ahí filepath.IsAbs diría que
// "C:\\SIDC" no es absoluta.
func RutaAbsoluta(p string) bool {
	if strings.HasPrefix(p, `\\`) || strings.HasPrefix(p, "/") {
		return true
	}
	return len(p) >= 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/')
}
