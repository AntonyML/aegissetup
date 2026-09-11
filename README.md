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

- **Crystal sin instalador** — el runtime de Crystal Reports 8 viaja dentro del `EXE` y Setup App lo instala y registra solo.
- **Elevación al pedido** — `[E]` en el menú (o el propio subcomando) relanza Aegis como administrador y hereda el código de salida; el diagnóstico nunca queda detrás del UAC.
- **Instalación desatendida** — un solo comando restaura la base, deja el entorno listo y verifica el resultado.
- **Perfil por máquina** — la primera vez pregunta en qué PC está corriendo y lo guarda; el resto de las corridas ya no pregunta.
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
| **Privilegios** | Administrador para lo que escribe (`setup-db`, `setup-app`): Aegis se relanza elevado solo. `check`, `checklist` y `dashboard` corren sin permisos. |

## Compilar

```bash
cd AegisSetup
go build -o bin/aegis.exe ./cmd/aegis
```

El binario queda en `bin/aegis.exe`. La configuración vive en
`%APPDATA%\AegisSetup\config.json` (o la que le pases con `--config`). Un `config.json`
viejo junto al binario se sigue leyendo si el canónico todavía no existe.

## Uso

### Comandos

| Comando | Función |
| --- | --- |
| `setup-db` | Restaura el `.bak` como `SIDC`: collation, compat level, logins y `CHECKDB`. |
| `setup-app` | Crea el DSN `SIDC_SQL` de 32 bits, instala los OCX legacy, instala el runtime de Crystal embebido y verifica exe/reportes. |
| `check` | Valida TCP + SQL + DSN + ficheros (App ⇄ DB). Solo lectura. |
| `checklist` | Requisitos de la máquina en orden: qué falta, por qué importa, cómo se arregla y qué opción queda trabada. Solo lectura. |
| `dashboard` | Panel de estado no interactivo (env, DSN, DB, app). |
| `menu` | TUI interactivo (instalación completa, por etapas y presets). |
| `configure` | Genera el `config.json` inicial. |

Sin subcomando, `aegis` abre directamente la TUI (si hay terminal interactiva).

### Primera vez: el perfil de la máquina

Si no hay `config.json`, la TUI **no** abre el menú: lo primero que pregunta es en qué PC
está corriendo. La elección se guarda y no se vuelve a preguntar.

```text
AEGIS SETUP
Todavía no hay config en esta PC: decime en qué PC estamos.

ESTA PC ES...
> [1] Pruebas - SQL Server 2019 en Docker, en esta misma PC
  [2] Producción en esta PC - SIDC y SQL Server locales, Windows Auth
  [3] Producción en un servidor - SQL Server en otra PC de la red
```

Los tres son **las mismas acciones** que los presets `4`/`5`/`6` del menú: la misma
decisión, tomada en dos momentos distintos. Elegir el perfil escribe el `config.json` y
**recalcula el checklist** contra el ambiente nuevo —los requisitos de Docker no aplican en
la PC de producción, y quedarse con la medición anterior trabaría el menú con requisitos del
ambiente que se acaba de abandonar.

Si elegís *Producción en un servidor*, el TUI pregunta el nombre del servidor en vez de
adivinar `CONTABILIDAD`: un nombre inventado se ve igual de válido que uno real hasta que
falla la conexión. Con `--server X` no pregunta (camino no interactivo).

`Esc`/`Q` en esa pantalla sale sin escribir nada. Lo que no hace es seguir al menú con el
perfil `dev` por defecto: ese era justamente el problema.

### Checklist y desbloqueo progresivo

El checklist es una lista **ordenada por dependencia**: cada requisito dice qué falta,
por qué importa, qué hacer y **qué opción del menú deja trabada**. Resolverlo de arriba
hacia abajo es lo que va destrabando el resto.

```text
$ aegis checklist
FALTA  3. Docker en marcha
           docker no responde (¿está instalado y el motor encendido?)
           por qué: El perfil de pruebas corre SQL Server 2019 en un contenedor.
           arreglo: Instalá Docker Desktop y levantá el motor: docker compose -f C:\ProgramData\AegisSetup\docker\docker-compose.yml up -d (Aegis deja ese archivo al correr Setup DB)
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

### Permisos

Aegis no arranca elevado: lo pide cuando hace falta.

| Dónde | Qué pasa |
| --- | --- |
| `setup-db`, `setup-app` | El proceso padre avisa, relanza el mismo comando elevado, espera y devuelve **el mismo código de salida** del hijo. |
| Menú de la TUI | `[E]` relanza Aegis elevado en otra ventana y cierra la actual (dos Aegis sobre la misma máquina se pisan). |
| `check`, `checklist`, `dashboard` | Nunca piden permisos: son el diagnóstico, y en una PC rota puede no haber forma de aceptar un UAC. |
| `AEGIS_NO_ELEVAR=1` | Corta la elevación. Es la marca que se le pone al proceso ya elevado para que no se relance a sí mismo en un bucle de UAC. |

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

./bin/aegis configure --env dev --db-mode docker   # 1. config.json + kit de Docker
./bin/aegis checklist                               # 2. ¿la máquina está lista? (exit 3 = no)
docker compose -f "$env:ProgramData\AegisSetup\docker\docker-compose.yml" up -d   # 3. base dev
./bin/aegis setup-db                                # 4. restaura el .bak como SIDC
./bin/aegis setup-app                               # 5. DSN + OCX + Crystal + verificación
./bin/aegis check                                   # 6. valida App ⇄ DB
./bin/aegis menu                                    # o todo de un tirón desde la TUI
```

