# HANDOFF · SIDC / AegisSetup — F7c terminado, F7d pendiente

> Archivo de traspaso para la próxima sesión. Se puede borrar cuando F7 esté cerrado.
> Fecha: cierre de la sesión que implementó **F7c**.

## 1. Estado del repo (verificado al escribir esto)

- Proyecto: `C:/DEV/SIDC/AegisSetup` (módulo Go `aegis-setup`, toolchain 1.27).
- **HEAD = `e1ecd28`**. Recordá que **F6 + F7a + F7b + F7c están SIN COMMITEAR**.
- `git status` (resumen): 21 archivos modificados + 16 nuevos. Los nuevos son:
  - F7c: `docker/embed.go`, `docker/embed_test.go`, `internal/setup/compose.go`,
    `internal/setup/docker_test.go`, `internal/ui/compose.go`, `internal/ui/compose_test.go`,
    `internal/cli/compose.go`, `internal/cli/compose_test.go`.
  - F7a (ya listos antes): `internal/ui/ocx.go`, `internal/ui/ocx_test.go`,
    `internal/cli/ocx.go`, `internal/cli/ocx_test.go`, `internal/setup/ocx_test.go`.
  - F7b: `internal/ui/appdir_test.go`, `internal/cli/appdir_test.go`.
- **Estado verde**: `gofmt -l .` vacío · `go vet ./...` limpio · `go build ./...` OK ·
  `GOOS=linux go build ./...` OK · `go test ./... -count=1` todo ok (8 paquetes).
- `bin/aegis.exe` está gitignored y quedó **desactualizado**: hay que reconstruirlo
  (`go build -o bin/aegis.exe ./cmd/aegis`) porque cambió el manual embebido.

## 2. Qué se hizo en F7c (parte de F7 que acaba de cerrarse)

Fase F7 = "requisitos externos + distribución". Se partió en F7a/F7b/F7c/F7d.
**F7c = que el kit de Docker viaje dentro del EXE.**

1. `docker/embed.go` — **paquete Go dentro de `docker/`** (`package docker`), porque
   `//go:embed` no acepta `..`. Patrones explícitos, **sin comodines**:
   `//go:embed docker-compose.yml GUIA-DOCKER.txt init` + `//go:embed all:.env.example`.
   Expone `FS() fs.FS`. **El `.env` NO se embebe** (tiene la clave del `sa`); el test
   `TestLaClaveDelDesarrolladorNoViajaEnElBinario` es el guardián.
2. `internal/setup/compose.go` — `InstallDockerAssets(origen fs.FS, dir string, out func(string)) []string`.
   Recorre el FS, **compara contenido** (no fecha), no reescribe lo que ya está igual,
   acumula fallos nombrando archivo + carpeta, y detecta kit vacío (`vacio`).
3. `config.DirDocker()` = `C:\ProgramData\AegisSetup\docker`; `Default().DockerDir` pasó de
   `C:\DEV\SIDC\docker-dev` (ruta colgada, la carpeta no existe) a `DirDocker()`.
4. `config.Load()` **repara** `docker_dir`: si viene vacío, relativo o apuntando a una carpeta
   que no existe, se reemplaza por `DirDocker()`. Una carpeta propia que existe se respeta.
5. Dónde se extrae el kit (misma función, idempotente):
   - `internal/cli/configure.go` → al guardar la config (es el momento en que se elige
     `db_mode=docker`, y el checklist da justo ese comando para levantar el motor).
   - `internal/cli/commands.go` → `ejecutarSetupDB` (extraído del `RunE`, patrón de F6), antes
     de buscar el `.bak`.
   - `internal/ui/app.go` → `runStep(taskSetupDB)`, antes de buscar el `.bak`.
   - Seams para tests: vars `composeFS`/`composeDir` en `internal/ui` y en `internal/cli`.
6. `internal/precheck/precheck.go` → `comandoCompose(cfg)` + `notaCompose`: el arreglo del motor
   y el de Docker ya no dicen `docker/docker-compose.yml` (ruta del repo, inexistente en la PC
   destino) sino `filepath.Join(cfg.DockerDir, "docker-compose.yml")` + "(Aegis deja ese
   archivo al correr Setup DB)".
7. Textos: `docker/GUIA-DOCKER.txt` reescrito para la PC destino (el kit llega del EXE, el
   `.env` se crea ahí) y `README.md` de AegisSetup actualizado (config de ejemplo, flujo
   rápido, sección Docker, estructura, tabla de lo que aporta el operador).
8. `docker/docker-compose.yml`: comentario del volumen del `.bak` explicando que
   `../assets/backups/sqlserver2014` resuelve al `backup_dir` real **porque el compose ahora
   vive en `<ProgramData>\AegisSetup\docker`** (coincidencia deliberada, no casual).

### Verificación real hecha (no solo tests)

- `AEGIS_PROGRAMDATA=<tmp> AEGIS_NO_ELEVAR=1 ./bin/aegis.exe setup-db --bak <inexistente>` →
  imprimió `DOCKER OK: .env.example / GUIA-DOCKER.txt / docker-compose.yml / init/01-sidc.sql`
  y dejó exactamente esos 4 archivos (sin `.env`) en `<tmp>\AegisSetup\docker`.
