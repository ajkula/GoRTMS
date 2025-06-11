package model

import "errors"

var (
	ErrUserDatabaseNotFound  = errors.New("user database file not found")
	ErrUserDatabaseCorrupted = errors.New("user database file corrupted")
	ErrInvalidChecksum       = errors.New("invalid file checksum")
)
