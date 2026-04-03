package models

type User struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	HomeFolder string `json:"homeFolder"`
	Role       string `json:"role"` // "admin" or "user"
}

type UsersConfig struct {
	Users []User `json:"users"`
}
