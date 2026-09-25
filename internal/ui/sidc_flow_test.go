// © Antony Monge López — Costa Rica — Céd. 604700548
package ui

import (
	"errors"
	"strings"
	"testing"
)

var errTestTUI = errors.New("error de prueba")

func TestInstalacionCompletaOfreceAbrirSIDC(t *testing.T) {
	previous := launchSIDC
	t.Cleanup(func() { launchSIDC = previous })

	cfg := prodCfg()
	cfg.AppDir = t.TempDir()
	calledWith := ""
	launchSIDC = func(appDir string) error {
		calledWith = appDir
		return nil
	}

	m := NewModel(cfg, "")
	out, _ := m.Update(taskFinishedMsg{kind: taskInstall, lines: []string{"INSTALACIÓN COMPLETADA EXITOSAMENTE."}})
	m = out.(Model)
	if !m.readyToOpen {
		t.Fatal("la instalación completa exitosa no habilitó abrir SIDC")
	}
	if !strings.Contains(m.doneBody(), "abrirlo y cerrar Aegis") {
		t.Fatal("la pantalla final no explica cómo abrir SIDC")
	}

	out, cmd := m.Update(tecla("a"))
	if calledWith != cfg.AppDir {
		t.Fatalf("app_dir enviado = %q, quiero %q", calledWith, cfg.AppDir)
	}
	if cmd == nil {
		t.Fatal("abrir SIDC no devolvió la orden de cerrar Aegis")
	}
	if out.(Model).taskErr != nil {
		t.Fatalf("abrir SIDC dejó error: %v", out.(Model).taskErr)
	}
}

func TestInstalacionFallidaNoOfreceAbrirSIDC(t *testing.T) {
	m := NewModel(prodCfg(), "")
	out, _ := m.Update(taskFinishedMsg{kind: taskInstall, err: errTestTUI})
	m = out.(Model)
	if m.readyToOpen {
		t.Fatal("una instalación fallida habilitó abrir SIDC")
	}
	if strings.Contains(m.doneBody(), "abrirlo y cerrar Aegis") {
		t.Fatal("una instalación fallida mostró la acción de abrir SIDC")
	}
}
