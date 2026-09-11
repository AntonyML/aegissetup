// © Antony Monge López — Costa Rica — Céd. 604700548
// Package setup implementa los dos setups: DB y App.
package setup

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"aegis-setup/internal/config"

	_ "github.com/microsoft/go-mssqldb"
)

// FindNewestBak devuelve el .bak más nuevo del BackupDir.
func FindNewestBak(dir string) (string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("backup_dir %s: %w", dir, err)
	}
	var cands []string
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(e.Name()), ".bak") {
			cands = append(cands, filepath.Join(dir, e.Name()))
		}
	}
	if len(cands) == 0 {
		return "", fmt.Errorf("no hay .bak en %s (bajá el respaldo del Release y dejalo ahí: corré aegis bak)", dir)
	}
	sort.Slice(cands, func(i, j int) bool {
		fi, _ := os.Stat(cands[i])
		fj, _ := os.Stat(cands[j])
		if fi == nil || fj == nil {
			return cands[i] < cands[j]
		}
		return fi.ModTime().After(fj.ModTime())
	})
	return cands[0], nil
}

// FindNewestBakCfg busca el .bak en los directorios candidatos de la config.
// El orden lo decide config.BackupDirs: primero el configurado (ProgramData en
// una PC instalada) y después el layout viejo junto al binario, para que el
// flujo dev siga encontrando el respaldo del repo sin configurar nada.
func FindNewestBakCfg(cfg config.Config) (string, error) {
	dirs := config.BackupDirs(cfg, exeDir())
	var sinRespaldo []string
	for _, dir := range dirs {
		bak, err := FindNewestBak(dir)
		if err == nil {
			return bak, nil
		}
		sinRespaldo = append(sinRespaldo, dir)
	}
	return "", fmt.Errorf("no hay .bak en ninguna de estas carpetas: %s (bajá el respaldo del Release y dejalo ahí: corré aegis bak)", strings.Join(sinRespaldo, ", "))
}

