package rest

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ajkula/GoRTMS/adapter/outbound/storage"
	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

// Test helper to create a temporary file path
func createTempFilePath(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "gortms-hmac-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Clean up temp directory after test
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	return filepath.Join(tempDir, "services.db")
}

// Test helper to create repository with test data
func createTestRepository(t *testing.T, logger outbound.Logger) outbound.ServiceRepository {
	filePath := createTempFilePath(t)
	repo, err := storage.NewSecureServiceRepository(filePath, logger)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}
	return repo
}

type mockLogger2 struct {
	logs []string
}

func (m *mockLogger2) Debug(msg string, args ...any) {
	m.logs = append(m.logs, fmt.Sprintf("DEBUG: %s %v", msg, args))
}

func (m *mockLogger2) Info(msg string, args ...any) {
	m.logs = append(m.logs, fmt.Sprintf("INFO: %s %v", msg, args))
}

func (m *mockLogger2) Warn(msg string, args ...any) {
	m.logs = append(m.logs, fmt.Sprintf("WARN: %s %v", msg, args))
}

func (m *mockLogger2) Error(msg string, args ...any) {
	m.logs = append(m.logs, fmt.Sprintf("ERROR: %s %v", msg, args))
}

func (m *mockLogger2) UpdateLevel(level string) {}

func (m *mockLogger2) Shutdown() {}

// Test helper to generate valid HMAC signature
func generateTestSignature(method, path, body, timestamp, secret string) string {
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s", method, path, body, timestamp)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(canonicalRequest))
	signature := hex.EncodeToString(h.Sum(nil))
	return fmt.Sprintf("sha256=%s", signature)
}

// Test helper to create a test service account
func createTestService() *model.ServiceAccount {
	return &model.ServiceAccount{
		ID:          "test-service-001",
		Name:        "Test Service",
		Secret:      "test-secret-key-123",
		Permissions: []string{"publish:orders", "consume:payments"},
		IPWhitelist: []string{}, // Empty whitelist for basic tests
		CreatedAt:   time.Now(),
		LastUsed:    time.Now(),
		Enabled:     true,
	}
}

// Test helper to create a test service account with IP whitelist
func createTestServiceWithIPWhitelist() *model.ServiceAccount {
	return &model.ServiceAccount{
		ID:          "test-service-001",
		Name:        "Test Service",
		Secret:      "test-secret-key-123",
		Permissions: []string{"publish:orders", "consume:payments"},
		IPWhitelist: []string{"127.0.0.1", "192.168.1.*"},
		CreatedAt:   time.Now(),
		LastUsed:    time.Now(),
		Enabled:     true,
	}
}

// Test helper to create HTTP request with HMAC headers
func createTestRequest(method, path, body string, service *model.ServiceAccount) *http.Request {
	timestamp := time.Now().Format(time.RFC3339)
	signature := generateTestSignature(method, path, body, timestamp, service.Secret)

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("X-Service-ID", service.ID)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", signature)
	req.Header.Set("Content-Type", "application/json")

	return req
}

