# AegisSetup

Instalador CLI de **SIDC** para N equipos, sin pasos manuales.

![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)
![Cobra](https://img.shields.io/badge/CLI-Cobra-FF6F61?logo=go&logoColor=white)
![Bubble Tea](https://img.shields.io/badge/TUI-Bubble_Tea-FF75B7)

## Qué hace

| Comando | Función |
| --- | --- |
| `setup-db` | Restaura el `.bak` como `SIDC` (collation, compat 120, logins). |
| `setup-app` | Crea el DSN `SIDC_SQL` de 32 bits, instala OCX legacy y verifica exe/reportes. |
| `check` | Valida TCP + SQL + DSN + ficheros (App ⇄ DB). |
| `dashboard` | Panel de estado no interactivo. |
| `menu` | TUI interactivo (Bubble Tea): instalación completa, por etapas y presets. |
| `configure` | Genera `config.json` inicial. |

## Requisitos

- **Go 1.27**
- **Windows** para el flujo completo (DSN ODBC 32 bits, registro OCX, SQL Server)
- **Docker** (opcional) para la base de desarrollo

## Uso rápido

```bash
cd AegisSetup
go build -o bin/aegis.exe ./cmd/aegis

./bin/aegis configure --env dev --db-mode docker      # genera config.json
docker compose -f docker/docker-compose.yml up -d     # base dev (SQL 2019, compat 120)
./bin/aegis setup-db --bak C:\ruta\SIDC.bak           # restaura la base
./bin/aegis setup-app                                  # DSN + OCX + verificación
./bin/aegis check                                      # valida App ⇄ DB
./bin/aegis menu                                       # TUI interactivo
```

## Configuración

`config.json` (junto al binario o con `--config`). Un JSON parcial hace override de
los defaults; `configure` lo genera por vos.

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
  "backup_dir": "C:\\DEV\\SIDC\\AegisSetup\\assets\\backups\\sqlserver2014"
}
```

### Modos (`db_mode`)

| Modo | Descripción | Auth |
| --- | --- | --- |
| `docker` | Docker por TCP (dev y opcional en prod) | SQL Auth |
| `local` | SQL en la misma PC (prod clásico) | Windows Auth |
| `server` | Servidor remoto (prod) | Windows / SQL Auth |

## Secretos

Nunca se escriben en claro en `config.json`; van por flags o variables de entorno:

| Variable | Uso |
| --- | --- |
| `AEGIS_SA_PASSWORD` | Clave SA para `setup-db` |
| `AEGIS_SQL_PASSWORD` | Clave del login SQL Auth de la app |

## Desarrollo

```bash
go test ./...   # tests
go vet ./...    # static check
gofmt -l .      # formatting
```

## Licencia

[Apache License 2.0](LICENSE)
