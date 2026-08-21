package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveJWTSecretFromEnv(t *testing.T) {
	const strong = "this-is-a-sufficiently-long-secret-value-xx"
	t.Setenv("JWT_SECRET", strong)
	got, err := resolveJWTSecret(t.TempDir())
	if err != nil {
		t.Fatalf("valid env secret: %v", err)
	}
	if got != strong {
		t.Errorf("env secret should be used verbatim, got %q", got)
	}
}

func TestResolveJWTSecretRejectsWeakEnv(t *testing.T) {
	t.Setenv("JWT_SECRET", "tooshort")
	if _, err := resolveJWTSecret(t.TempDir()); err == nil {
		t.Error("a <32-char JWT_SECRET should be rejected")
	}
	t.Setenv("JWT_SECRET", "change-me-in-production")
	if _, err := resolveJWTSecret(t.TempDir()); err == nil {
		t.Error("the placeholder default should be rejected")
	}
}

func TestResolveJWTSecretGeneratesAndPersists(t *testing.T) {
	t.Setenv("JWT_SECRET", "") // empty == unset for our purposes → generate
	dir := t.TempDir()

	first, err := resolveJWTSecret(dir)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(first) < 32 {
		t.Errorf("generated secret too short: %d chars", len(first))
	}

	// Persisted to <dir>/.jwt_secret.
	path := filepath.Join(dir, ".jwt_secret")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf(".jwt_secret should be written: %v", err)
	}

	// A second call returns the SAME persisted secret (stable across restarts).
	second, err := resolveJWTSecret(dir)
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Errorf("secret should be stable across calls: %q != %q", first, second)
	}
}