func TestHMACMiddleware_ValidAuthentication(t *testing.T) {
	// Setup
	logger := &mockLogger2{}
	repo := createTestRepository(t, logger)
	middleware := NewHMACMiddleware(repo, logger)
	middleware.SetEnabled(true)

	service := createTestService()
	repo.Create(context.Background(), service)

	// Test handler
	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true

		// Verify service is in context
		ctxService := middleware.GetServiceFromContext(r.Context())
		if ctxService == nil {
			t.Error("Expected service in context")
		} else {
			if ctxService.ID != service.ID {
				t.Errorf("Expected service ID %s, got %s", service.ID, ctxService.ID)
			}
		}

		w.WriteHeader(http.StatusOK)
	})

	// Create request
	body := `{"message":"test"}`
	req := createTestRequest("POST", "/api/domains/orders/queues/payments/messages", body, service)
	w := httptest.NewRecorder()

	// Debug: print service permissions
	t.Logf("DEBUG: Service permissions: %v", service.Permissions)

	// Execute
	middleware.Middleware(testHandler).ServeHTTP(w, req)

	// Debug: print response
	t.Logf("DEBUG: Response status: %d", w.Code)
	t.Logf("DEBUG: Response body: %s", w.Body.String())

	// Verify
	if !handlerCalled {
		t.Error("Expected handler to be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHMACMiddleware_MissingHeaders(t *testing.T) {
	// Setup
	logger := &mockLogger2{}
	repo := createTestRepository(t, logger)
	middleware := NewHMACMiddleware(repo, logger)
	middleware.SetEnabled(true)

	testCases := []struct {
		name    string
		headers map[string]string
	}{
		{
			name:    "Missing all headers",
			headers: map[string]string{},
		},
		{
			name: "Missing timestamp",
			headers: map[string]string{
				"X-Service-ID": "test-service",
				"X-Signature":  "sha256=test",
			},
		},
		{
			name: "Missing signature",
			headers: map[string]string{
				"X-Service-ID": "test-service",
				"X-Timestamp":  time.Now().Format(time.RFC3339),
			},
		},
		{
			name: "Missing service ID",
			headers: map[string]string{
				"X-Timestamp": time.Now().Format(time.RFC3339),
				"X-Signature": "sha256=test",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handlerCalled := false
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
			})

			req := httptest.NewRequest("POST", "/api/test", nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			w := httptest.NewRecorder()

			middleware.Middleware(testHandler).ServeHTTP(w, req)

			if handlerCalled {
				t.Error("Expected handler NOT to be called")
			}
			if w.Code != http.StatusUnauthorized {
				t.Errorf("Expected status 401, got %d", w.Code)
			}
		})
	}
}

func TestHMACMiddleware_InvalidTimestamp(t *testing.T) {
	// Setup
	logger := &mockLogger2{}
	repo := createTestRepository(t, logger)
	middleware := NewHMACMiddleware(repo, logger)
	middleware.SetEnabled(true)

	testCases := []struct {
		name      string
		timestamp string
	}{
		{
			name:      "Invalid format",
			timestamp: "invalid-timestamp",
		},
		{
			name:      "Too old",
			timestamp: time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
		},
		{
			name:      "Too far in future",
			timestamp: time.Now().Add(10 * time.Minute).Format(time.RFC3339),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handlerCalled := false
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
			})

			req := httptest.NewRequest("POST", "/api/test", nil)
			req.Header.Set("X-Service-ID", "test-service")
			req.Header.Set("X-Timestamp", tc.timestamp)
			req.Header.Set("X-Signature", "sha256=test")
			w := httptest.NewRecorder()

			middleware.Middleware(testHandler).ServeHTTP(w, req)

			if handlerCalled {
				t.Error("Expected handler NOT to be called")
			}
			if w.Code != http.StatusUnauthorized {
				t.Errorf("Expected status 401, got %d", w.Code)
			}
		})
	}
}

