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
| `checklist` | Requisitos de la máquina en orden: qué falta, por qué importa, cómo se arregla y qué opción queda trabada. Solo lectura. |
| `dashboard` | Panel de estado no interactivo (env, DSN, DB, app). |
| `menu` | TUI interactivo (instalación completa, por etapas y presets). |
| `configure` | Genera el `config.json` inicial. |

Sin subcomando, `aegis` abre directamente la TUI (si hay terminal interactiva).

### Checklist y desbloqueo progresivo

El checklist es una lista **ordenada por dependencia**: cada requisito dice qué falta,
por qué importa, qué hacer y **qué opción del menú deja trabada**. Resolverlo de arriba
hacia abajo es lo que va destrabando el resto.

```text
$ aegis checklist
FALTA  3. Docker en marcha
           docker no responde (¿está instalado y el motor encendido?)
           por qué: El perfil de pruebas corre SQL Server 2019 en un contenedor.
           arreglo: Instalá Docker Desktop y levantá el motor: docker compose -f docker/docker-compose.yml up -d
           traba:   Setup DB, Instalación completa
```

Tres reglas que explican por qué está armado así:

- **El diagnóstico nunca se traba.** `check`, `checklist`, `dashboard` y los presets
  siguen disponibles con todo roto: son justamente la herramienta que dice qué arreglar.
- **Lo que falta no siempre traba.** En una PC limpia la base y el DSN faltan y el flujo
  arranca igual, porque son el **resultado** de `setup-db`/`setup-app` y no su condición
  previa. Contarlos como bloqueo sería circular.
- **Cada bloqueo tiene salida.** Los OCX faltantes se avisan pero no traban: los instala
  el propio `setup-app`. Si trabaran esa opción, el operador no tendría cómo resolverlos.
- **Hay un tercer estado.** `AVISO` significa "no lo pude verificar", no "está mal". El caso
  real: una instancia con nombre (`localhost\SQLEXPRESS`, el default de Express, que es el
  motor de producción) negocia el puerto por SQL Browser, así que sondear un puerto fijo no
  prueba nada. Ahí se avisa y decide la conexión real a la base. Un `AVISO` nunca traba.

La TUI aplica la misma puerta y muestra el checklist con `[C]`. La política de qué traba
qué vive en `internal/precheck` (`trabaPorRequisito`), y la comparten la TUI y la CLI: no
pueden discrepar sobre la misma máquina.

### Códigos de salida

| Código | Significado |
| ---: | --- |
| `0` | OK |
| `1` | Error general |
| `2` | Error de configuración |
| `3` | `check` o `checklist` con fallos: la máquina no está lista |

El `3` es distinto del `1` a propósito: permite que un script distinga "esta máquina no
está lista" (se arregla instalando algo) de "Aegis se rompió" (se arregla Aegis).

### Flags

```text
Globales
  --config string    ruta a config.json (default: %APPDATA%\AegisSetup\config.json)
  --server string    server del preset "prod server" de la TUI

setup-db
  --bak string           .bak a restaurar (default: el más nuevo de backup_dir)
  --sa-password string   clave SA   (o env AEGIS_SA_PASSWORD; vacío = Windows Auth)
  --app-password string  clave del login SQL Auth (o env AEGIS_SQL_PASSWORD)

setup-app
  --app-password string  clave del login SQL Auth (o env AEGIS_SQL_PASSWORD)
  --save-pwd             guarda el PWD en el DSN (SOLO dev/docker, nunca prod)
  --patch-docker         genera _DOCKER.exe con UID/PWD embebidos (solo dev/docker)

check / checklist / dashboard
  --app-password string  clave del login SQL Auth (o env AEGIS_SQL_PASSWORD)

configure
  --env string       dev | prod
  --db-mode string   docker | local | server
  --server string    localhost,14333 | localhost | CONTABILIDAD | MI_SERVIDOR
  --out string       ruta de salida (default: %APPDATA%\AegisSetup\config.json)
```

### Códigos de salida

Los códigos de salida están documentados arriba, con el checklist.

## Flujo rápido

