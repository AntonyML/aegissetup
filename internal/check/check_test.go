// © Antony Monge López — Costa Rica — Céd. 604700548
package check

import (
	"context"
	"strings"
	"testing"

	"aegis-setup/internal/config"
)

func TestCheckNamedInstanceTCP(t *testing.T) {
	cfg := config.Default()
	cfg.Server = `192.168.2.145\SQLEXPRESS`
	cfg.AppDir = t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel right away so SQL dial doesn't wait

	rs := Run(ctx, cfg, "")
	for _, r := range rs {
		if strings.HasPrefix(r.Name, "TCP") {
			if !r.OK {
				t.Errorf("TCP check falló para instancia con nombre: %s", r.Info)
			}
			if !strings.Contains(r.Info, "SQL Browser") {
				t.Errorf("TCP check debe mencionar SQL Browser para instancias con nombre: %s", r.Info)
			}
		}
	}
}