func TestHMACMiddleware_ServiceNotFound(t *testing.T) {
	// Setup
	logger := &mockLogger2{}
	repo := createTestRepository(t, logger)
	middleware := NewHMACMiddleware(repo, logger)
	middleware.SetEnabled(true)

	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	})

	req := httptest.NewRequest("POST", "/api/test", nil)
	req.Header.Set("X-Service-ID", "non-existent-service")
	req.Header.Set("X-Timestamp", time.Now().Format(time.RFC3339))
	req.Header.Set("X-Signature", "sha256=test")
	w := httptest.NewRecorder()

	// Execute
	middleware.Middleware(testHandler).ServeHTTP(w, req)

	// Verify
	if handlerCalled {
		t.Error("Expected handler NOT to be called")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestHMACMiddleware_ServiceDisabled(t *testing.T) {
	// Setup
	logger := &mockLogger2{}
	repo := createTestRepository(t, logger)
	middleware := NewHMACMiddleware(repo, logger)
	middleware.SetEnabled(true)

	service := createTestService()
	service.Enabled = false // Disable service
	repo.Create(context.Background(), service)

	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	})

	body := `{"message":"test"}`
	req := createTestRequest("POST", "/api/test", body, service)
	w := httptest.NewRecorder()

	// Execute
	middleware.Middleware(testHandler).ServeHTTP(w, req)

	// Verify
	if handlerCalled {
		t.Error("Expected handler NOT to be called")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestHMACMiddleware_InvalidSignature(t *testing.T) {
	// Setup
	logger := &mockLogger2{}
	repo := createTestRepository(t, logger)
	middleware := NewHMACMiddleware(repo, logger)
	middleware.SetEnabled(true)

	service := createTestService()
	repo.Create(context.Background(), service)

	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	})

	// Create request with invalid signature
	req := httptest.NewRequest("POST", "/api/test", strings.NewReader(`{"test":"data"}`))
	req.Header.Set("X-Service-ID", service.ID)
	req.Header.Set("X-Timestamp", time.Now().Format(time.RFC3339))
	req.Header.Set("X-Signature", "sha256=invalid-signature")
	w := httptest.NewRecorder()

	// Execute
	middleware.Middleware(testHandler).ServeHTTP(w, req)

	// Verify
	if handlerCalled {
		t.Error("Expected handler NOT to be called")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestHMACMiddleware_Permissions(t *testing.T) {
	// Setup
	logger := &mockLogger2{}
	repo := createTestRepository(t, logger)
	middleware := NewHMACMiddleware(repo, logger)
	middleware.SetEnabled(true)

	service := createTestService()
	service.Permissions = []string{"publish:orders"} // Only publish to orders domain
	repo.Create(context.Background(), service)

	testCases := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		shouldCall     bool
		description    string
	}{
		{
			name:           "Valid publish permission",
			method:         "POST",
			path:           "/api/domains/orders/queues/payments/messages",
			expectedStatus: http.StatusOK,
			shouldCall:     true,
			description:    "Service has publish:orders permission for POST to orders domain",
		},
		{
			name:           "Invalid consume permission",
			method:         "GET",
			path:           "/api/domains/orders/queues/payments/messages",
			expectedStatus: http.StatusForbidden,
			shouldCall:     false,
			description:    "Service lacks consume:orders permission for GET from orders domain",
		},
		{
			name:           "Invalid domain permission",
			method:         "POST",
			path:           "/api/domains/inventory/queues/items/messages",
			expectedStatus: http.StatusForbidden,
			shouldCall:     false,
			description:    "Service lacks publish:inventory permission for POST to inventory domain",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing: %s", tc.description)

			handlerCalled := false
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			})

			body := `{"message":"test"}`
			req := createTestRequest(tc.method, tc.path, body, service)
			w := httptest.NewRecorder()

			middleware.Middleware(testHandler).ServeHTTP(w, req)

			if handlerCalled != tc.shouldCall {
				t.Errorf("Expected handler called: %v, got: %v", tc.shouldCall, handlerCalled)
			}
			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, w.Code)
			}
		})
	}
}

func TestHMACMiddleware_WildcardPermissions(t *testing.T) {
	// Setup
	logger := &mockLogger2{}
	repo := createTestRepository(t, logger)
	middleware := NewHMACMiddleware(repo, logger)
	middleware.SetEnabled(true)

	service := createTestService()
	service.Permissions = []string{"publish:*", "manage:orders"} // Wildcard permissions
	repo.Create(context.Background(), service)

	testCases := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{
			name:           "Wildcard publish to any domain",
			method:         "POST",
			path:           "/api/domains/inventory/queues/items/messages",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Specific manage permission",
			method:         "POST",
			path:           "/api/domains/orders/queues/payments/consumer-groups/group1/consumers",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "No permission for consume",
			method:         "GET",
			path:           "/api/domains/orders/queues/payments/messages",
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handlerCalled := false
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			})

			body := `{"message":"test"}`
			req := createTestRequest(tc.method, tc.path, body, service)
			w := httptest.NewRecorder()

			middleware.Middleware(testHandler).ServeHTTP(w, req)

			if tc.expectedStatus == http.StatusOK && !handlerCalled {
				t.Error("Expected handler to be called")
			}
			if tc.expectedStatus != http.StatusOK && handlerCalled {
				t.Error("Expected handler NOT to be called")
			}
			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, w.Code)
			}
		})
	}
}

