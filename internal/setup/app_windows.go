//go:build windows

// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import "os/exec"

func killAppProcess(name string) {
	_ = exec.Command("taskkill", "/F", "/IM", name).Run()
}
