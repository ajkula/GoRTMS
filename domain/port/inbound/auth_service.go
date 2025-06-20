package inbound

import (
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
)

type AuthService interface {
	Login(username, password string) (*model.User, string, error) // user, token, error
	ValidateToken(token string) (*model.User, error)
	CreateUser(username, password string, role model.UserRole) (*model.User, error)
	UpdateUser(userID string, updates UpdateUserRequest, isAdmin bool) (*model.User, error)
	GetUser(username string) (*model.User, bool)
	ListUsers() ([]*model.User, error)
	BootstrapAdmin() (*model.User, string, error) // user, plainPassword, error
	GenerateToken(user *model.User, issuedAt time.Time) (string, error)
}

type UpdateUserRequest struct {
	Username *string         `json:"username,omitempty"`
	Role     *model.UserRole `json:"role,omitempty"`
	Enabled  *bool           `json:"enabled,omitempty"`
}