func TestHMACMiddleware_IPWhitelist(t *testing.T) {
	// Setup
	logger := &mockLogger2{}
	repo := createTestRepository(t, logger)
	middleware := NewHMACMiddleware(repo, logger)
	middleware.SetEnabled(true)

	service := createTestServiceWithIPWhitelist() // Use service with IP whitelist
	repo.Create(context.Background(), service)

	testCases := []struct {
		name           string
		remoteAddr     string
		expectedStatus int
	}{
		{
			name:           "Allowed exact IP",
			remoteAddr:     "127.0.0.1:12345",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Allowed wildcard IP",
			remoteAddr:     "192.168.1.100:54321",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Forbidden IP",
			remoteAddr:     "10.0.0.1:12345",
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handlerCalled := false
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			})

			body := `{"message":"test"}`
			req := createTestRequest("POST", "/api/domains/orders/queues/payments/messages", body, service)
			req.RemoteAddr = tc.remoteAddr
			w := httptest.NewRecorder()

			middleware.Middleware(testHandler).ServeHTTP(w, req)

			if tc.expectedStatus == http.StatusOK && !handlerCalled {
				t.Error("Expected handler to be called")
			}
			if tc.expectedStatus != http.StatusOK && handlerCalled {
				t.Error("Expected handler NOT to be called")
			}
			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, w.Code)
			}
		})
	}
}

func TestHMACMiddleware_Disabled(t *testing.T) {
	// Setup
	logger := &mockLogger2{}
	repo := createTestRepository(t, logger)
	middleware := NewHMACMiddleware(repo, logger)
	middleware.SetEnabled(false) // Middleware disabled

	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Create request without any HMAC headers
	req := httptest.NewRequest("POST", "/api/test", nil)
	w := httptest.NewRecorder()

	// Execute
	middleware.Middleware(testHandler).ServeHTTP(w, req)

	// Verify - should pass through when disabled
	if !handlerCalled {
		t.Error("Expected handler to be called when middleware is disabled")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHMACMiddleware_LastUsedUpdate(t *testing.T) {
	// Setup
	logger := &mockLogger2{}
	repo := createTestRepository(t, logger)
	middleware := NewHMACMiddleware(repo, logger)
	middleware.SetEnabled(true)

	service := createTestService()

	// Ensure some time passes before the test
	time.Sleep(10 * time.Millisecond)
	originalLastUsed := service.LastUsed

	repo.Create(context.Background(), service)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	body := `{"message":"test"}`
	req := createTestRequest("POST", "/api/domains/orders/queues/payments/messages", body, service)
	w := httptest.NewRecorder()

	// Execute
	middleware.Middleware(testHandler).ServeHTTP(w, req)

	// Verify
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Give async operation more time to complete
	time.Sleep(200 * time.Millisecond)

	// Verify lastUsed was updated by checking the service from repository
	retrievedService, err := repo.GetByID(context.Background(), service.ID)
	if err != nil {
		t.Errorf("Failed to retrieve service: %v", err)
	}

	if !retrievedService.LastUsed.After(originalLastUsed) {
		t.Errorf("Expected lastUsed to be updated. Original: %v, Retrieved: %v",
			originalLastUsed.Format(time.RFC3339Nano),
			retrievedService.LastUsed.Format(time.RFC3339Nano))
	}
}
