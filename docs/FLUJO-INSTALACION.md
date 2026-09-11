# Flujo de instalación en una PC cliente

Para quien instala SIDC en la PC de un cliente, no para quien desarrolla Aegis.
Si vas a compilar o tocar el código, eso está en el [README](../README.md).

---

## 1. Antes de tocar la PC

### Lo que hay que llevar

| Qué | De dónde sale | Tamaño |
| --- | --- | --- |
| `aegis.exe` | Release de GitHub (`…/releases/latest`) | ~52 MB (lleva el runtime de Crystal adentro) |
| `SIDC_2014.bak` | El mismo Release, asset aparte | ~39 MB |

Aegis **no descarga nada**. Si te falta el `.bak`, el paso 3 no puede correr: `aegis bak` te
dice la URL exacta y en qué carpeta dejarlo, pero el archivo lo bajás vos.

### Lo que la PC tiene que tener

- Windows de 64 bits.
- SQL Server instalado y **andando** (2014 o 2019). Aegis no lo instala.
- La carpeta de SIDC ya copiada en la PC (el `.exe`, `Reportes/`, `Implode.dll`).
  Aegis **no** copia la aplicación: la verifica.
- Sesión con permisos de administrador. Los pasos 3, 4 y la desinstalación piden UAC.

### Cómo saber si estás listo

```powershell
.\aegis.exe checklist
```

- Sale con **0** → la máquina está lista, seguí.
- Sale con **3** → algo traba la instalación. El checklist dice **qué falta, por qué importa, cómo
  se arregla y qué paso queda trabado**. Arreglá eso antes de seguir: cada paso que sigue asume
  que el anterior está bien.

`checklist` y `check` **nunca** piden permisos de administrador. Es a propósito: se usan
justamente cuando la PC está rota, y en una PC rota puede no haber forma de aceptar un UAC
(sesión remota, script, otro usuario).

---

## 2. Elegir el perfil de la máquina

La PC tiene que saber dónde va a vivir la base. Corré:

```powershell
.\aegis.exe menu
```

y elegí el preset que corresponda:

| Tecla | Cuándo | Deja la config en |
| --- | --- | --- |
| `4` | Desarrollo, pruebas de migración | `dev` + `docker` + `localhost,14333` |
| `5` | **Cliente con SQL en la misma PC** | `prod` + `local` + `localhost` + Windows Auth |
| `6` | **Cliente con SQL en un servidor** | `prod` + `server` + el nombre del servidor |

En el preset `6` te va a preguntar el nombre del servidor (o pasalo con `aegis --server MI_SERVIDOR`).

Si preferís no usar el menú:

```powershell
.\aegis.exe configure --env prod --db-mode local
.\aegis.exe configure --env prod --db-mode server --server MI_SERVIDOR
```

> Cambiar de preset vuelve a medir la máquina: lo que traba se recalcula contra el ambiente
> nuevo, no contra el anterior. Si te equivocaste de perfil, cambialo y volvé a correr
> `checklist`.

---

## 3. Restaurar la base

```powershell
# Si tenés el .bak en el Release y todavía no lo bajaste:
.\aegis.exe bak          # te dice la URL y dónde dejarlo; sale con 3 si falta

# Restaurar:
.\aegis.exe setup-db
```

Esto deja la base `SIDC` con la collation (`Modern_Spanish_CI_AS`), el compatibility level (120),
los logins y un `CHECKDB` al final.

**Este paso pide UAC.** El proceso padre avisa, se relanza elevado, espera al hijo y devuelve el
mismo código de salida. Si ya estás en una consola elevada no pasa nada: no se relanza dos veces.

Si la instancia usa autenticación SQL en vez de Windows Auth:

```powershell
.\aegis.exe setup-db --sa-password "$env:MI_SA" --app-password "$env:MI_APP"
```

Los secretos también entran por `AEGIS_SA_PASSWORD` y `AEGIS_SQL_PASSWORD`. **Nunca** se escriben
en `config.json`.

---

## 4. Preparar la aplicación

```powershell
.\aegis.exe setup-app
```

En orden:

1. Crea el DSN `SIDC_SQL` de **32 bits** (la app es VB6: un DSN de 64 bits no le sirve).
2. Copia los OCX legacy a `SysWOW64` y los registra con `regsvr32` (son los que dan error 339).
3. Copia los 43 archivos del runtime de Crystal y registra los 4 componentes COM que usan los
   reportes.
4. Verifica que estén el `.exe` de SIDC y las plantillas de `Reportes/`.

Los OCX y Crystal salen **del propio `aegis.exe`**, no de una carpeta vieja: en una PC limpia no
hay de dónde copiarlos.

**También pide UAC.** Si dejás `--save-pwd` sin querer en una PC de producción, la clave del login
queda en texto plano en el registro: es una bandera **solo para dev/docker**.

---

## 5. Verificar que App y DB se hablan

