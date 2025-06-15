package service

import (
	"crypto/rand"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/inbound"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserNotFound       = errors.New("user not found")
	ErrUserExists         = errors.New("user already exists")
	ErrInvalidToken       = errors.New("invalid token")
	ErrUserDisabled       = errors.New("user disabled")
	ErrFileNotFound       = errors.New("user database file not found")
)

type authService struct {
	userRepo     outbound.UserRepository
	crypto       outbound.CryptoService
	logger       outbound.Logger
	jwtSecret    string
	jwtExpiry    time.Duration
	userDatabase *model.UserDatabase
}

func NewAuthService(
	userRepo outbound.UserRepository,
	crypto outbound.CryptoService,
	logger outbound.Logger,
	jwtSecret string,
	jwtExpiryMinutes int,
) inbound.AuthService {
	return &authService{
		userRepo:  userRepo,
		crypto:    crypto,
		logger:    logger,
		jwtSecret: jwtSecret,
		jwtExpiry: time.Duration(jwtExpiryMinutes) * time.Minute,
	}
}

func (s *authService) Login(username, password string) (*model.User, string, error) {
	if err := s.loadDatabase(); err != nil {
		return nil, "", err
	}

	user, exists := s.userDatabase.Users[username]
	if !exists {
		return nil, "", ErrUserNotFound
	}

	if !user.Enabled {
		return nil, "", ErrUserDisabled
	}

	if !s.crypto.VerifyPassword(password, user.PasswordHash, user.Salt) {
		return nil, "", ErrInvalidCredentials
	}

	now := time.Now().Truncate(time.Second)
	user.LastValidLogin = now
	user.LastLogin = now
	s.saveDatabase()

	token, err := s.generateToken(user, now)
	if err != nil {
		return nil, "", err
	}

	return user, token, nil
}

func (s *authService) ValidateToken(tokenString string) (*model.User, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
		}
		return []byte(s.jwtSecret), nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		username, ok := claims["username"].(string)
		if !ok {
			return nil, ErrInvalidToken
		}

		iatFloat, ok := claims["iat"].(float64)
		if !ok {
			return nil, ErrInvalidToken
		}
		tokeIssuedAt := time.Unix(int64(iatFloat), 0)

		if err := s.loadDatabase(); err != nil {
			return nil, err
		}

		user, exists := s.userDatabase.Users[username]
		if !exists {
			return nil, ErrUserNotFound
		}

		if !user.Enabled {
			return nil, ErrUserDisabled
		}

		if tokeIssuedAt.Before(user.LastValidLogin) {
			return nil, ErrInvalidToken
		}

		return user, nil
	}

	return nil, ErrInvalidToken
}

func (s *authService) CreateUser(username, password string, role model.UserRole) (*model.User, error) {
	if err := s.loadDatabase(); err != nil {
		return nil, err
	}

	if _, exists := s.userDatabase.Users[username]; exists {
		return nil, ErrUserExists
	}

	var salt [16]byte
	rand.Read(salt[:])

	user := &model.User{
		ID:           uuid.New().String(),
		Username:     username,
		PasswordHash: s.crypto.HashPassword(password, salt),
		Salt:         salt,
		Role:         role,
		CreatedAt:    time.Now(),
		Enabled:      true,
	}

	s.userDatabase.Users[username] = user

	if err := s.saveDatabase(); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *authService) GetUser(username string) (*model.User, bool) {
	if err := s.loadDatabase(); err != nil {
		return nil, false
	}

	user, exists := s.userDatabase.Users[username]
	return user, exists
}

func (s *authService) ListUsers() ([]*model.User, error) {
	if err := s.loadDatabase(); err != nil {
		return nil, err
	}

	users := make([]*model.User, 0, len(s.userDatabase.Users))
	for _, user := range s.userDatabase.Users {
		users = append(users, user)
	}

	return users, nil
}

func (s *authService) BootstrapAdmin() (*model.User, string, error) {
	if err := s.loadDatabase(); err != nil && err != ErrFileNotFound {
		return nil, "", err
	}

	if s.userDatabase != nil && len(s.userDatabase.Users) > 0 {
		return nil, "", errors.New("users already exist, bootstrap not needed")
	}

	plainPassword := s.generateSecurePassword()
	admin, err := s.CreateUser("admin", plainPassword, model.RoleAdmin)
	if err != nil {
		return nil, "", err
	}

	s.logger.Info("Bootstrap admin created", "username", admin.Username)
	return admin, plainPassword, nil
}

func (s *authService) loadDatabase() error {
	if s.userDatabase != nil {
		return nil
	}

	db, err := s.userRepo.Load()
	if err != nil {
		if err == model.ErrUserDatabaseNotFound {
			s.userDatabase = &model.UserDatabase{
				Users: make(map[string]*model.User),
				Salt:  s.crypto.GenerateSalt(),
			}
			return nil
		}
		return err
	}

	s.userDatabase = db

	for _, user := range s.userDatabase.Users {
		if user.LastValidLogin.IsZero() {
			user.LastValidLogin = user.CreatedAt
		}
	}
	return nil
}

func (s *authService) saveDatabase() error {
	return s.userRepo.Save(s.userDatabase)
}

func (s *authService) generateToken(user *model.User, issuedAt time.Time) (string, error) {
	claims := jwt.MapClaims{
		"username": user.Username,
		"role":     user.Role,
		"exp":      issuedAt.Add(s.jwtExpiry).Unix(),
		"iat":      issuedAt.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *authService) generateSecurePassword() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	const length = 16

	password := make([]byte, length)
	for i := range password {
		b := make([]byte, 1)
		rand.Read(b)
		password[i] = charset[int(b[0])%len(charset)]
	}

	return string(password)
}
