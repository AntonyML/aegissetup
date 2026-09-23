// © Antony Monge López — Costa Rica — Céd. 604700548
package securestore

import (
	"os"
	"testing"
)

func TestSaveAndGetPassword(t *testing.T) {
	// Clean before
	_ = DeletePassword()
	defer DeletePassword()

	testPass := "  MySecretP@ss#2026!  "
	expected := "MySecretP@ss#2026!"

	if err := SavePassword(testPass); err != nil {
		t.Fatalf("SavePassword falló: %v", err)
	}

	if !HasPassword() {
		t.Errorf("HasPassword() esperaba true, dio false")
	}

	got, err := GetPassword()
	if err != nil {
		t.Fatalf("GetPassword falló: %v", err)
	}
	if got != expected {
		t.Errorf("GetPassword() = %q, esperado %q", got, expected)
	}

	if err := DeletePassword(); err != nil {
		t.Fatalf("DeletePassword falló: %v", err)
	}

	if HasPassword() {
		t.Errorf("HasPassword() esperaba false después de DeletePassword, dio true")
	}
}

func TestResolvePasswordPrecedence(t *testing.T) {
	_ = DeletePassword()
	defer DeletePassword()

	// 1. Explicit takes priority
	p := ResolvePassword("explicit123", nil)
	if p != "explicit123" {
		t.Errorf("esperaba explicit123, dio %s", p)
	}

	// 2. Env takes priority over stored
	_ = SavePassword("stored456")
	_ = os.Setenv(envSQLPassword, "env789")
	defer os.Unsetenv(envSQLPassword)

	p = ResolvePassword("", nil)
	if p != "env789" {
		t.Errorf("esperaba env789, dio %s", p)
	}

	// 3. Stored takes priority over DSN fallback
	_ = os.Unsetenv(envSQLPassword)
	p = ResolvePassword("", func() string { return "dsnFallback" })
	if p != "stored456" {
		t.Errorf("esperaba stored456, dio %s", p)
	}

	// 4. DSN fallback when nothing else
	_ = DeletePassword()
	p = ResolvePassword("", func() string { return "dsnFallback" })
	if p != "dsnFallback" {
		t.Errorf("esperaba dsnFallback, dio %s", p)
	}
}
