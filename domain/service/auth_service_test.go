package service

import (
	"errors"
	"testing"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Save(db *model.UserDatabase) error {
	args := m.Called(db)
	return args.Error(0)
}

func (m *MockUserRepository) Load() (*model.UserDatabase, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.UserDatabase), args.Error(1)
}

func (m *MockUserRepository) Exists() bool {
	args := m.Called()
	return args.Bool(0)
}

type MockCryptoService struct {
	mock.Mock
}

func (m *MockCryptoService) Encrypt(data []byte, key [32]byte) ([]byte, []byte, error) {
	args := m.Called(data, key)
	return args.Get(0).([]byte), args.Get(1).([]byte), args.Error(2)
}

func (m *MockCryptoService) Decrypt(encrypted []byte, nonce []byte, key [32]byte) ([]byte, error) {
	args := m.Called(encrypted, nonce, key)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockCryptoService) DeriveKey(machineID string) [32]byte {
	args := m.Called(machineID)
	return args.Get(0).([32]byte)
}

func (m *MockCryptoService) GenerateSalt() [32]byte {
	args := m.Called()
	return args.Get(0).([32]byte)
}

func (m *MockCryptoService) HashPassword(password string, salt [16]byte) string {
	args := m.Called(password, salt)
	return args.String(0)
}

func (m *MockCryptoService) VerifyPassword(password, hash string, salt [16]byte) bool {
	args := m.Called(password, hash, salt)
	return args.Bool(0)
}

type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) UpdateLevel(level string) {
	m.Called(level)
}

func (m *MockLogger) Debug(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *MockLogger) Info(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *MockLogger) Warn(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *MockLogger) Error(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *MockLogger) Shutdown() {}

func setupAuthService() (*authService, *MockUserRepository, *MockCryptoService, *MockLogger) {
	userRepo := &MockUserRepository{}
	crypto := &MockCryptoService{}
	logger := &MockLogger{}

	service := &authService{
		userRepo:  userRepo,
		crypto:    crypto,
		logger:    logger,
		jwtSecret: "test-secret",
		jwtExpiry: 60 * time.Minute,
	}

	return service, userRepo, crypto, logger
}

func createTestUser() *model.User {
	return &model.User{
		ID:             "test-id",
		Username:       "testuser",
		PasswordHash:   "hashed-password",
		Salt:           [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		Role:           model.RoleUser,
		CreatedAt:      time.Now(),
		LastValidLogin: time.Now(),
		Enabled:        true,
	}
}

func createTestDatabase() *model.UserDatabase {
	user := createTestUser()
	return &model.UserDatabase{
		Users: map[string]*model.User{
			user.Username: user,
		},
		Salt: [32]byte{},
	}
}

func TestAuthService_Login_Success(t *testing.T) {
	service, userRepo, crypto, logger := setupAuthService()
	testDB := createTestDatabase()

	userRepo.On("Load").Return(testDB, nil)
	userRepo.On("Save", mock.Anything).Return(nil)
	crypto.On("VerifyPassword", "password", "hashed-password", mock.Anything).Return(true)
	logger.On("Info", mock.Anything, mock.Anything).Return()

	user, token, err := service.Login("testuser", "password")

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.NotEmpty(t, token)
	assert.Equal(t, "testuser", user.Username)
	userRepo.AssertExpectations(t)
	crypto.AssertExpectations(t)
}

func TestAuthService_Login_InvalidPassword(t *testing.T) {
	service, userRepo, crypto, _ := setupAuthService()
	testDB := createTestDatabase()

	userRepo.On("Load").Return(testDB, nil)
	crypto.On("VerifyPassword", "wrongpassword", "hashed-password", mock.Anything).Return(false)

	user, token, err := service.Login("testuser", "wrongpassword")

	assert.Error(t, err)
	assert.Equal(t, ErrInvalidCredentials, err)
	assert.Nil(t, user)
	assert.Empty(t, token)
}

func TestAuthService_Login_UserNotFound(t *testing.T) {
	service, userRepo, _, _ := setupAuthService()
	testDB := createTestDatabase()

	userRepo.On("Load").Return(testDB, nil)

	user, token, err := service.Login("nonexistent", "password")

	assert.Error(t, err)
	assert.Equal(t, ErrUserNotFound, err)
	assert.Nil(t, user)
	assert.Empty(t, token)
}

func TestAuthService_Login_UserDisabled(t *testing.T) {
	service, userRepo, _, _ := setupAuthService()
	testDB := createTestDatabase()
	testDB.Users["testuser"].Enabled = false

	userRepo.On("Load").Return(testDB, nil)

	user, token, err := service.Login("testuser", "password")

	assert.Error(t, err)
	assert.Equal(t, ErrUserDisabled, err)
	assert.Nil(t, user)
	assert.Empty(t, token)
}

func TestAuthService_CreateUser_Success(t *testing.T) {
	service, userRepo, crypto, logger := setupAuthService()
	testDB := &model.UserDatabase{
		Users: make(map[string]*model.User),
		Salt:  [32]byte{},
	}

	userRepo.On("Load").Return(testDB, nil)
	userRepo.On("Save", mock.Anything).Return(nil)
	crypto.On("HashPassword", "password", mock.Anything).Return("hashed-password")
	logger.On("Info", mock.Anything, mock.Anything).Return()

	user, err := service.CreateUser("newuser", "password", model.RoleUser)

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "newuser", user.Username)
	assert.Equal(t, model.RoleUser, user.Role)
	assert.True(t, user.Enabled)
	userRepo.AssertExpectations(t)
	crypto.AssertExpectations(t)
}

