// © Antony Monge López — Costa Rica — Céd. 604700548

// Generador experimental Aegis exclusivo del laboratorio local T9.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf16"

	"aegis-setup/internal/setup"
)

const labRoot = `C:\DEV\_wt\sidc-tercer-informe-lab`
const localConnection = `Driver={SQL Server};Server=.\SQLEXPRESS;Database=SIDC;Trusted_Connection=Yes;`
const referenceConnection = `Driver={SQL Server};Server=.\SQLEXPRESS;Database=S9B;Trusted_Connection=Yes;`
const baselineHash = "7515e4e96886f76031514d128d87b0f66baed4581e29d688ccac745f5b56891f"

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func run() error {
	data, err := os.ReadFile(filepath.Join(labRoot, "app", "SIDC.baseline.bin"))
	if err != nil {
		return err
	}
	if hash(data) != baselineHash {
		return fmt.Errorf("baseline no coincide con versión autorizada")
	}
	marker := []byte{}
	for _, c := range utf16.Encode([]rune("Provider=MSDASQL")) {
		marker = append(marker, byte(c), byte(c>>8))
	}
	slot := bytes.Index(data, marker)
	if slot < 0 {
		return fmt.Errorf("no se encontró conexión original")
	}
	local, _, err := setup.PatchExeBuffer(data, localConnection, nil)
	if err != nil {
		return err
	}
	chars := utf16.Encode([]rune(localConnection))
	expected := make([]byte, setup.SlotExeConnBytes)
	for i, c := range chars {
		expected[2*i], expected[2*i+1] = byte(c), byte(c>>8)
	}
	if !bytes.Equal(local[slot:slot+len(expected)], expected) {
		return fmt.Errorf("destino local no verificado")
	}
	perf, err := setup.PatchThirdReportCache(data, 100)
	if err != nil {
		return err
	}
	patched, _, err := setup.PatchExeBuffer(perf, localConnection, nil)
	if err != nil {
		return err
	}
	if !bytes.Equal(patched[slot:slot+len(expected)], expected) {
		return fmt.Errorf("destino local del candidato no verificado")
	}
	variants := []struct {
		name string
		data []byte
	}{{"qa-baseline", local}, {"qa-cache100", patched}}
	if len(os.Args) == 2 && os.Args[1] == "reference" {
		ref, _, err := setup.PatchExeBuffer(data, referenceConnection, nil)
		if err != nil {
			return err
		}
		refExpected := make([]byte, setup.SlotExeConnBytes)
		for i, c := range utf16.Encode([]rune(referenceConnection)) {
			refExpected[2*i], refExpected[2*i+1] = byte(c), byte(c>>8)
		}
		if !bytes.Equal(ref[slot:slot+len(refExpected)], refExpected) {
			return fmt.Errorf("destino local de referencia no verificado")
		}
		variants = []struct {
			name string
			data []byte
		}{{"qa-reference", ref}}
	} else if len(os.Args) != 1 {
		return fmt.Errorf("usá sin argumentos o reference")
	}
	for _, variant := range variants {
		dir := filepath.Join(labRoot, variant.name)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
		out := filepath.Join(dir, "SIDC_QA.exe")
		f, err := os.OpenFile(out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, err = f.Write(variant.data)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		manifest, err := json.MarshalIndent(map[string]any{"sha256": hash(variant.data), "source_sha256": baselineHash, "size": len(variant.data), "sql_instance": "local SQLEXPRESS", "windows_auth": true, "connection_slot_verified": true, "production_ready": false}, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "manifest.json"), manifest, 0600); err != nil {
			return err
		}
		fmt.Printf("%s: %d bytes, SHA256 %s, destino local verificado\n", variant.name, len(variant.data), hash(variant.data))
	}
	return nil
}

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "production" || os.Args[1] == "deploy") {
		if err := production(os.Args[1] == "deploy"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
