# AGENTS.md — AegisSetup

Este archivo es un **adaptador**. No contiene reglas propias.

> Las reglas viven en **`C:\DEV\SIDC\docs\agents\`**. Es el único lugar donde una
> regla vive. Si acá encontrás algo que contradice a `docs/agents/`, gana
> `docs/agents/` y este archivo tiene un bug.

## Qué es este repo

`AegisSetup` es un CLI en **Go 1.27** que instala SIDC en PC cliente sin pasos
manuales: restaura el `.bak`, arma el DSN de 32 bits, instala el runtime Crystal y
los OCX, verifica, parchea el binario de dev, y se desinstala sin tocar SIDC.

- Módulo Go: `aegis-setup`. **La raíz del workspace no es módulo Go.**
- TUI: Bubble Tea v2 + Cobra.
- 196 archivos trackeados. 72 archivos `.go`, 31 de ellos `_test.go`.
- Remote: `github.com/AntonyML/aegissetup` — **repo público**.
- Este repo es **anidado**: el repo SIDC lo ignora. Un commit desde
  `C:\DEV\SIDC` **nunca** toca este código.

## Antes de hacer nada

1. `C:\DEV\SIDC\docs\agents\STATUS.md`
2. `C:\DEV\SIDC\docs\agents\ROLES.md`
3. `C:\DEV\SIDC\docs\agents\WORKSPACE.md`
4. `C:\DEV\SIDC\docs\agents\SECRETS_POLICY.md`
5. Tu `C:\DEV\SIDC\docs\agents\tasks\T*.md`

Y leé `HANDOFF.md` de este repo: tiene las reglas de la casa que no están en otro
lado — el encabezado de copyright obligatorio, los build tags primero, el voseo
costarricense, y los atajos que la sesión anterior aprendió peleando con el
arnés. **Cuidado: está desactualizado en su estado.** Es memoria, no estado.

## Estructura

| Ruta | Qué |
| --- | --- |
| `cmd/aegis/main.go` | Punto de entrada |
| `internal/{check,cli,config,precheck,setup,ui,auth,securestore}` | Lógica de dominio |
| `internal/securestore/` | **DPAPI.** El único almacenamiento admitido para una clave |
| `assets/legacy/crystal/SIDC_CRYSTAL/` | 43 archivos de runtime Crystal 8, embebidos |
| `assets/legacy/ocx/` | 23 archivos OCX de VB6 |
| `docker/` | Kit de desarrollo + `embed.go` |
| `.github/workflows/release.yml` | Publica en tags `v*` |
| `docs/FLUJO-INSTALACION.md` | El runbook del operador |

## Comandos

```powershell
cd C:\DEV\SIDC\AegisSetup

gofmt -l .                    # VACIO = limpio
go vet ./...                  # limpio
go build ./...
GOOS=linux go build ./...     # no es decoracion: los seams estan tras build tags
go test ./... -count=1        # 8 paquetes

go build -o bin\aegis.exe .\cmd\aegis
```

Detalle y por qué de cada uno: skill `aegis-build-release`.

## Reglas que no se negocian acá

- **TDD estricto** (`openspec/config.yaml` → `strict_tdd: true`). Los tests
  existentes son el contrato: **no los borres para que pase**.
- **No debilites la allowlist** de `.gitignore` para `assets/legacy/`. Es
  deliberada: es runtime propietario que tiene que viajar en el `.exe`.
- **No agregues comodines** al `//go:embed` de `docker/embed.go`. El `.env` queda
  afuera a propósito; `TestLaClaveDelDesarrolladorNoViajaEnElBinario` lo vigila.
- **No muevas ni borres tags `v*`.** `release.yml` calcula la versión semántica a
  partir de ellos.
- **Encabezado de copyright** en todo archivo:
  `// © Antony Monge López — Costa Rica — Céd. 604700548`
  **Build tags primero**, después el copyright, después `package`.
- Comentarios y errores en **español con voseo costarricense**; nombres de
  funciones en inglés en el paquete `setup`.
- **No commiteás ni pusheás.** Eso lo hace el humano (salvo los archivos del
  arquitecto de entorno, que van en su propia rama).

## Deuda técnica conocida

La secuencia de instalación está **duplicada** entre `internal/ui/app.go`
(`runStep`) y `internal/cli/commands.go` (`ejecutarSetupDB` /
`ejecutarSetupApp`), y **ya divergió**: el TUI imprime la línea de resumen de
Crystal y el CLI no, y el TUI ignora fallos de Crystal en `taskSetupApp` mientras
el CLI los reporta. Verificar y corregir es la tarea del subagente
`audit-tui-flow`.