func TestAuthService_CreateUser_UserExists(t *testing.T) {
	service, userRepo, _, _ := setupAuthService()
	testDB := createTestDatabase()

	userRepo.On("Load").Return(testDB, nil)

	user, err := service.CreateUser("testuser", "password", model.RoleUser)

	assert.Error(t, err)
	assert.Equal(t, ErrUserExists, err)
	assert.Nil(t, user)
}

func TestAuthService_ValidateToken_Success(t *testing.T) {
	service, userRepo, _, logger := setupAuthService()
	logger.On("Warn", mock.Anything, mock.Anything).Return()
	testDB := createTestDatabase()

	userRepo.On("Load").Return(testDB, nil).Maybe()

	user := testDB.Users["testuser"]
	user.LastValidLogin = time.Now().Add(-1 * time.Minute)
	token, err := service.GenerateToken(user, time.Now())
	assert.NoError(t, err)

	validatedUser, err := service.ValidateToken(token)

	assert.NoError(t, err)
	assert.NotNil(t, validatedUser)
	assert.Equal(t, "testuser", validatedUser.Username)
}

func TestAuthService_ValidateToken_InvalidToken(t *testing.T) {
	service, _, _, _ := setupAuthService()

	user, err := service.ValidateToken("invalid-token")

	assert.Error(t, err)
	assert.Equal(t, ErrInvalidToken, err)
	assert.Nil(t, user)
}

func TestAuthService_GetUser_Success(t *testing.T) {
	service, userRepo, _, _ := setupAuthService()
	testDB := createTestDatabase()

	userRepo.On("Load").Return(testDB, nil)

	user, exists := service.GetUser("testuser")

	assert.True(t, exists)
	assert.NotNil(t, user)
	assert.Equal(t, "testuser", user.Username)
}

func TestAuthService_GetUser_NotFound(t *testing.T) {
	service, userRepo, _, _ := setupAuthService()
	testDB := createTestDatabase()

	userRepo.On("Load").Return(testDB, nil)

	user, exists := service.GetUser("nonexistent")

	assert.False(t, exists)
	assert.Nil(t, user)
}

func TestAuthService_ListUsers_Success(t *testing.T) {
	service, userRepo, _, _ := setupAuthService()
	testDB := createTestDatabase()

	userRepo.On("Load").Return(testDB, nil)

	users, err := service.ListUsers()

	assert.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, "testuser", users[0].Username)
}