En una PC nueva, `./bin/aegis menu` es el primer paso real: pregunta el perfil, lo guarda y
de ahí en adelante todo lo demás usa esa configuración.

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
  "app_dir": "C:\\SIDC",
  "docker_dir": "C:\\ProgramData\\AegisSetup\\docker",
  "backup_dir": "C:\\ProgramData\\AegisSetup\\assets\\backups\\sqlserver2014",
  "legacy_dir": ""
}
```

> `database` (`SIDC`) y `dsn_name` (`SIDC_SQL`) son fijos: el `.exe` los trae hardcodeados.
>
> **Ningún default apunta a la máquina de desarrollo.** `app_dir` no tiene default (lo dice el
> perfil del TUI o `--app-dir`), `backup_dir` y `docker_dir` viven bajo `ProgramData`, y
> `legacy_dir` vacío significa que los OCX salen del propio binario.
>
> `docker_dir` es una carpeta que administra Aegis: si el `config.json` trae una que no existe
> (por ejemplo la del repo de quien programa Aegis), al cargar se repara a la del producto.

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
| `6` | prod server | `prod` + `server` + el nombre que le digas (o `--server X`) + Windows Auth |

El `6` pregunta el nombre del servidor si no viene por `--server`. Cambiar de preset cambia
la config **y** vuelve a medir la máquina: lo que traba se recalcula contra el ambiente
nuevo, no contra el anterior.

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
producción (collation `Modern_Spanish_CI_AS`, compat `120`). El kit (compose, guía,
`.env.example` e `init/`) **viaja dentro del `.exe`** y Aegis lo deja en `docker_dir`, así que
en la PC destino no hace falta el repo:

```bash
# Aegis ya dejó el kit en C:\ProgramData\AegisSetup\docker (lo hace configure y el paso 1)
cd "$env:ProgramData\AegisSetup\docker"
copy .env.example .env                  # ajustá SA_PASSWORD
docker compose up -d
```

> El `.env` **no** va dentro del binario: tiene la clave del `sa` de quien desarrolla. Lo
> sostienen el `.gitignore` y el test `TestLaClaveDelDesarrolladorNoViajaEnElBinario`.

| Dato | Valor |
| --- | --- |
| Contenedor | `sidc_sql2019` |
| Puerto | `14333` → `1433` |
| Memoria | `mem_limit: 2g` |
| Respaldo | `<ProgramData>\AegisSetup\assets\backups\sqlserver2014` montado `:ro` |

La guía detallada está en [`docker/GUIA-DOCKER.txt`](docker/GUIA-DOCKER.txt).

## Qué debe aportar el operador

Estos artefactos **no** van al control de versiones y los colocás vos antes de instalar:

| Ruta | Contenido |
| --- | --- |
| `assets/backups/sqlserver2014/` | El `.bak` de SIDC (SQL 2014). En la PC destino va a `C:\ProgramData\AegisSetup\assets\backups\sqlserver2014\`, o al lado del `.exe`. |
| `assets/legacy/ocx/` | Los OCX/DLL de la PC vieja (se registran con `regsvr32`). Van dentro del binario. |
| `assets/legacy/crystal/` | Runtime de Crystal Reports 8. Va dentro del binario: Setup App lo instala solo (no es un paso manual). |
| `assets/oldpc/NOTAS.txt` | DSN, collation y usuarios de la app (referencia). |

El compose del SQL de pruebas **no** lo aporta el operador: viaja dentro del binario.

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
├── docker/             # SQL Server 2019 para dev (paquete Go: embed del kit + guía)
├── assets/             # backups / legacy / oldpc (fuera del repo)
├── scripts/legacy/     # scripts de migración de la PC vieja (referencia)
└── bin/                # binario compilado
```

## Notas técnicas

- **DSN de 32 bits**: se escribe directo en el registro (`HKLM\SOFTWARE\WOW6432Node\ODBC\ODBC.INI\SIDC_SQL`) con `registry.CreateKey`, no vía la API de ODBC. Por eso `--save-pwd` sí persiste el `PWD` (solo dev).
- **OCX legacy**: se copian a `SysWOW64` y se registran con `regsvr32` de 32 bits. Además de los 11 controles, hay 11 DLL de soporte (satélites en español y data binding) que solo se copian.
- **Crystal Reports**: el runtime completo (43 archivos, 24 MB) viaja dentro de `Aegis.exe`. Setup App lo copia a `SysWOW64` y registra los 4 componentes COM que usan los reportes (`crviewer.dll`, `Crystl32.OCX`, `craxdrt.dll`, `craxddrt.dll`); el resto son dependencias que con existir alcanzan. Se copia solo lo que falta o difiere, y el registro se repite igual (que el archivo esté no quiere decir que esté registrado). `check` valida el mínimo del runtime.
- **Elevación**: no hay manifiesto `requireAdministrator`, porque dejaría `check`, `checklist` y `dashboard` detrás de un UAC — justo los comandos que se usan cuando algo está mal. En su lugar Aegis se relanza elevado cuando el subcomando escribe en la máquina (`setup-db`, `setup-app`), y en el menú con `[E]`. El proceso padre espera al hijo y devuelve su mismo código de salida.
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

Instalador funcional de punta a punta en dev. Hecho: CI + releases, runtime embebido, rutas
machine-wide, checklist-puerta, perfil por máquina al arrancar e instalación automática de
Crystal con elevación al pedido. Pendiente: descarga del `.bak`, desinstalación limpia y
validación en una PC limpia (incluye probar el UAC real).

## Licencia

Software **propietario**. Todos los derechos reservados © Antony Monge López.
Su uso, copia, distribución o modificación por terceros requiere permiso expreso
por escrito o una licencia comercial. Ver [LICENSE](LICENSE).
