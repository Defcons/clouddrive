package models

type User struct {
	Username string `json:"username"`
	// Password is the bcrypt hash. We never serialize this to clients (handlers
	// use PublicUser); the `omitempty` keeps users.json clean for fresh users.
	Password   string `json:"password,omitempty"`
	HomeFolder string `json:"homeFolder"`
	Role       string `json:"role"`
	PwVersion  int    `json:"pwVersion"`

	// MFA (TOTP). MfaSecret is the base32-encoded shared secret; MfaEnabled
	// gates whether login requires a TOTP code. BackupCodes are bcrypt hashes
	// of one-time recovery codes, consumed on use.
	MfaSecret   string   `json:"mfaSecret,omitempty"`
	MfaEnabled  bool     `json:"mfaEnabled,omitempty"`
	BackupCodes []string `json:"backupCodes,omitempty"`
}

// PublicUser is User without the password hash. Use this when serializing to
// clients.
type PublicUser struct {
	Username   string `json:"username"`
	HomeFolder string `json:"homeFolder"`
	Role       string `json:"role"`
}

func (u *User) Public() PublicUser {
	return PublicUser{Username: u.Username, HomeFolder: u.HomeFolder, Role: u.Role}
}

type UsersConfig struct {
	Users []User `json:"users"`
}
