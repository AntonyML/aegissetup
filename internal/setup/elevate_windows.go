//go:build windows

// © Antony Monge López — Costa Rica — Céd. 604700548

package setup

import (
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// shellExecuteInfo es SHELLEXECUTEINFOW. El orden y el tamaño de los campos tienen
// que ser exactos: ShellExecuteExW lee esta estructura por puntero y cbSize es su
// único control. Si el layout queda mal, la llamada no falla con un error claro:
// devuelve un código de error de shell genérico.
type shellExecuteInfo struct {
	cbSize       uint32
	fMask        uint32
	hwnd         windows.Handle
	lpVerb       *uint16
	lpFile       *uint16
	lpParameters *uint16
	lpDirectory  *uint16
	nShow        int32
	hInstApp     windows.Handle
	lpIDList     uintptr
	lpClass      *uint16
	hkeyClass    windows.Handle
	dwHotKey     uint32
	hIcon        windows.Handle
	hProcess     windows.Handle
}

const (
	// seeMaskNoCloseProcess es lo que hace que venga hProcess: sin esto no hay
	// forma de saber cuándo terminó el proceso elevado.
	seeMaskNoCloseProcess = 0x00000040
	// seeMaskNoConsole hereda la consola del padre en vez de abrir una nueva: la
	// salida del proceso elevado se lee en la misma ventana.
	seeMaskNoConsole = 0x00008000
	swShowNormal     = 1
)

var procShellExecuteExW = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW")

// isAdminOS pregunta por el token del proceso, que es exactamente lo que mira UAC.
func isAdminOS() bool { return windows.GetCurrentProcessToken().IsElevated() }

// launchElevated lanza exe con el verbo runas (el cartel de UAC) y, si wait,
// espera a que termine para devolver su código de salida.
//
// No se usa un manifiesto requireAdministrator a propósito: este mismo binario
// tiene que poder correr check, checklist y dashboard SIN permisos, que es justo
// cuando más se necesitan (PC recién instalada, sesión remota, usuario sin cuenta
// de administrador). Elevar el ejecutable entero habría dejado el diagnóstico atrás
// de un cartel de UAC.
func launchElevated(exe string, args []string, wait bool) (int, error) {
	if isAdminOS() {
		// Ya tiene permisos: relanzar solo agregaría un cartel de UAC inútil.
		return 0, nil
	}
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return 1, err
	}
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return 1, err
	}
	var params *uint16
	if len(args) > 0 {
		if params, err = windows.UTF16PtrFromString(quoteArgs(args)); err != nil {
			return 1, err
		}
	}
	// Un proceso elevado NO hereda el directorio actual: arranca en system32. Si no
	// se fija acá, una ruta relativa (--config x.json, --bak x.bak) se resuelve en
	// la carpeta equivocada.
	var dir *uint16
	if cwd, err := os.Getwd(); err == nil {
		dir, _ = windows.UTF16PtrFromString(cwd)
	}

	sei := shellExecuteInfo{
		cbSize:       uint32(unsafe.Sizeof(shellExecuteInfo{})),
		fMask:        seeMaskNoCloseProcess | seeMaskNoConsole,
		lpVerb:       verb,
		lpFile:       file,
		lpParameters: params,
		lpDirectory:  dir,
		nShow:        swShowNormal,
	}
	r, _, errno := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&sei)))
	runtime.KeepAlive(sei)
	if r == 0 {
		if errno == windows.ERROR_CANCELLED {
			return 1, fmt.Errorf("el operador canceló el pedido de permisos de administrador")
		}
		return 1, fmt.Errorf("no se pudo pedir permisos de administrador: %v", errno)
	}
	if !wait {
		return 0, nil
	}
	// Sin handle no hay forma de saber qué pasó: ShellExecuteExW solo lo entrega con
	// SEE_MASK_NOCLOSEPROCESS y, si llegara vacío, decir "salió bien" sería inventar
	// el resultado de un trabajo que quizá nunca corrió.
	if sei.hProcess == 0 {
		return 1, fmt.Errorf("el proceso elevado arrancó pero no se pudo esperar: no se sabe si terminó bien")
	}
	defer windows.CloseHandle(sei.hProcess)

	if _, err := windows.WaitForSingleObject(sei.hProcess, windows.INFINITE); err != nil {
		return 1, err
	}
	var code uint32
	if err := windows.GetExitCodeProcess(sei.hProcess, &code); err != nil {
		return 1, err
	}
	return int(code), nil
}
