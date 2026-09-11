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
	Env       string `json:"env"`        // dev | prod
	DbMode    DbMode `json:"db_mode"`    // docker | local | server
	Server    string `json:"server"`     // ej dev: localhost,14333 | prod local: localhost | prod server: CONTABILIDAD o MI_SERVIDOR
	Database  string `json:"database"`   // siempre SIDC (el exe lo trae hardcodeado)
	DsnName   string `json:"dsn_name"`   // siempre SIDC_SQL
	Driver    string `json:"driver"`     // prod: SQL Server (legacy) | dev docker: ODBC Driver 17 for SQL Server o SQL Server
	UseWinAuth bool  `json:"use_win_auth"` // true en prod local/server con AD, false en docker
	SQLUser   string `json:"sql_user"`   // solo cuando UseWinAuth=false (dev/docker)
	// SQLPassword va por flag/env AEGIS_SQL_PASSWORD, nunca en el json en claro.
	Collation string `json:"collation"` // ej Modern_Spanish_CI_AS (debe igualar prod)
	Compat    int    `json:"compat"`    // 120 = SQL 2014 como CONTABILIDAD
	AppDir    string `json:"app_dir"`   // donde vive SIDC, ej C:\DEV\SIDC
	DockerDir string `json:"docker_dir"` // donde está el compose, ej C:\DEV\SIDC\docker-dev
	BackupDir string `json:"backup_dir"` // donde dejas el .bak, ej C:\DEV\SIDC\AegisSetup\assets\backups\sqlserver2014
	LegacyDir string `json:"legacy_dir"` // donde dejas los OCX de la PC vieja
}

// Default devuelve parámetros que replican prod CONTABILIDAD + dev Docker.
func Default() Config {
	return Config{
		Env:       "dev",
		DbMode:    DbDocker,
		Server:    "localhost,14333",
		Database:  "SIDC",
		DsnName:   "SIDC_SQL",
		Driver:    "ODBC Driver 17 for SQL Server",
		UseWinAuth: false,
		SQLUser:   "dev",
		Collation: "Modern_Spanish_CI_AS",
		Compat:    120,
		AppDir:    `C:\DEV\SIDC`,
		DockerDir: `C:\DEV\SIDC\docker-dev`,
		BackupDir: `C:\DEV\SIDC\AegisSetup\assets\backups\sqlserver2014`,
		LegacyDir: `C:\DEV\SIDC\AegisSetup\assets\legacy\ocx`,
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

// Validate chequea sentido antes de tocar SQL, registro o disco.
func (c Config) Validate() error {
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
	if c.AppDir == "" || c.BackupDir == "" {
		return fmt.Errorf("config: app_dir y backup_dir son obligatorios")
	}
	return nil
}
