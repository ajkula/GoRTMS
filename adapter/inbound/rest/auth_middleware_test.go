package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockAuthService struct {
	mock.Mock
}

func (m *MockAuthService) Login(username, password string) (*model.User, string, error) {
	args := m.Called(username, password)
	if args.Get(0) == nil {
		return nil, args.String(1), args.Error(2)
	}
	return args.Get(0).(*model.User), args.String(1), args.Error(2)
}

func (m *MockAuthService) ValidateToken(token string) (*model.User, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *MockAuthService) CreateUser(username, password string, role model.UserRole) (*model.User, error) {
	args := m.Called(username, password, role)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *MockAuthService) GetUser(username string) (*model.User, bool) {
	args := m.Called(username)
	if args.Get(0) == nil {
		return nil, args.Bool(1)
	}
	return args.Get(0).(*model.User), args.Bool(1)
}

func (m *MockAuthService) ListUsers() ([]*model.User, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.User), args.Error(1)
}

func (m *MockAuthService) BootstrapAdmin() (*model.User, string, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.String(1), args.Error(2)
	}
	return args.Get(0).(*model.User), args.String(1), args.Error(2)
}

type MockAuthLogger struct {
	mock.Mock
}

func (m *MockAuthLogger) UpdateLevel(level string) {
	m.Called(level)
}

func (m *MockAuthLogger) Debug(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *MockAuthLogger) Info(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *MockAuthLogger) Warn(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *MockAuthLogger) Error(msg string, args ...interface{}) {
	m.Called(msg, args)
}

func (m *MockAuthLogger) Shutdown() {}

func createTestUserModel() *model.User {
	return &model.User{
		ID:       "test-id",
		Username: "testuser",
		Role:     model.RoleUser,
		Enabled:  true,
	}
}

func createTestAdminModel() *model.User {
	return &model.User{
		ID:       "admin-id",
		Username: "admin",
		Role:     model.RoleAdmin,
		Enabled:  true,
	}
}

func setupAuthMiddleware() (*AuthMiddleware, *MockAuthService, *MockAuthLogger) {
	authService := &MockAuthService{}
	logger := &MockAuthLogger{}
	logger.On("Warn", mock.Anything, mock.Anything).Return()
	middleware := NewAuthMiddleware(authService, logger)
	return middleware, authService, logger
}

func TestAuthMiddleware_Disabled(t *testing.T) {
	middleware, _, logger := setupAuthMiddleware()
	logger.On("Warn", mock.Anything, mock.Anything).Return()
	middleware.SetEnabled(false)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	req := httptest.NewRequest("GET", "/api/protected", nil)
	w := httptest.NewRecorder()

	middleware.Middleware(handler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

func TestAuthMiddleware_PublicRoute(t *testing.T) {
	middleware, _, _ := setupAuthMiddleware()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	publicRoutes := []string{
		"/api/auth/login",
		"/api/auth/bootstrap",
		"/api/health",
		"/web/index.html",
		"/",
	}

	for _, route := range publicRoutes {
		req := httptest.NewRequest("GET", route, nil)
		w := httptest.NewRecorder()

		middleware.Middleware(handler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Route %s should be public", route)
	}
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	middleware, _, logger := setupAuthMiddleware()
	middleware.SetEnabled(true)

	logger.On("Warn", "Unauthorized access", mock.Anything).Return()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/protected", nil)
	w := httptest.NewRecorder()

	middleware.Middleware(handler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "unauthorized")
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	middleware, authService, logger := setupAuthMiddleware()
	middleware.SetEnabled(true)

	authService.On("ValidateToken", "invalid-token").Return(nil, assert.AnError)
	logger.On("Warn", "Unauthorized access", mock.Anything).Return()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	middleware.Middleware(handler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	middleware, authService, _ := setupAuthMiddleware()
	middleware.SetEnabled(true)
	testUser := createTestUserModel()

	authService.On("ValidateToken", "valid-token").Return(testUser, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := middleware.GetUserFromContext(r.Context())
		assert.NotNil(t, user)
		assert.Equal(t, "testuser", user.Username)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/protected", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	middleware.Middleware(handler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_RequireRole_Success(t *testing.T) {
	middleware, _, _ := setupAuthMiddleware()
	middleware.SetEnabled(true)
	testUser := createTestUserModel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/user-only", nil)
	ctx := context.WithValue(req.Context(), UserContextKey, testUser)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	middleware.RequireRole(model.RoleUser)(handler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_RequireRole_Admin_Success(t *testing.T) {
	middleware, _, _ := setupAuthMiddleware()
	middleware.SetEnabled(true)
	testAdmin := createTestAdminModel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/user-only", nil)
	ctx := context.WithValue(req.Context(), UserContextKey, testAdmin)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	middleware.RequireRole(model.RoleUser)(handler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_RequireRole_Forbidden(t *testing.T) {
	middleware, _, logger := setupAuthMiddleware()
	middleware.SetEnabled(true)
	testUser := createTestUserModel()

	logger.On("Warn", "Forbidden access", mock.Anything).Return()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/admin-only", nil)
	ctx := context.WithValue(req.Context(), UserContextKey, testUser)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	middleware.RequireRole(model.RoleAdmin)(handler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAuthMiddleware_RequireRole_NoUser(t *testing.T) {
	middleware, _, logger := setupAuthMiddleware()
	middleware.SetEnabled(true)

	logger.On("Warn", "Forbidden access", mock.Anything).Return()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/admin-only", nil)
	w := httptest.NewRecorder()

	middleware.RequireRole(model.RoleAdmin)(handler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAuthMiddleware_GetUserFromContext_Success(t *testing.T) {
	middleware, _, _ := setupAuthMiddleware()
	testUser := createTestUserModel()

	ctx := context.WithValue(context.Background(), UserContextKey, testUser)
	user := middleware.GetUserFromContext(ctx)

	assert.NotNil(t, user)
	assert.Equal(t, "testuser", user.Username)
}

func TestAuthMiddleware_GetUserFromContext_NotFound(t *testing.T) {
	middleware, _, _ := setupAuthMiddleware()

	ctx := context.Background()
	user := middleware.GetUserFromContext(ctx)

	assert.Nil(t, user)
}