```powershell
.\aegis.exe check
```

Valida las cuatro capas: TCP, SQL, DSN y ficheros. Es el único paso que prueba la cadena completa.

- Sale con **0** → SIDC funciona. Terminaste.
- Sale con **3** → hay fallos; los imprime línea por línea. Miralos con la tabla de abajo.

Para dejarlo pegado en un correo de soporte:

```powershell
.\aegis.exe dashboard | clip
```

---

## Si algo se traba

| Qué ves | Qué pasa | Qué hacer |
| --- | --- | --- |
| `checklist` sale con 3 por el `.bak` | No hay respaldo donde Aegis lo busca | `aegis bak` te da la URL y la carpeta. El orden de carpetas que imprime es el orden en que las busca: la primera gana. |
| `check` falla en TCP | El motor no está arriba o el puerto no es el que dice la config | Verificá el servicio de SQL Server y el `--server`/`db_mode`. |
| `check` falla en SQL | Hay TCP pero no llega a la base | Fijate usuario, clave y que la base `SIDC` exista. |
| `check` falla en DSN | El DSN 32-bit no está o quedó viejo | `aegis setup-app`. Tiene que ser el de 32 bits: un DSN de 64 bits deja a la app sin ver la base. |
| SIDC abre y falla con **error 339** | Falta registrar un OCX | `aegis setup-app` de nuevo. Si te pide UAC, aceptalo: `regsvr32` sin permisos no registra nada y el error vuelve igual. |
| SIDC abre y falla con **error 3146** | No llega a la base por el DSN | `aegis check` para confirmar cuál de las cuatro capas falla. Ojo: si corriste `aegis uninstall` sin `--keep-dsn`, el DSN se borró a propósito y hay que recrearlo con `aegis setup-app`. |
| Los reportes de Crystal no abren | Falta el runtime o un componente COM | `aegis setup-app` y mirá la línea `CRYSTAL:` con los conteos. |
| El UAC no aparece y el comando falla pidiendo permisos | Estás en una sesión sin UAC interactivo | Corré la consola **como Administrador** de entrada. Los pasos 3 y 4 escriben en `HKLM` y en `SysWOW64`. |
| Todo anda pero `AEGIS_NO_ELEVAR` quedó seteado | Es la marca del proceso ya elevado, no una opción de uso | Sacala del entorno. Si queda puesta, Aegis no va a poder elevarse nunca más. |

**Regla general:** el diagnóstico se corre chico y de arriba hacia abajo. `checklist` antes de
instalar, `check` después. Los dos son de solo lectura y no rompen nada: no dudes en correrlos
todas las veces que haga falta.

---

## Sacar Aegis de la PC

Cuando SIDC ya funciona y querés dejar la PC sin la herramienta:

```powershell
.\aegis.exe uninstall                     # muestra el plan; NO borra nada
.\aegis.exe uninstall --keep-dsn --yes    # saca Aegis y deja SIDC andando
```

**Usá `--keep-dsn`.** El DSN `SIDC_SQL` no es de Aegis, es de la app: sin él SIDC falla con error
3146. Sacalo solo si querés desarmar la instalación completa.

Sin `--yes` el comando solo imprime el plan, y antes de la lista avisa si se va a llevar algún
`.bak` (son los ~39 MB que bajaste vos). El borrado nunca es a ciegas.

Aegis **no** toca, a propósito: la base `SIDC`, el login `app`, la carpeta de SIDC (reportes,
fotos, ejecutable) y los OCX y el runtime de Crystal en `SysWOW64`. Esos últimos los comparte
Windows y otros programas VB6: borrarlos puede romper software que no es SIDC.

---

## Qué hace y qué no hace Aegis

**Hace:** restaura la base, arma el DSN de 32 bits, instala y registra OCX y Crystal, verifica las
cuatro capas, y se desinstala sin llevarse nada ajeno.

**No hace:**

- No instala SQL Server.
- No copia la carpeta de SIDC ni las plantillas de `Reportes/`.
- No descarga el `.bak` ni ningún otro archivo.
- No actualiza la app SIDC (no hay código fuente de la app en este proyecto).
- No borra la base, los datos del cliente ni los controles compartidos de Windows.

---

## Referencia rápida

| Comando | Pide UAC | Sale con 3 cuando… |
| --- | --- | --- |
| `checklist` | No | algo traba la instalación |
| `check` | No | App y DB no se hablan |
| `dashboard` | No | nunca |
| `bak` | No | todavía no hay respaldo |
| `setup-db` | Sí | el restore falló |
| `setup-app` | Sí | el DSN o la verificación fallaron |
| `uninstall` | Sí | un paso del borrado falló |
| `configure` | No | la config no se pudo escribir |
| `menu` | Solo si elegís 0, 1 o 2 | según el paso que corras desde el menú |

Códigos de salida: `0` OK · `1` error general · `2` error de configuración · `3` la máquina no
está lista.
