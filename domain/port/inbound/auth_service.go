package inbound

import "github.com/ajkula/GoRTMS/domain/model"

type AuthService interface {
	Login(username, password string) (*model.User, string, error) // user, token, error
	ValidateToken(token string) (*model.User, error)
	CreateUser(username, password string, role model.UserRole) (*model.User, error)
	GetUser(username string) (*model.User, bool)
	ListUsers() ([]*model.User, error)
	BootstrapAdmin() (*model.User, string, error) // user, plainPassword, error
}
