package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ajkula/GoRTMS/config"
	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/inbound"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

// Helper to create test logger
func createTestLogger() outbound.Logger {
	return &mockLogger{}
}

// Helper to create mock auth service
func createMockAuthService() inbound.AuthService {
	return &mockAuthService{
		users: make(map[string]*model.User),
	}
}

func TestHybridMiddleware_HMACRouting(t *testing.T) {
	// Setup real middlewares
	logger := createTestLogger()
	repo := createTestRepository(t, logger)
	cfg := config.DefaultConfig()
	cfg.Security.EnableAuthentication = true

	hmacMiddleware := NewHMACMiddleware(repo, logger, cfg)
	jwtMiddleware := NewAuthMiddleware(nil, logger, cfg)
	hybrid := NewHybridMiddleware(cfg, hmacMiddleware, jwtMiddleware, logger)

	// Create test service
	service := createTestService()
	repo.Create(context.Background(), service)

	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true

		// Verify HMAC service is in context (means HMAC middleware was used)
		if ctx := hmacMiddleware.GetServiceFromContext(r.Context()); ctx != nil {
			t.Logf("HMAC service found in context: %s", ctx.ID)
		} else {
			t.Error("Expected HMAC service in context")
		}

		w.WriteHeader(http.StatusOK)
	})

	// Create request with HMAC headers
	body := `{"test": "data"}`
	req := createTestRequest("POST", "/api/test", body, service)
	w := httptest.NewRecorder()

	// Execute
	hybrid.Middleware(testHandler).ServeHTTP(w, req)

	// Verify routing worked
	if !handlerCalled {
		t.Error("Expected handler to be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestHybridMiddleware_JWTRouting(t *testing.T) {
	// Setup
	logger := createTestLogger()
	repo := createTestRepository(t, logger)

	cfg := config.DefaultConfig()
	cfg.Security.EnableAuthentication = true

	hmacMiddleware := NewHMACMiddleware(repo, logger, cfg)
	authService := createMockAuthService()
	jwtMiddleware := NewAuthMiddleware(authService, logger, cfg)
	hybrid := NewHybridMiddleware(cfg, hmacMiddleware, jwtMiddleware, logger)

	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true

		// Verify JWT user is in context (means JWT middleware was used)
		if user := GetUserFromContext(r.Context()); user != nil {
			t.Logf("JWT user found in context: %s", user.Username)
		} else {
			t.Log("No user in context - JWT middleware might be disabled or request failed validation")
		}

		w.WriteHeader(http.StatusOK)
	})

	// Create request with JWT Authorization header (no HMAC headers)
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer mock-jwt-token")
	w := httptest.NewRecorder()

	// Execute
	hybrid.Middleware(testHandler).ServeHTTP(w, req)

	// Verify routing worked - handler should be called
	if !handlerCalled {
		t.Error("Expected handler to be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestHybridMiddleware_PartialHMACHeaders(t *testing.T) {
	// Setup
	logger := createTestLogger()
	repo := createTestRepository(t, logger)

	cfg := config.DefaultConfig()
	cfg.Security.EnableAuthentication = true

	hmacMiddleware := NewHMACMiddleware(repo, logger, cfg)
	authService := createMockAuthService()
	jwtMiddleware := NewAuthMiddleware(authService, logger, cfg)
	hybrid := NewHybridMiddleware(cfg, hmacMiddleware, jwtMiddleware, logger)

	testCases := []struct {
		name    string
		headers map[string]string
	}{
		{
			name: "Only Service ID",
			headers: map[string]string{
				"X-Service-ID": "test-service",
			},
		},
		{
			name: "Service ID + Timestamp",
			headers: map[string]string{
				"X-Service-ID": "test-service",
				"X-Timestamp":  time.Now().Format(time.RFC3339),
			},
		},
		{
			name: "Service ID + Signature",
			headers: map[string]string{
				"X-Service-ID": "test-service",
				"X-Signature":  "sha256=test",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handlerCalled := false
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest("POST", "/api/test", nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			w := httptest.NewRecorder()

			hybrid.Middleware(testHandler).ServeHTTP(w, req)

			// Should route to JWT when HMAC headers are incomplete
			// Since JWT is disabled, should pass through to handler
			if handlerCalled {
				t.Error("Expected handler to NOT be called with incomplete HMAC headers (auth should reject)")
			}

			// Verify authentication method detection
			authMethod := hybrid.GetAuthenticationMethod(req)
			if authMethod != "JWT" {
				t.Errorf("Expected JWT authentication method for incomplete headers, got %s", authMethod)
			}
		})
	}
}

func TestHybridMiddleware_Disabled(t *testing.T) {
	// Setup
	logger := createTestLogger()
	repo := createTestRepository(t, logger)

	cfg := config.DefaultConfig()
	cfg.Security.EnableAuthentication = false

	hmacMiddleware := NewHMACMiddleware(repo, logger, cfg)
	authService := createMockAuthService()
	jwtMiddleware := NewAuthMiddleware(authService, logger, cfg)
	hybrid := NewHybridMiddleware(cfg, hmacMiddleware, jwtMiddleware, logger)

	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Create request with HMAC headers
	req := httptest.NewRequest("POST", "/api/test", nil)
	req.Header.Set("X-Service-ID", "test-service")
	req.Header.Set("X-Timestamp", time.Now().Format(time.RFC3339))
	req.Header.Set("X-Signature", "sha256=test")
	w := httptest.NewRecorder()

	// Execute
	hybrid.Middleware(testHandler).ServeHTTP(w, req)

	// Should bypass all middleware when disabled
	if !handlerCalled {
		t.Error("Expected handler to be called when middleware disabled")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHybridMiddleware_GetAuthenticationMethod(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := createTestLogger()
	hybrid := NewHybridMiddleware(cfg, nil, nil, logger)

	testCases := []struct {
		name     string
		headers  map[string]string
		expected string
	}{
		{
			name: "HMAC request",
			headers: map[string]string{
				"X-Service-ID": "test-service",
				"X-Timestamp":  time.Now().Format(time.RFC3339),
				"X-Signature":  "sha256=test",
			},
			expected: "HMAC",
		},
		{
			name: "JWT request",
			headers: map[string]string{
				"Authorization": "Bearer jwt-token",
			},
			expected: "JWT",
		},
		{
			name:     "No auth headers",
			headers:  map[string]string{},
			expected: "JWT",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/test", nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}

			method := hybrid.GetAuthenticationMethod(req)
			if method != tc.expected {
				t.Errorf("Expected authentication method %s, got %s", tc.expected, method)
			}
		})
	}
}

func TestHybridMiddleware_IsHMACRequest(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := createTestLogger()
	hybrid := NewHybridMiddleware(cfg, nil, nil, logger)

	testCases := []struct {
		name     string
		headers  map[string]string
		expected bool
	}{
		{
			name: "Complete HMAC headers",
			headers: map[string]string{
				"X-Service-ID": "service-123",
				"X-Timestamp":  "2025-06-15T10:30:00Z",
				"X-Signature":  "sha256=abcd1234",
			},
			expected: true,
		},
		{
			name: "Missing signature",
			headers: map[string]string{
				"X-Service-ID": "service-123",
				"X-Timestamp":  "2025-06-15T10:30:00Z",
			},
			expected: false,
		},
		{
			name: "Missing timestamp",
			headers: map[string]string{
				"X-Service-ID": "service-123",
				"X-Signature":  "sha256=abcd1234",
			},
			expected: false,
		},
		{
			name: "Missing service ID",
			headers: map[string]string{
				"X-Timestamp": "2025-06-15T10:30:00Z",
				"X-Signature": "sha256=abcd1234",
			},
			expected: false,
		},
		{
			name:     "No HMAC headers",
			headers:  map[string]string{},
			expected: false,
		},
		{
			name: "Empty header values",
			headers: map[string]string{
				"X-Service-ID": "",
				"X-Timestamp":  "",
				"X-Signature":  "",
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/test", nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}

			result := hybrid.isHMACRequest(req)
			if result != tc.expected {
				t.Errorf("Expected isHMACRequest to return %v, got %v", tc.expected, result)
			}
		})
	}
}
