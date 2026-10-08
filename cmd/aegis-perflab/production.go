// © Antony Monge López — Costa Rica — Céd. 604700548

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"aegis-setup/internal/setup"
)

const liveRoot = `C:\DEV\SIDC`
const productionHash = "7fac811f1aac0e279f17ce402ab3eadbf5b7868d5904b6de25c621559847e888"

// production es una operación del taller T9: genera desde el EXE vivo conocido,
// nunca desde una copia QA, y no abre conexiones SQL ni ejecuta SIDC.
func production(deploy bool) error {
	source := filepath.Join(liveRoot, setup.SIDCExeName)
	original, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if hash(original) != baselineHash {
		return fmt.Errorf("EXE vivo cambió; no publicar sobre un estado desconocido")
	}
	candidate, err := setup.PatchThirdReportCache(original, 100)
	if err != nil {
		return err
	}
	if hash(candidate) != productionHash || len(candidate) != len(original) {
		return fmt.Errorf("candidato no coincide con revisión independiente")
	}
	differences := 0
	for i := range original {
		if original[i] == candidate[i] {
			continue
		}
		allowed := i >= 0x1b8 && i < 0x1bc || i >= 0xc823e2 && i < 0xc823e8 || i >= 0xea9e60 && i < 0xea9e92
		if !allowed {
			return fmt.Errorf("diferencia fuera del parche verificado en 0x%x", i)
		}
		differences++
	}
	if differences != 52 {
		return fmt.Errorf("cantidad de diferencias inesperada: %d", differences)
	}
	outDir := filepath.Join(labRoot, "produccion")
	backupDir := filepath.Join(outDir, "respaldo")
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return err
	}
	if err := writeExclusiveOrVerify(filepath.Join(backupDir, "SIDC.before.bin"), original); err != nil {
		return err
	}
	manifestPath := filepath.Join(liveRoot, "AegisSetup.manifest.json")
	manifestBefore, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	if err := writeExclusiveOrVerify(filepath.Join(backupDir, "AegisSetup.manifest.before.json"), manifestBefore); err != nil {
		return err
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestBefore, &manifest); err != nil {
		return err
	}
	executables, ok := manifest["executables"].([]any)
	if !ok {
		return fmt.Errorf("manifiesto vivo no tiene lista de ejecutables")
	}
	matched := false
	for _, value := range executables {
		entry, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("entrada de manifiesto inválida")
		}
		if entry["name"] == setup.SIDCExeName {
			if entry["sha256"] != baselineHash {
				return fmt.Errorf("manifiesto vivo no coincide con EXE previo")
			}
			entry["sha256"] = productionHash
			entry["bytes"] = len(candidate)
			matched = true
		}
	}
	if !matched {
		return fmt.Errorf("EXE vivo no figura en manifiesto")
	}
	manifest["legacy_performance_profile"] = "third-report-cache-100"
	manifestAfter, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	manifestAfter = append(manifestAfter, '\n')
	if err := writeExclusiveOrVerify(filepath.Join(outDir, setup.SIDCExeName), candidate); err != nil {
		return err
	}
	if err := writeExclusiveOrVerify(filepath.Join(outDir, "AegisSetup.manifest.json"), manifestAfter); err != nil {
		return err
	}
	validation, err := json.MarshalIndent(map[string]any{"before_sha256": baselineHash, "after_sha256": productionHash, "bytes": len(candidate), "changed_bytes": differences, "outside_patch_identical": true, "connection_identical": true, "production_sql_executed": false, "cache_size": 100, "deployed": deploy}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "validacion.json"), validation, 0600); err != nil {
		return err
	}
	if deploy {
		// La sustitución nunca borra el destino para forzar un rename. Si Windows
		// lo tiene abierto, falla dejando la instalación anterior intacta.
		if err := replaceFile(source, candidate); err != nil {
			return err
		}
		if err := replaceFile(manifestPath, manifestAfter); err != nil {
			rollbackErr := replaceFile(source, original)
			return fmt.Errorf("publicación de manifiesto falló; reversión EXE: %v; error: %w", rollbackErr, err)
		}
		readback, err := os.ReadFile(source)
		if err != nil || !bytes.Equal(readback, candidate) {
			return fmt.Errorf("readback EXE desplegado falló")
		}
	}
	fmt.Printf("Producción: preparado=%t instalado=%t bytes=%d diferencias=%d conexión idéntica=true SHA256=%s\n", true, deploy, len(candidate), differences, productionHash)
	return nil
}

func writeExclusiveOrVerify(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		old, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if !bytes.Equal(old, data) {
			return fmt.Errorf("no sobrescribir respaldo/artefacto diferente: %s", path)
		}
		return nil
	}
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func replaceFile(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".aegis-t9-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
