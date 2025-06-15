package model

import (
	"time"
)

type UserRole string

const (
	RoleAdmin UserRole = "admin"
	RoleUser  UserRole = "user"
)

type User struct {
	ID             string    `json:"id"`
	Username       string    `json:"username"`
	PasswordHash   string    `json:"passwordHash"`
	Salt           [16]byte  `json:"salt"`
	Role           UserRole  `json:"role"`
	CreatedAt      time.Time `json:"createdAt"`
	LastLogin      time.Time `json:"lastLogin"`
	LastValidLogin time.Time `json:"lastValidLogin"`
	Enabled        bool      `json:"enabled"`
}

type UserDatabase struct {
	Users map[string]*User `json:"users"`
	Salt  [32]byte         `json:"salt"`
}