```bash
cd AegisSetup
go build -o bin/aegis.exe ./cmd/aegis

./bin/aegis configure --env dev --db-mode docker   # 1. genera config.json
./bin/aegis checklist                                # 2. ¿la máquina está lista? (exit 3 = no)
docker compose -f docker/docker-compose.yml up -d   # 3. base dev (SQL 2019, compat 120)
./bin/aegis setup-db                                 # 4. restaura el .bak como SIDC
./bin/aegis setup-app                                # 5. DSN + OCX + verificación
./bin/aegis check                                    # 6. valida App ⇄ DB
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
  "backup_dir": "C:\\ProgramData\\AegisSetup\\assets\\backups\\sqlserver2014",
  "legacy_dir": "C:\\DEV\\SIDC\\AegisSetup\\assets\\legacy\\ocx"
}
```

> `database` (`SIDC`) y `dsn_name` (`SIDC_SQL`) son fijos: el `.exe` los trae hardcodeados.
>
> **`app_dir`, `docker_dir` y `legacy_dir` todavía traen rutas de la máquina de
desarrollo.** En una PC de FEMUCARIBE hay que apuntarlos a donde esté SIDC. `backup_dir`
ya usa la ruta de máquina correcta (ver abajo).

### Rutas de datos

El instalador corre **elevado**, así que no puede escribir en "Documentos": esa carpeta
apuntaría al perfil del administrador y no al del operador. Los datos viven en rutas de
máquina:

| Ruta | Qué guarda |
| --- | --- |
| `C:\ProgramData\AegisSetup\assets\backups\sqlserver2014\` | El `.bak` de SIDC. |
| `C:\ProgramData\AegisSetup\assets\` | Artefactos que el instalador deja en la máquina. |
| `%APPDATA%\AegisSetup\config.json` | La config del entorno. |

`aegis` crea esas carpetas al arrancar (best-effort: el diagnóstico tiene que poder correr
justo cuando algo está mal). El `.bak` se busca en este orden, y se usa el `.bak` **más
nuevo** del primer directorio que tenga alguno:

1. `backup_dir` de la config.
2. `assets\backups\sqlserver2014\` **al lado del ejecutable** — el operador lo deja ahí y listo.
3. `..\assets\backups\sqlserver2014\` — el binario de desarrollo vive en `bin/`.

La config también se resuelve en orden: `--config` → `%APPDATA%\AegisSetup\config.json` →
`config.json` junto al binario (compatibilidad con instalaciones previas).

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
| `assets/backups/sqlserver2014/` | El `.bak` de SIDC (SQL 2014). En la PC destino va a `C:\ProgramData\AegisSetup\assets\backups\sqlserver2014\`, o al lado del `.exe`. |
| `assets/legacy/ocx/` | Los OCX/DLL de la PC vieja (se registran con `regsvr32`). Van dentro del binario. |
| `assets/legacy/crystal/` | Runtime de Crystal Reports 8 (va dentro del binario). |
| `assets/oldpc/NOTAS.txt` | DSN, collation y usuarios de la app (referencia). |

## Estructura

```text
AegisSetup/
├── cmd/aegis/          # punto de entrada (main)
├── internal/
│   ├── cli/            # comandos Cobra: root, commands, menu, configure
│   ├── config/         # config.json, defaults, validación y presets
│   ├── setup/          # SetupDB, WriteDSN, InstallOCX, PatchDockerExe
│   ├── precheck/       # checklist de requisitos + política de qué traba qué
│   ├── check/          # verificación TCP + SQL + DSN + ficheros
│   └── ui/             # TUI Bubble Tea (menú, checklist-puerta, pasos, prompts)
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
- **Checklist-puerta**: `internal/precheck` diagnostica (sondas inyectadas, así se testea sin ser admin, sin Docker y sin SysWOW64) y declara en `trabaPorRequisito` qué etapas traba cada requisito. `internal/ui/gate.go` traduce una tecla del menú a la etapa que le corresponde. La política vive en un solo lugar para que la TUI y `aegis checklist` no puedan discrepar sobre la misma máquina.
- **Conexión SQL compartida**: el checklist y `check` usan el mismo `setup.AppDSN`, en vez de armar cada uno su cadena: dos constructores distintos terminan reportando cosas distintas de la misma base.

## Desarrollo

```bash
go test ./...   # tests
go vet ./...    # static check
gofmt -l .      # formatting
```

## Estado

Instalador funcional de punta a punta en dev. Pendiente: instalación automática de Crystal
con elevación, descarga del `.bak`, desinstalación limpia y validación en una PC limpia.

## Licencia

Software **propietario**. Todos los derechos reservados © Antony Monge López.
Su uso, copia, distribución o modificación por terceros requiere permiso expreso
por escrito o una licencia comercial. Ver [LICENSE](LICENSE).
