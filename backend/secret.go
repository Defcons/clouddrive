package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// resolveJWTSecret returns the secret used to sign session JWTs.
//
// Precedence:
//  1. JWT_SECRET env var, if set — used verbatim after validation (>=32 chars,
//     not the old placeholder default). An explicitly-set weak secret is a hard
//     error, never silently replaced.
//  2. A secret persisted at <storageRoot>/.jwt_secret from a previous run.
//  3. Otherwise a fresh 32-byte random secret is generated, persisted to
//     <storageRoot>/.jwt_secret (mode 0600), and used.
//
// This lets a first run work with zero configuration while still giving every
// install a strong, unique, stable secret. The file name starts with a dot, so
// the file handlers already exclude it from listing/search/disk.
func resolveJWTSecret(storageRoot string) (string, error) {
	if env := os.Getenv("JWT_SECRET"); env != "" {
		if env == "change-me-in-production" {
			return "", fmt.Errorf("JWT_SECRET is set to the placeholder default; use a strong random value (min 32 chars)")
		}
		if len(env) < 32 {
			return "", fmt.Errorf("JWT_SECRET is too short; use at least 32 random characters")
		}
		return env, nil
	}

	secretPath := filepath.Join(storageRoot, ".jwt_secret")
	if data, err := os.ReadFile(secretPath); err == nil {
		if s := strings.TrimSpace(string(data)); len(s) >= 32 {
			return s, nil
		}
		// Present but unusable (empty/short/corrupt) — fall through and regenerate.
		slog.Warn("existing .jwt_secret is empty or too short; regenerating", "path", secretPath)
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate JWT secret: %w", err)
	}
	secret := hex.EncodeToString(buf) // 64 hex chars

	if err := os.MkdirAll(storageRoot, 0o755); err != nil {
		return "", fmt.Errorf("failed to create storage root %q: %w", storageRoot, err)
	}
	if err := os.WriteFile(secretPath, []byte(secret), 0o600); err != nil {
		return "", fmt.Errorf("failed to persist JWT secret to %q: %w", secretPath, err)
	}
	slog.Info("generated a new random JWT secret (first run)", "path", secretPath)
	return secret, nil
}