// exeDir devuelve la carpeta del binario. Se usa para reconocer el layout viejo
// de respaldos, que vivía junto al ejecutable.
func exeDir() string {
	if exe, err := os.Executable(); err == nil {
		if dir := filepath.Dir(exe); dir != "" {
			return dir
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

// dockerBakMount es el directorio de respaldos *dentro* del motor SQL cuando la
// base corre en Docker. Tiene que coincidir con el bind :ro de
// AegisSetup/docker/docker-compose.yml; si allá cambia, acá también.
const dockerBakMount = "/var/opt/mssql/backup"

// isEnginePath reconoce una ruta que ya es la que ve el motor SQL, no la de
// Windows. El operador puede pasarla directo por --bak.
func isEnginePath(cfg config.Config, bakPath string) bool {
	return cfg.DbMode == config.DbDocker && strings.HasPrefix(bakPath, dockerBakMount+"/")
}

// bakPathForEngine traduce la ruta local del .bak a la que ve el motor SQL.
// En Docker el motor corre en un contenedor Linux que no tiene C:\: el respaldo
// se monta :ro en dockerBakMount, así que una ruta Windows no le dice nada y el
// FILELISTONLY falla antes de restaurar.
func bakPathForEngine(cfg config.Config, bakPath string) string {
	if cfg.DbMode != config.DbDocker || isEnginePath(cfg, bakPath) {
		return bakPath
	}
	return dockerBakMount + "/" + filepath.Base(bakPath)
}

// localStatNeeded dice si la ruta del .bak hay que validarla contra el sistema
// de archivos local. En Docker el operador puede pasar la ruta que ve el motor,
// que por definición no existe en Windows: validarla dejaría --bak inutilizable.
func localStatNeeded(cfg config.Config, bakPath string) bool {
	return !isEnginePath(cfg, bakPath)
}

// PuertoPorDefecto es el de SQL Server cuando el server no trae puerto. Es el mismo
// que asume el driver, y tiene que seguir siéndolo: si acá dijéramos un puerto y el
// driver usara otro, el checklist hablaría de un puerto y la conexión de otro.
const PuertoPorDefecto = "1433"

// HostPuerto convierte el "host,puerto" que usa ODBC al "host:puerto" que hablan la
// red y los drivers.
//
// Tres casos, y los tres importan:
//
//   - "localhost,14333" -> "localhost:14333"
//   - "localhost" o "CONTABILIDAD" -> le agrega :1433. Sin esto, net.Dial falla con
//     "missing port in address" y una máquina sana queda en rojo.
//   - "localhost\SQLEXPRESS" -> se deja tal cual. Una instancia con nombre negocia
//     el puerto por SQL Browser; forzarle el 1433 la rompe.
func HostPuerto(server string) string {
	s := strings.Replace(server, ",", ":", 1)
	if strings.Contains(s, "\\") || tienePuerto(s) {
		return s
	}
	return s + ":" + PuertoPorDefecto
}

// tienePuerto distingue "localhost:1433" de "localhost". Un ':' seguido de algo que
// no sea puerto no cuenta.
func tienePuerto(s string) bool {
	i := strings.LastIndex(s, ":")
	if i < 0 || i == len(s)-1 {
		return false
	}
	for _, c := range s[i+1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// adminDSN arma conexión SA/master contra el Server configurado.
// ODBC usa "host,puerto" pero go-mssqldb exige "host:puerto".
func adminDSN(cfg config.Config, saPass string) string {
	srv := HostPuerto(cfg.Server)
	return fmt.Sprintf("sqlserver://sa:%s@%s?database=master&dial+timeout=15&encrypt=disable", urlEscape(saPass), srv)
}

func urlEscape(s string) string {
	r := strings.NewReplacer(":", "%3A", "@", "%3A", "/", "%2F", "?", "%3F", "#", "%23", " ", "%20", "*", "%2A")
	return r.Replace(s)
}

// AppDSN arma la conexión con la que la app (o el check) lee la base: login de
// app en dev/docker, Windows Auth en prod. La comparten el check y el precheck
// a propósito: si cada uno armara la suya, un cambio en uno solo haría que el
// checklist y la verificación dijeran cosas distintas de la misma base.
func AppDSN(cfg config.Config, appPass string) string {
	srv := HostPuerto(cfg.Server)
	if cfg.UseWinAuth || appPass == "" {
		return fmt.Sprintf("sqlserver://%s?database=%s&dial+timeout=10&encrypt=disable&trusted+connection=yes", srv, cfg.Database)
	}
	return fmt.Sprintf("sqlserver://%s:%s@%s?database=%s&dial+timeout=10&encrypt=disable", cfg.SQLUser, urlEscape(appPass), srv, cfg.Database)
}

// SetupDB restaura el .bak más nuevo como cfg.Database, fija compat,
// crea login/usuario de app (solo SQL Auth) y corre CHECKDB.
// saPass viene de env AEGIS_SA_PASSWORD. appPass de AEGIS_SQL_PASSWORD.
// En prod local/server con Windows Auth igual se restaura vía SA o vía
// trusted (si saPass vacío se usa trusted).
func SetupDB(ctx context.Context, cfg config.Config, bakPath, saPass, appPass string, out func(string)) error {
	if bakPath == "" {
		var err error
		bakPath, err = FindNewestBakCfg(cfg)
		if err != nil {
			return err
		}
	}
	out("BAK: " + bakPath)

	db, err := openAdmin(ctx, cfg, saPass)
	if err != nil {
		return err
	}
	defer db.Close()

	// El .bak de prod vive en el servidor; para Docker se monta :ro en
	// /var/opt/mssql/backup/. La ruta que se le manda al motor no es la de
	// Windows: en Docker hay que traducirla al punto de montaje del contenedor.
	if localStatNeeded(cfg, bakPath) {
		if _, err := os.Stat(bakPath); err != nil {
			return fmt.Errorf("no se puede leer %s: %w", bakPath, err)
		}
	}
	engineBak := bakPathForEngine(cfg, bakPath)
	if engineBak != bakPath {
		out("motor: " + engineBak)
	}

	logical, err := fileListOnly(ctx, db, engineBak)
	if err != nil {
		return fmt.Errorf("FILELISTONLY: %w (revisa que el .bak sea de %s y el servicio SQL pueda leerlo)", err, cfg.Database)
	}
	out(fmt.Sprintf("DATA=%s LOG=%s", logical.data, logical.log))

	restore := fmt.Sprintf(`RESTORE DATABASE [%s] FROM DISK = @p1 WITH REPLACE,
	  MOVE @p2 TO '/var/opt/mssql/data/%s.mdf',
	  MOVE @p3 TO '/var/opt/mssql/data/%s_log.ldf'`, cfg.Database, cfg.Database, cfg.Database)
	// En SQL Windows local las rutas Linux no aplican; el servidor decide.
	// Por eso primero intentamos rutas Linux (Docker) y si falla, con .mdf locales del DATA dir.
	if err := execQ(ctx, db, restore, engineBak, logical.data, logical.log); err != nil {
		// Fallback Windows: deja que SQL ponga los archivos en su DATA default.
		out("MOVE Linux falló, reintento con rutas default del servidor...")
		restore2 := fmt.Sprintf(`RESTORE DATABASE [%s] FROM DISK = @p1 WITH REPLACE`, cfg.Database)
		if err2 := execQ(ctx, db, restore2, engineBak); err2 != nil {
			return fmt.Errorf("RESTORE: %v / %v", err, err2)
		}
	}
	out("RESTORE OK")

	if err := execQ(ctx, db, fmt.Sprintf(`ALTER DATABASE [%s] SET COMPATIBILITY_LEVEL = %d`, cfg.Database, cfg.Compat)); err != nil {
		return fmt.Errorf("compat %d: %w", cfg.Compat, err)
	}
	if err := execQ(ctx, db, fmt.Sprintf(`DBCC CHECKDB ([%s]) WITH NO_INFOMSGS`, cfg.Database)); err != nil {
		return fmt.Errorf("CHECKDB: %w", err)
	}
	out("CHECKDB sin errores")

	if !cfg.UseWinAuth {
		if appPass == "" {
			return fmt.Errorf("falta AEGIS_SQL_PASSWORD para crear el login %s", cfg.SQLUser)
		}
		stmts := []string{
			fmt.Sprintf(`IF NOT EXISTS (SELECT * FROM sys.server_principals WHERE name = '%s') CREATE LOGIN [%s] WITH PASSWORD = '%s', DEFAULT_DATABASE = [%s], DEFAULT_LANGUAGE = us_english, CHECK_POLICY = OFF ELSE ALTER LOGIN [%s] WITH PASSWORD = '%s', DEFAULT_DATABASE = [%s], CHECK_POLICY = OFF`,
				esc(cfg.SQLUser), esc(cfg.SQLUser), escPass(appPass), esc(cfg.Database), esc(cfg.SQLUser), escPass(appPass), esc(cfg.Database)),
			fmt.Sprintf(`ALTER LOGIN [%s] ENABLE`, esc(cfg.SQLUser)),
			fmt.Sprintf(`USE [%s]; IF NOT EXISTS (SELECT * FROM sys.database_principals WHERE name = '%s') BEGIN CREATE USER [%s] FOR LOGIN [%s]; ALTER ROLE db_owner ADD MEMBER [%s]; END`,
				esc(cfg.Database), esc(cfg.SQLUser), esc(cfg.SQLUser), esc(cfg.SQLUser), esc(cfg.SQLUser)),
		}
		for _, s := range stmts {
			if err := execQ(ctx, db, s); err != nil {
				return fmt.Errorf("login app: %w", err)
			}
		}
		out("login app OK: " + cfg.SQLUser)
	} else {
		out("Windows Auth: no se crea login SQL (prod usa AD como CONTABILIDAD)")
	}

	var name string
	var level int
	var coll string
	err = db.QueryRowContext(ctx, `SELECT name, compatibility_level, collation_name FROM sys.databases WHERE name = @p1`, cfg.Database).Scan(&name, &level, &coll)
	if err != nil {
		return err
	}
	out(fmt.Sprintf("OK: %s compat=%d collation=%s", name, level, coll))
	return nil
}

type fileNames struct{ data, log string }

func fileListOnly(ctx context.Context, db *sql.DB, bak string) (fileNames, error) {
	rows, err := db.QueryContext(ctx, `RESTORE FILELISTONLY FROM DISK = @p1`, bak)
	if err != nil {
		return fileNames{}, err
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	var data, log string
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return fileNames{}, err
		}
		m := map[string]string{}
		for i, c := range cols {
			m[strings.ToLower(c)] = fmt.Sprint(vals[i])
		}
		typ := m["type"]
		logic := m["logicalname"]
		if logic == "" {
			logic = m["logical_name"]
		}
		switch strings.ToUpper(typ) {
		case "D":
			if data == "" {
				data = logic
			}
		case "L":
			if log == "" {
				log = logic
			}
		}
	}
	if data == "" || log == "" {
		return fileNames{}, fmt.Errorf("no se encontraron DATA/LOG en el .bak")
	}
	return fileNames{data, log}, rows.Err()
}

func openAdmin(ctx context.Context, cfg config.Config, saPass string) (*sql.DB, error) {
	var dsn string
	if saPass == "" {
		// Windows Auth (prod local/server con AD).
		srv := strings.Replace(cfg.Server, ",", ":", 1)
		dsn = fmt.Sprintf("sqlserver://%s?database=master&dial+timeout=15&encrypt=disable&trusted+connection=yes", srv)
	} else {
		dsn = adminDSN(cfg, saPass)
	}
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		return nil, err
	}
	db.SetConnMaxLifetime(time.Minute)
	ctx2, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx2); err != nil {
		db.Close()
		return nil, fmt.Errorf("conectar a %s: %w", cfg.Server, err)
	}
	return db, nil
}

func execQ(ctx context.Context, db *sql.DB, q string, args ...any) error {
	ctx2, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	_, err := db.ExecContext(ctx2, q, args...)
	return err
}

func esc(s string) string     { return strings.ReplaceAll(s, "]", "]]") }
func escPass(s string) string { return strings.ReplaceAll(s, "'", "''") }