- `configure --out <tmp>` → `"docker_dir": "<tmp>\AegisSetup\docker"`.
- `checklist` → arreglo del motor con la ruta real del compose de esa PC.
- Barrido paquete por paquete confirmando que **ningún test escribe en el `ProgramData` real**.

## 3. Lo que falta: F7d (siguiente)

`release.yml` adjunta el `.bak` como asset del Release + comando `aegis bak` que lo diga
(decidido: **Aegis NO descarga nada**, el operador lo baja a mano del Release) + ajustar el
texto de precheck/README. Detalle: el `.bak` pesa ~38.9 MB y vive en
`assets/backups/sqlserver2014/SIDC_2014_COPYONLY_20260911.bak`. Después: **F8** (desinstalación
limpia), **F9** (deuda: `runStep` duplicado entre ui y cli, `sidc_dev` vs `dev`, `COMDLG32.OCX`
fantasma, `openspec/project.md` viejo), **F10** (PC limpia de punta a punta: UAC + Crystal
reales), **F11** (docs de flujo).

## 4. Por qué esta sesión tardó tanto (para no repetirlo)

El trabajo real fueron ~2 horas de máquina; el resto fue pelear con el arnés. Cinco causas, con
el atajo para cada una:

1. **Heredoc + barras invertidas.** `cat > x <<'EOF'` y `python - <<'PY'` **comen `\`**. Me hizo
   fallar 4 reemplazos de rutas Windows (`C:\ProgramData`, `\n`, `` `\CONTABILIDAD\SIDC` ``).
   → **Atajo:** para cualquier script con rutas Windows, escribir el `.py` con la herramienta
   `write` y ejecutarlo (`python zz_x.py`), después borrarlo. Dentro del script usar
   `B = chr(92)` y concatenar. Nunca heredoc con backslashes.
2. **La salida de las herramientas viene RE-INDENTADA.** `sed -n '24,28p' f | cat -A` me mostró
   10 espacios de sangría cuando el archivo tenía 6. Conclusión: **la indentación que ves en la
   salida de `bash`/`read` NO es la del archivo**. → **Atajo:** para construir un `oldText`
   exacto, pedir `python -c "print(repr(linea))"` y copiar de ahí. Así el `edit` acierta a la
   primera; si falla dos veces, no insistir: script con `write`.
3. **Tests que escriben fuera de `t.TempDir()`.** Al agregar la extracción del kit, tres tests
   empezaron a escribir en `C:\ProgramData\AegisSetup\docker` (uno no se puede stubear: un test
   de `cli` que maneja un `ui.Model` no alcanza las vars internas de `ui`). → **Atajo:** (a)
   correr el paquete, mirar si el directorio real quedó creado; (b) aislar por test con un loop
   `for t in $(go test ./pkg/ -v | grep '=== RUN'); do rm -rf <dir real>; go test -run "^$t$"; [ -d <dir real> ] && echo ESCRIBE: $t; done`.
   Eso ubica al culpable en 1 minuto en lugar de adivinar. **Regla que quedó: la extracción del
   kit NO va en `ui.aplicarPerfil`** (los tests de `cli` no la pueden stubear).
4. **Rediseño en el aire.** Di dos vueltas al "¿dónde se extrae el kit?" (perfil del TUI vs paso
   DB vs configure). → **Atajo:** decidir con la pregunta "¿qué test puede cubrirlo?" antes de
   escribir código; si un sitio no es testeable sin exportar API solo para tests, no es el sitio.
5. **Verificación cara.** `go test ./...` completo + `GOOS=linux go build` en cada iteración. Fue
   ~30 corridas de 20-60 s. → **Atajo:** durante el ciclo, sólo el paquete tocado
   (`go test ./internal/cli/ -run X -count=1`); el gate completo **una vez al final**.
   Y nunca subir el binario con `-count=1` a la vez que tests de UI (se pisan en el tiempo).

## 5. Reglas de la casa que no se negocian (ya validadas en esta fase)

- **No commitear ni pushear**: eso lo hace el usuario. Como mucho, proponer un split por unidad
  de trabajo.
- **No volver a plantear la visibilidad del repo** (`AntonyML/aegissetup` es público y así queda).
- Encabezado de copyright en **todo** archivo: `// © Antony Monge López — Costa Rica — Céd. 604700548`.
  **Los build tags van primero**, después el copyright, después `package`.
- Comentarios y textos de error **en español con voseo costarricense**; nombres de funciones en
  inglés en el paquete `setup`.
- **TDD estricto** (`strict_tdd: true`): test que falla (RED), implementación (GREEN), después
  triangular. Los tests de esta fase que ya existen son el contrato: no borrarlos para "que pase".
- **Nunca** automatizar una prueba de `ShellExecuteExW`/UAC: aparece un cartel de consentimiento
  real y bloquea 3 minutos (ya pasó). La elevación se valida sólo en F10, a mano.
- Escribir archivos nuevos con `write` y editar con `edit`. **No** usar `sed`/`str.replace` a
  ciegas: fallan en silencio sin decir nada.
- `git diff` escupe `warning: LF will be replaced by CRLF` → filtrar con `grep -v warning`.
- `lens_diagnostics(scope="paths")` quiere rutas relativas a la raíz del proyecto. Si dice que
  el LSP no está, no inventar: seguir con `go vet`/`go test`.
