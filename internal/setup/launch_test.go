// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"strings"
	"testing"
)

func TestLaunchSIDCRechazaCarpetaSinEjecutablePreparado(t *testing.T) {
	err := LaunchSIDC(t.TempDir())
	if err == nil {
		t.Fatal("LaunchSIDC devolvió nil sin ejecutable preparado")
	}
	if !strings.Contains(err.Error(), SIDCExeName) {
		t.Fatalf("error = %q, falta el nombre del ejecutable preparado", err)
	}
}
