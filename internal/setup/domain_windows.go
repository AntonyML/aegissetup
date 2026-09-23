//go:build windows

// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modnetapi32               = windows.NewLazySystemDLL("netapi32.dll")
	procNetGetJoinInformation = modnetapi32.NewProc("NetGetJoinInformation")
	procNetApiBufferFree      = modnetapi32.NewProc("NetApiBufferFree")
)

// NetJoinStatus representa el estado de unión a dominio de Windows.
type NetJoinStatus uint32

const (
	NetSetupUnknownStatus NetJoinStatus = 0
	NetSetupUnjoined      NetJoinStatus = 1
	NetSetupWorkgroupName NetJoinStatus = 2
	NetSetupDomainName    NetJoinStatus = 3
)

// DetectDomain consulta netapi32.dll para determinar si la máquina está en un dominio.
func DetectDomain() (bool, string, error) {
	var namePtr *uint16
	var status NetJoinStatus

	r0, _, err := procNetGetJoinInformation.Call(
		0,
		uintptr(unsafe.Pointer(&namePtr)),
		uintptr(unsafe.Pointer(&status)),
	)
	if r0 != 0 {
		return false, "", err
	}
	defer procNetApiBufferFree.Call(uintptr(unsafe.Pointer(namePtr)))

	name := windows.UTF16PtrToString(namePtr)
	return status == NetSetupDomainName, name, nil
}
