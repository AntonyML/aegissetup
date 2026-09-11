# AegisSetup

Instalador CLI de **SIDC** para N equipos, sin pasos manuales.

![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)
![Cobra](https://img.shields.io/badge/CLI-Cobra-FF6F61?logo=go&logoColor=white)
![Bubble Tea](https://img.shields.io/badge/TUI-Bubble_Tea-FF75B7)
![Plataforma](https://img.shields.io/badge/plataforma-Windows-0078D6?logo=windows&logoColor=white)

## Qué es

**AegisSetup** es una herramienta independiente que automatiza la instalación de
**SIDC** (*Sistema Integrado de Controles y Presupuesto*, la aplicación de escritorio
legacy VB6 + SQL Server) en cualquier cantidad de PCs, sin pasos manuales.

No forma parte de SIDC: es un proyecto aparte que trabaja **sobre** el proyecto
principal. Toma un respaldo `.bak`, lo restaura en SQL Server, prepara el entorno de
Windows que la app legacy necesita (DSN ODBC de 32 bits, controles OCX, runtime de
Crystal Reports) y verifica que App y base de datos se hablen antes de dar por
terminada la instalación.

## Características

- **Instalación desatendida** — un solo comando restaura la base, deja el entorno listo y verifica el resultado.
- **Multi-entorno con un mismo binario** — `dev` (Docker), `prod local`, `prod server`, sin recompilar la app.
- **Redirección por DSN** — el DSN es el único punto de conexión: cambiarlo apunta a otro servidor sin tocar el `.exe`.
- **TUI interactivo** — menú Bubble Tea que **ejecuta** el flujo (no solo lo documenta).
- **Configuración declarativa** — un `config.json` describe el entorno; los presets de la TUI lo generan por vos.
- **Verificación real** — `check` prueba TCP + SQL + DSN + archivos y falla con código de salida si algo no cierra.

## Requisitos

| Componente | Detalle |
| --- | --- |
| **Go** | 1.27 (para compilar) |
| **Windows** | Requerido para el flujo completo: DSN ODBC de 32 bits, registro OCX y SQL Server |
| **SQL Server** | 2014+ (prod: 2014 Express; dev: Docker) |
| **Docker** | Opcional, para la base de desarrollo |
| **Privilegios** | Administrador (el DSN se escribe en `HKLM` y los OCX en `SysWOW64`) |

## Compilar

```bash
cd AegisSetup
go build -o bin/aegis.exe ./cmd/aegis
```

El binario queda en `bin/aegis.exe` y lee `config.json` **junto a él** (o la ruta que le pases con `--config`).

## Uso

### Comandos

| Comando | Función |
| --- | --- |
| `setup-db` | Restaura el `.bak` como `SIDC`: collation, compat level, logins y `CHECKDB`. |
| `setup-app` | Crea el DSN `SIDC_SQL` de 32 bits, instala los OCX legacy y verifica exe/reportes. |
| `check` | Valida TCP + SQL + DSN + ficheros (App ⇄ DB). Solo lectura. |
| `dashboard` | Panel de estado no interactivo (env, DSN, DB, app). |
| `menu` | TUI interactivo (instalación completa, por etapas y presets). |
| `configure` | Genera el `config.json` inicial. |

Sin subcomando, `aegis` abre directamente la TUI (si hay terminal interactiva).

### Flags

```text
Globales
  --config string    ruta a config.json (default: junto al binario)
  --server string    server del preset "prod server" de la TUI

setup-db
  --bak string           .bak a restaurar (default: el más nuevo de backup_dir)
  --sa-password string   clave SA   (o env AEGIS_SA_PASSWORD; vacío = Windows Auth)
  --app-password string  clave del login SQL Auth (o env AEGIS_SQL_PASSWORD)

setup-app
  --app-password string  clave del login SQL Auth (o env AEGIS_SQL_PASSWORD)
  --save-pwd             guarda el PWD en el DSN (SOLO dev/docker, nunca prod)
  --patch-docker         genera _DOCKER.exe con UID/PWD embebidos (solo dev/docker)

check / dashboard
  --app-password string  clave del login SQL Auth (o env AEGIS_SQL_PASSWORD)

configure
  --env string       dev | prod
  --db-mode string   docker | local | server
  --server string    localhost,14333 | localhost | CONTABILIDAD | MI_SERVIDOR
  --out string       ruta de salida (default: config.json junto al binario)
```

### Códigos de salida

| Código | Significado |
| ---: | --- |
| `0` | OK |
| `1` | Error general |
| `2` | Error de configuración |
| `3` | `check` con fallos |

## Flujo rápido

```bash
cd AegisSetup
go build -o bin/aegis.exe ./cmd/aegis

./bin/aegis configure --env dev --db-mode docker   # 1. genera config.json
docker compose -f docker/docker-compose.yml up -d   # 2. base dev (SQL 2019, compat 120)
./bin/aegis setup-db                                 # 3. restaura el .bak como SIDC
./bin/aegis setup-app                                # 4. DSN + OCX + verificación
./bin/aegis check                                    # 5. valida App ⇄ DB
./bin/aegis menu                                     # o todo de un tirón desde la TUI
```

> Corré la terminal **como Administrador**: el DSN (`HKLM`) y los OCX (`SysWOW64`) lo exigen.

## Configuración

`config.json` describe el entorno. Un JSON **parcial** hace override de los defaults
(no hace falta escribirlo entero); `aegis configure` lo genera completo.

```json
{
  "env": "dev",
  "db_mode": "docker",
  "server": "localhost,14333",
  "database": "SIDC",
  "dsn_name": "SIDC_SQL",
  "driver": "ODBC Driver 17 for SQL Server",
  "use_win_auth": false,
  "sql_user": "dev",
  "collation": "Modern_Spanish_CI_AS",
  "compat": 120,
  "app_dir": "C:\\DEV\\SIDC",
  "docker_dir": "C:\\DEV\\SIDC\\docker-dev",
  "backup_dir": "C:\\DEV\\SIDC\\AegisSetup\\assets\\backups\\sqlserver2014",
  "legacy_dir": "C:\\DEV\\SIDC\\AegisSetup\\assets\\legacy\\ocx"
}
```

> `database` (`SIDC`) y `dsn_name` (`SIDC_SQL`) son fijos: el `.exe` los trae hardcodeados.

### Modos (`db_mode`)

| Modo | Dónde vive la base | Autenticación | Uso |
| --- | --- | --- | --- |
| `docker` | Docker por TCP | SQL Auth | Desarrollo y pruebas de migración |
| `local` | SQL en la misma PC | Windows Auth | Producción clásica |
| `server` | Instancia remota | Windows / SQL Auth | Producción distribuida |

### Presets de la TUI

| Tecla | Preset | Deja la config en |
| --- | --- | --- |
| `4` | dev | `dev` + `docker` + `localhost,14333` |
| `5` | prod local | `prod` + `local` + `localhost` + Windows Auth |
| `6` | prod server | `prod` + `server` + `CONTABILIDAD` (o `--server X`) + Windows Auth |

## Secretos

Las claves **nunca** se escriben en `config.json`. Salen de variables de entorno o de
flags, y el TUI las pide al inicio (solo en memoria, solo para esa corrida).

| Variable | Uso |
| --- | --- |
| `AEGIS_SA_PASSWORD` | Clave `sa` para `setup-db` |
| `AEGIS_SQL_PASSWORD` | Clave del login SQL Auth de la app (también es el `SA_PASSWORD` de Docker) |

```bash
export AEGIS_SA_PASSWORD='...'    # PowerShell: $env:AEGIS_SA_PASSWORD='...'
export AEGIS_SQL_PASSWORD='...'
```

## Flujo de desarrollo con Docker

`docker/docker-compose.yml` levanta un **SQL Server 2019 Developer** que replica
producción (collation `Modern_Spanish_CI_AS`, compat `120`).

```bash
cp docker/.env.example docker/.env      # ajustá SA_PASSWORD
docker compose -f docker/docker-compose.yml up -d
```

| Dato | Valor |
| --- | --- |
| Contenedor | `sidc_sql2019` |
| Puerto | `14333` → `1433` |
| Memoria | `mem_limit: 2g` |
| Respaldo | `assets/backups/sqlserver2014` montado `:ro` |

La guía detallada está en [`docker/GUIA-DOCKER.txt`](docker/GUIA-DOCKER.txt).

## Qué debe aportar el operador

Estos artefactos **no** van al control de versiones y los colocás vos antes de instalar:

| Ruta | Contenido |
| --- | --- |
| `assets/backups/sqlserver2014/` | El `.bak` de SIDC (SQL 2014). |
| `assets/legacy/ocx/` | Los OCX/DLL de la PC vieja (se registran con `regsvr32`). |
| `assets/legacy/crystal/` | Instalador del runtime de Crystal Reports 8. |
| `assets/oldpc/NOTAS.txt` | DSN, collation y usuarios de la app (referencia). |

## Estructura

```text
AegisSetup/
├── cmd/aegis/          # punto de entrada (main)
├── internal/
│   ├── cli/            # comandos Cobra: root, commands, menu, configure
│   ├── config/         # config.json, defaults, validación y presets
│   ├── setup/          # SetupDB, WriteDSN, InstallOCX, PatchDockerExe
│   ├── check/          # verificación TCP + SQL + DSN + ficheros
│   └── ui/             # TUI Bubble Tea (menú, pasos, prompts de claves)
├── docker/             # SQL Server 2019 para dev + guía
├── assets/             # backups / legacy / oldpc (fuera del repo)
├── scripts/legacy/     # scripts de migración de la PC vieja (referencia)
└── bin/                # binario compilado
```

## Notas técnicas

- **DSN de 32 bits**: se escribe directo en el registro (`HKLM\SOFTWARE\WOW6432Node\ODBC\ODBC.INI\SIDC_SQL`) con `registry.CreateKey`, no vía la API de ODBC. Por eso `--save-pwd` sí persiste el `PWD` (solo dev).
- **OCX legacy**: se copian a `SysWOW64` y se registran con `regsvr32` de 32 bits. Además de los 11 controles, hay 11 DLL de soporte (satélites en español y data binding) que solo se copian.
- **Crystal Reports**: `check` valida el mínimo del runtime (`crpe32`, `craxddrt`, `crviewer`, puentes ODBC `p2sodbc`/`u2fodbc`). El runtime completo se instala con su Setup original.
- **Parche `_DOCKER.exe`**: reemplaza `Initial Catalog=SIDC` (20 caracteres) por `UID=<user>;PWD=<pass>` dentro del binario, sin alargarlo. El largo de la clave está acotado por ese espacio — la TUI valida antes de arrancar para no fallar al final.

## Desarrollo

```bash
go test ./...   # tests
go vet ./...    # static check
gofmt -l .      # formatting
```

## Estado

Análisis del binario legacy y documentación completados; implementación del
instalador en curso.

## Licencia

Software **propietario**. Todos los derechos reservados © Antony Monge López.
Su uso, copia, distribución o modificación por terceros requiere permiso expreso
por escrito o una licencia comercial. Ver [LICENSE](LICENSE).
