package models

type User struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	HomeFolder string `json:"homeFolder"`
	Role       string `json:"role"`    // "admin" or "user"
	PwVersion  int    `json:"pwVersion"` // incremented on password change, invalidates old tokens
}

type UsersConfig struct {
	Users []User `json:"users"`
}
