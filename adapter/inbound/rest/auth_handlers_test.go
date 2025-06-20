package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ajkula/GoRTMS/config"
	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupAuthHandler() (*AuthHandler, *MockAuthService, *MockAuthLogger) {
	authService := &MockAuthService{}
	logger := &MockAuthLogger{}
	handler := NewAuthHandler(authService, logger)
	return handler, authService, logger
}

func TestAuthHandler_Login_Success(t *testing.T) {
	handler, authService, logger := setupAuthHandler()
	testUser := createTestUserModel()

	authService.On("Login", "testuser", "password").Return(testUser, "test-token", nil)
	logger.On("Info", "User logged in", mock.Anything).Return()

	reqBody := LoginRequest{
		Username: "testuser",
		Password: "password",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response UserApiResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "testuser", response.User.Username)
	assert.Equal(t, "test-token", response.Token)
}

func TestAuthHandler_Login_InvalidCredentials(t *testing.T) {
	handler, authService, logger := setupAuthHandler()

	authService.On("Login", "testuser", "wrongpassword").Return(nil, "", assert.AnError)
	logger.On("Warn", "Login failed", mock.Anything).Return()

	reqBody := LoginRequest{
		Username: "testuser",
		Password: "wrongpassword",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthHandler_Login_MissingFields(t *testing.T) {
	handler, _, _ := setupAuthHandler()

	reqBody := LoginRequest{
		Username: "",
		Password: "password",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthHandler_Login_InvalidJSON(t *testing.T) {
	handler, _, logger := setupAuthHandler()

	logger.On("Error", "Failed to decode login request", mock.Anything).Return()

	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer([]byte("invalid-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthHandler_CreateUser_Success(t *testing.T) {
	handler, authService, logger := setupAuthHandler()
	testUser := createTestUserModel()

	authService.On("CreateUser", "newuser", "password", model.RoleUser).Return(testUser, nil)
	logger.On("Info", "User created", mock.Anything).Return()

	reqBody := CreateUserRequest{
		Username: "newuser",
		Password: "password",
		Role:     model.RoleUser,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/admin/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CreateUser(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response UserApiResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "testuser", response.User.Username)
}

func TestAuthHandler_CreateUser_DefaultRole(t *testing.T) {
	handler, authService, logger := setupAuthHandler()
	testUser := createTestUserModel()

	authService.On("CreateUser", "newuser", "password", model.RoleUser).Return(testUser, nil)
	logger.On("Info", "User created", mock.Anything).Return()

	reqBody := CreateUserRequest{
		Username: "newuser",
		Password: "password",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/admin/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CreateUser(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthHandler_CreateUser_UserExists(t *testing.T) {
	handler, authService, logger := setupAuthHandler()

	authService.On("ListUsers").Return([]*model.User{}, nil)
	authService.On("CreateUser", "existinguser", "password", model.RoleUser).Return(nil, assert.AnError)
	logger.On("Error", "failed to create user", mock.Anything).Return()

	reqBody := CreateUserRequest{
		Username: "existinguser",
		Password: "password",
		Role:     model.RoleUser,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/admin/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CreateUser(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthHandler_ListUsers_Success(t *testing.T) {
	handler, authService, _ := setupAuthHandler()
	testUsers := []*model.User{createTestUserModel()}

	authService.On("ListUsers").Return(testUsers, nil)

	req := httptest.NewRequest("GET", "/api/admin/users", nil)
	w := httptest.NewRecorder()

	handler.ListUsers(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []*model.User
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Len(t, response, 1)
	assert.Equal(t, "testuser", response[0].Username)
}

func TestAuthHandler_ListUsers_Error(t *testing.T) {
	handler, authService, logger := setupAuthHandler()

	authService.On("ListUsers").Return(nil, assert.AnError)
	logger.On("Error", "Failed to list users", mock.Anything).Return()

	req := httptest.NewRequest("GET", "/api/admin/users", nil)
	w := httptest.NewRecorder()

	handler.ListUsers(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAuthHandler_Bootstrap_Success(t *testing.T) {
	handler, authService, logger := setupAuthHandler()
	testAdmin := createTestAdminModel()

	authService.On("ListUsers").Return([]*model.User{}, nil)
	authService.On("BootstrapAdmin").Return(testAdmin, "generated-password", nil)
	logger.On("Info", "Admin bootstrapped", mock.Anything).Return()

	req := httptest.NewRequest("POST", "/api/auth/bootstrap", nil)
	w := httptest.NewRecorder()

	handler.Bootstrap(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response BootstrapResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "admin", response.Admin.Username)
	assert.Equal(t, "generated-password", response.Password)
	assert.Contains(t, response.Message, "Save this password")
}

func TestAuthHandler_Bootstrap_AlreadyExists(t *testing.T) {
	handler, authService, logger := setupAuthHandler()

	authService.On("ListUsers").Return([]*model.User{}, nil)
	authService.On("BootstrapAdmin").Return(nil, "", assert.AnError)
	logger.On("Error", "Bootstrap failed", mock.Anything).Return()

	req := httptest.NewRequest("POST", "/api/auth/bootstrap", nil)
	w := httptest.NewRecorder()

	handler.Bootstrap(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthHandler_GetProfile_Success(t *testing.T) {
	handler, authService, logger := setupAuthHandler()
	logger.On("Warn", mock.Anything, mock.Anything).Return()
	testUser := createTestUserModel()

	req := httptest.NewRequest("GET", "/api/auth/profile", nil)
	_ = NewAuthMiddleware(authService, &MockAuthLogger{}, config.DefaultConfig())
	ctx := context.WithValue(req.Context(), UserContextKey, testUser)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.GetProfile(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response model.User
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "testuser", response.Username)
}

func TestAuthHandler_GetProfile_UserNotFound(t *testing.T) {
	handler, _, logger := setupAuthHandler()
	logger.On("Warn", mock.Anything, mock.Anything).Return()

	req := httptest.NewRequest("GET", "/api/auth/profile", nil)
	w := httptest.NewRecorder()

	handler.GetProfile(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
