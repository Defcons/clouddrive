package models

type User struct {
	Username   string `json:"username"`
	// Password is the bcrypt hash. `json:"-"` in outbound responses prevents
	// leaking the hash; `json:"password"` is only used when reading users.json.
	Password   string `json:"password,omitempty"`
	HomeFolder string `json:"homeFolder"`
	Role       string `json:"role"`
	PwVersion  int    `json:"pwVersion"`
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
