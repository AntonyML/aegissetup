// © Antony Monge López — Costa Rica — Céd. 604700548
package check

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func sampleReport(repDir string) (string, error) {
	ents, err := os.ReadDir(repDir)
	if err != nil {
		return "", err
	}
	var names []string
	for _, ent := range ents {
		if !ent.IsDir() && strings.EqualFold(filepath.Ext(ent.Name()), ".rpt") {
			names = append(names, ent.Name())
		}
	}
	if len(names) == 0 {
		return "", fmt.Errorf("no hay plantillas .rpt")
	}
	sort.Strings(names)
	return filepath.Join(repDir, names[0]), nil
}