func TestAuthService_BootstrapAdmin_Success(t *testing.T) {
	service, userRepo, crypto, logger := setupAuthService()

	userRepo.On("Load").Return(nil, model.ErrUserDatabaseNotFound)
	userRepo.On("Save", mock.Anything).Return(nil)
	crypto.On("GenerateSalt").Return([32]byte{})
	crypto.On("HashPassword", mock.AnythingOfType("string"), mock.Anything).Return("hashed-password")
	logger.On("Info", mock.Anything, mock.Anything).Return()

	admin, password, err := service.BootstrapAdmin()

	assert.NoError(t, err)
	assert.NotNil(t, admin)
	assert.NotEmpty(t, password)
	assert.Equal(t, "admin", admin.Username)
	assert.Equal(t, model.RoleAdmin, admin.Role)
	userRepo.AssertExpectations(t)
	crypto.AssertExpectations(t)
}

func TestAuthService_BootstrapAdmin_UsersExist(t *testing.T) {
	service, userRepo, _, _ := setupAuthService()
	testDB := createTestDatabase()

	userRepo.On("Load").Return(testDB, nil)

	admin, password, err := service.BootstrapAdmin()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "users already exist")
	assert.Nil(t, admin)
	assert.Empty(t, password)
}

func TestAuthService_LoadDatabase_Error(t *testing.T) {
	service, userRepo, _, _ := setupAuthService()

	userRepo.On("Load").Return(nil, errors.New("database error"))

	user, token, err := service.Login("testuser", "password")

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Empty(t, token)
}

func TestAuthService_LastValidLogin_InvalidatesOldTokens(t *testing.T) {
	service, userRepo, crypto, logger := setupAuthService()
	testDB := createTestDatabase()

	userRepo.On("Load").Return(testDB, nil)
	userRepo.On("Save", mock.Anything).Return(nil)
	crypto.On("VerifyPassword", "password", "hashed-password", mock.Anything).Return(true)
	logger.On("Info", mock.Anything, mock.Anything).Return()
	logger.On("Warn", mock.Anything, mock.Anything).Return()
	logger.On("Debug", mock.Anything, mock.Anything).Return()

	// First login
	_, token1, err := service.Login("testuser", "password")
	assert.NoError(t, err)

	firstLogin := testDB.Users["testuser"].LastValidLogin
	t.Logf("First login time: %v", firstLogin)

	// Wait longer to ensure timestamp difference
	time.Sleep(1 * time.Second)

	// Clear cache
	service.userDatabase = nil

	// Second login
	_, token2, err := service.Login("testuser", "password")
	assert.NoError(t, err)

	secondLogin := testDB.Users["testuser"].LastValidLogin
	t.Logf("Second login time: %v", secondLogin)
	t.Logf("Time difference: %v", secondLogin.Sub(firstLogin))

	// Clear cache for validation
	service.userDatabase = nil

	// Debug token1 validation
	_, err = service.ValidateToken(token1)
	t.Logf("Token1 validation error: %v", err)
	assert.Equal(t, ErrInvalidToken, err)

	// Token2 should be valid
	user, err := service.ValidateToken(token2)
	assert.NoError(t, err)
	if user != nil {
		assert.Equal(t, "testuser", user.Username)
	}
}

func TestAuthService_LastValidLogin_Migration(t *testing.T) {
	service, userRepo, _, _ := setupAuthService()

	user := createTestUser()
	user.LastValidLogin = time.Time{}
	testDB := &model.UserDatabase{
		Users: map[string]*model.User{"testuser": user},
		Salt:  [32]byte{},
	}

	userRepo.On("Load").Return(testDB, nil)

	err := service.loadDatabase()
	assert.NoError(t, err)
	assert.False(t, testDB.Users["testuser"].LastValidLogin.IsZero())
	assert.Equal(t, testDB.Users["testuser"].CreatedAt, testDB.Users["testuser"].LastValidLogin)
}
