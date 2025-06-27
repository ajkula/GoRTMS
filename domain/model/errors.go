package model

import "errors"

var (
	ErrUserDatabaseNotFound  = errors.New("user database file not found")
	ErrUserDatabaseCorrupted = errors.New("user database file corrupted")
	ErrInvalidChecksum       = errors.New("invalid file checksum")

	// Account request related errors
	ErrAccountRequestNotFound          = errors.New("account request not found")
	ErrAccountRequestAlreadyExists     = errors.New("account request already exists for this username")
	ErrAccountRequestAlreadyReviewed   = errors.New("account request has already been reviewed")
	ErrAccountRequestInvalidStatus     = errors.New("invalid account request status")
	ErrAccountRequestDatabaseNotFound  = errors.New("account request database file not found")
	ErrAccountRequestDatabaseCorrupted = errors.New("account request database file corrupted")
	ErrUsernameAlreadyTaken            = errors.New("username is already taken")
	ErrInvalidRequestedRole            = errors.New("invalid requested role")
)
