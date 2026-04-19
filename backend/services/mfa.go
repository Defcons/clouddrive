package services

import (
	"bytes"
	"clouddrive/models"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image/png"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

const (
	// issuerName shows up in authenticator apps as the account issuer.
	issuerName = "CloudDrive"
	// backupCodeCount is the number of one-time recovery codes issued.
	backupCodeCount = 10
)

// MfaEnrollment is the result of starting MFA setup — returned to the client
// so they can scan the QR code and confirm with the first TOTP code.
type MfaEnrollment struct {
	Secret    string `json:"secret"`    // base32-encoded, shown to user for manual entry
	QRCodeDataURL string `json:"qrCodeDataUrl"` // data:image/png;base64,... for <img src>
	OTPAuthURL    string `json:"otpAuthUrl"`    // raw otpauth:// URL if client wants its own QR
}

// GenerateEnrollment creates a new TOTP secret for username without committing
// it. The caller must call ConfirmEnrollment after the user verifies the first
// code.
func (s *UserStore) GenerateEnrollment(username string) (*MfaEnrollment, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuerName,
		AccountName: username,
	})
	if err != nil {
		return nil, fmt.Errorf("totp generate: %w", err)
	}

	// Encode QR code as PNG → data URL for easy <img src> embedding.
	img, err := key.Image(240, 240)
	if err != nil {
		return nil, fmt.Errorf("qr image: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("qr encode: %w", err)
	}
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	return &MfaEnrollment{
		Secret:        key.Secret(),
		QRCodeDataURL: dataURL,
		OTPAuthURL:    key.URL(),
	}, nil
}

// ConfirmEnrollment validates the first TOTP code against the proposed secret
// and, on success, persists the secret, marks MFA enabled, and issues fresh
// backup codes. Returns the plaintext backup codes (shown once — never again).
func (s *UserStore) ConfirmEnrollment(username, secret, totpCode string) ([]string, error) {
	if !totp.Validate(totpCode, secret) {
		return nil, fmt.Errorf("invalid code")
	}

	plainCodes, hashedCodes, err := generateBackupCodes()
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.findIndexLocked(username)
	if idx < 0 {
		return nil, fmt.Errorf("user not found")
	}
	s.users[idx].MfaSecret = secret
	s.users[idx].MfaEnabled = true
	s.users[idx].BackupCodes = hashedCodes
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return plainCodes, nil
}

// DisableMFA removes MFA state from the user. Caller must have verified the
// user's password already (or admin role).
func (s *UserStore) DisableMFA(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.findIndexLocked(username)
	if idx < 0 {
		return fmt.Errorf("user not found")
	}
	s.users[idx].MfaEnabled = false
	s.users[idx].MfaSecret = ""
	s.users[idx].BackupCodes = nil
	return s.saveLocked()
}

// ValidateTOTP returns true if code is a valid TOTP for the user's secret
// right now (with ±30s drift window handled by the otp library).
func (s *UserStore) ValidateTOTP(username, code string) bool {
	s.mu.RLock()
	u := s.getUserLocked(username)
	s.mu.RUnlock()
	if u == nil || !u.MfaEnabled || u.MfaSecret == "" {
		return false
	}
	ok, _ := totp.ValidateCustom(code, u.MfaSecret, time.Now().UTC(), totp.ValidateOpts{
		Period:    30,
		Skew:      1, // accept one 30s step before and after, for clock drift
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	return ok
}

// ValidateBackupCode returns true if code matches an unused backup code,
// AND consumes it so it can't be used again. This is atomic.
func (s *UserStore) ValidateBackupCode(username, code string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.findIndexLocked(username)
	if idx < 0 {
		return false
	}
	normalized := normalizeBackupCode(code)
	if normalized == "" {
		return false
	}
	for i, hash := range s.users[idx].BackupCodes {
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(normalized)) == nil {
			// Consume this code.
			s.users[idx].BackupCodes = append(s.users[idx].BackupCodes[:i], s.users[idx].BackupCodes[i+1:]...)
			_ = s.saveLocked()
			return true
		}
	}
	return false
}

// RegenerateBackupCodes issues a fresh set of 10 codes, invalidating the old
// ones. Returns the plaintext codes for the user to save.
func (s *UserStore) RegenerateBackupCodes(username string) ([]string, error) {
	plainCodes, hashedCodes, err := generateBackupCodes()
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.findIndexLocked(username)
	if idx < 0 {
		return nil, fmt.Errorf("user not found")
	}
	s.users[idx].BackupCodes = hashedCodes
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return plainCodes, nil
}

// MfaStatus returns whether MFA is enabled and how many backup codes remain.
func (s *UserStore) MfaStatus(username string) (enabled bool, backupCodesRemaining int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u := s.getUserLocked(username)
	if u == nil {
		return false, 0
	}
	return u.MfaEnabled, len(u.BackupCodes)
}

// ---- internal helpers ----

func (s *UserStore) findIndexLocked(username string) int {
	for i, u := range s.users {
		if u.Username == username {
			return i
		}
	}
	return -1
}

func (s *UserStore) getUserLocked(username string) *models.User {
	for i, u := range s.users {
		if u.Username == username {
			return &s.users[i]
		}
	}
	return nil
}

// generateBackupCodes returns 10 codes (plain + bcrypt hashed).
// Each code is 8 hex chars (32 bits) — shown in the format "abcd-1234" for
// legibility. The hash is of the normalized (no-dash, lowercase) form.
func generateBackupCodes() (plain []string, hashed []string, err error) {
	plain = make([]string, backupCodeCount)
	hashed = make([]string, backupCodeCount)
	for i := 0; i < backupCodeCount; i++ {
		b := make([]byte, 4)
		if _, err := rand.Read(b); err != nil {
			return nil, nil, err
		}
		raw := hex.EncodeToString(b)
		plain[i] = raw[:4] + "-" + raw[4:]
		h, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
		if err != nil {
			return nil, nil, err
		}
		hashed[i] = string(h)
	}
	return plain, hashed, nil
}

func normalizeBackupCode(code string) string {
	out := make([]byte, 0, len(code))
	for _, r := range code {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			out = append(out, byte(r))
		} else if r >= 'A' && r <= 'F' {
			out = append(out, byte(r+('a'-'A')))
		}
	}
	if len(out) != 8 {
		return ""
	}
	return string(out)
}

