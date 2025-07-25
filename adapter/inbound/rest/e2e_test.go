package rest

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ajkula/GoRTMS/adapter/outbound/crypto"
	"github.com/ajkula/GoRTMS/adapter/outbound/storage"
	"github.com/ajkula/GoRTMS/config"
	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/inbound"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/mock"
)

// ============================================================================
// TLS E2E TESTS - Ajout après TestE2E_CompleteWorkflow
// ============================================================================

// E2E test that validates TLS certificate generation and HTTPS functionality
func TestE2E_TLS_Complete(t *testing.T) {
	t.Log("=== Starting TLS E2E Test ===")

	// ====================================
	// STEP 1: Test certificate generation
	// ====================================

	server := setupTLSTestServer(t)
	defer server.cleanup()

	t.Log("=== STEP 1: Testing certificate generation ===")
	server.testCertificateGeneration(t)

	// ====================================
	// STEP 2: Test HTTPS server functionality
	// ====================================

	t.Log("=== STEP 2: Testing HTTPS server ===")
	server.testHTTPSServer(t)

	// ====================================
	// STEP 3: Test HMAC RequireTLS guard
	// ====================================

	t.Log("=== STEP 3: Testing HMAC RequireTLS guard ===")
	server.testHMACRequireTLS(t)

	// ====================================
	// STEP 4: Test TLS with real HMAC flow
	// ====================================

	t.Log("=== STEP 4: Testing complete HMAC flow over TLS ===")
	server.testCompleteHMACFlowOverTLS(t)

	t.Log("=== TLS E2E TEST COMPLETED SUCCESSFULLY ===")
}

// ============================================================================
// TLS TEST SERVER SETUP - Ajout après completeTestServer
// ============================================================================

// tlsTestServer extends completeTestServer with TLS capabilities
type tlsTestServer struct {
	*completeTestServer
	cryptoService    outbound.CryptoService
	tlsServer        *httptest.Server
	clientWithTLS    *http.Client
	clientWithoutTLS *http.Client
}

// setupTLSTestServer creates a test server with TLS support
func setupTLSTestServer(t *testing.T) *tlsTestServer {
	base := setupCompleteTestServer(t)

	// Create crypto service
	cryptoService := &MockCryptoService{}

	// Setup mock for certificate generation
	certPEM := []byte(`-----BEGIN CERTIFICATE-----
MIICljCCAX4CCQDTFgP8S8ZH6TANBgkqhkiG9w0BAQsFADATMREwDwYDVQQDDAhs
b2NhbGhvc3QwHhcNMjQwMTAxMDAwMDAwWhcNMjUwMTAxMDAwMDAwWjATMREwDwYD
VQQDDAhsb2NhbGhvc3QwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC8
mock_certificate_data_here_for_testing_purposes_only
-----END CERTIFICATE-----`)

	keyPEM := []byte(`-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC8mock_key_data
here_for_testing_purposes_only
-----END PRIVATE KEY-----`)

	cryptoService.On("GenerateTLSCertificate", "localhost").Return(certPEM, keyPEM, nil)

	// Create TLS config for testing
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		// Use a real certificate generation for testing
		realCrypto := crypto.NewAESCryptoService()
		realCertPEM, realKeyPEM, err := realCrypto.GenerateTLSCertificate("localhost")
		if err != nil {
			t.Fatalf("Failed to generate test certificate: %v", err)
		}
		cert, err = tls.X509KeyPair(realCertPEM, realKeyPEM)
		if err != nil {
			t.Fatalf("Failed to create test key pair: %v", err)
		}
	}

	// Create HTTPS test server
	tlsServer := httptest.NewUnstartedServer(base.router)
	tlsServer.TLS = &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	tlsServer.StartTLS()

	// Create clients - one that accepts self-signed certs, one that doesn't
	clientWithTLS := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Accept self-signed for testing
			},
		},
		Timeout: 5 * time.Second,
	}

	clientWithoutTLS := &http.Client{
		Timeout: 5 * time.Second,
	}

	return &tlsTestServer{
		completeTestServer: base,
		cryptoService:      cryptoService,
		tlsServer:          tlsServer,
		clientWithTLS:      clientWithTLS,
		clientWithoutTLS:   clientWithoutTLS,
	}
}

func (s *tlsTestServer) cleanup() {
	if s.tlsServer != nil {
		s.tlsServer.Close()
	}
	s.completeTestServer.cleanup()
}

// ============================================================================
// TLS SPECIFIC TEST METHODS - Ajout après les méthodes existantes
// ============================================================================

// testCertificateGeneration validates certificate generation
func (s *tlsTestServer) testCertificateGeneration(t *testing.T) {
	certPEM, keyPEM, err := s.cryptoService.GenerateTLSCertificate("localhost")
	if err != nil {
		t.Fatalf("Failed to generate certificate: %v", err)
	}

	if len(certPEM) == 0 {
		t.Error("Certificate PEM is empty")
	}

	if len(keyPEM) == 0 {
		t.Error("Key PEM is empty")
	}

	// Validate certificate format
	if !bytes.Contains(certPEM, []byte("BEGIN CERTIFICATE")) {
		t.Error("Certificate doesn't have proper PEM format")
	}

	if !bytes.Contains(keyPEM, []byte("BEGIN PRIVATE KEY")) {
		t.Error("Key doesn't have proper PEM format")
	}

	t.Logf("✅ Certificate generated: %d bytes, Key: %d bytes", len(certPEM), len(keyPEM))
}

// testHTTPSServer validates HTTPS server functionality
func (s *tlsTestServer) testHTTPSServer(t *testing.T) {
	// Test 1: HTTPS health check
	url := s.tlsServer.URL + "/health"
	resp, err := s.clientWithTLS.Get(url)
	if err != nil {
		t.Fatalf("Failed to make HTTPS request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	// Test 2: Verify TLS connection details
	if resp.TLS == nil {
		t.Error("Response should have TLS information")
	} else {
		t.Logf("✅ TLS Version: %s", tlsVersionToString(resp.TLS.Version))
		t.Logf("✅ Cipher Suite: %s", tls.CipherSuiteName(resp.TLS.CipherSuite))
	}

	// Test 3: Verify client without TLS trust fails
	_, err = s.clientWithoutTLS.Get(url)
	if err == nil {
		t.Error("Expected TLS verification error with untrusted client")
	} else {
		t.Logf("✅ Untrusted client correctly rejected: %v", err)
	}
}

// testHMACRequireTLS validates HMAC RequireTLS guard functionality
func (s *tlsTestServer) testHMACRequireTLS(t *testing.T) {
	// Create service account first
	serviceSecret, serviceID := s.createServiceAccountTLS(t, "tls-test-service", []string{"publish:*"})

	// Enable RequireTLS in config
	s.handler.config.Security.HMAC.RequireTLS = true
	s.handler.config.Security.EnableAuthentication = true

	// Create domain and queue for testing
	domainName := "tls-test-domain"
	queueName := "tls-test-queue"

	// Test 1: HMAC request over HTTPS should work
	t.Log("Testing HMAC over HTTPS (should work)")
	s.createDomainWithHMACOverTLS(t, serviceID, serviceSecret, domainName)
	s.createQueueWithHMACOverTLS(t, serviceID, serviceSecret, domainName, queueName)

	// Test 2: Simulate HMAC request over HTTP (should fail with 404)
	t.Log("Testing HMAC over HTTP with RequireTLS=true (should fail)")
	s.testHMACOverHTTPShouldFail(t, serviceID, serviceSecret, domainName)
}

// testCompleteHMACFlowOverTLS validates complete HMAC workflow over TLS
func (s *tlsTestServer) testCompleteHMACFlowOverTLS(t *testing.T) {
	serviceSecret, serviceID := s.createServiceAccountTLS(t, "tls-flow-service", []string{"publish:*", "consume:*", "manage:*"})

	domainName := "tls-flow-domain"
	queueName := "tls-flow-queue"

	// Complete workflow over TLS
	s.createDomainWithHMACOverTLS(t, serviceID, serviceSecret, domainName)
	s.createQueueWithHMACOverTLS(t, serviceID, serviceSecret, domainName, queueName)

	messageID := s.publishMessageWithHMACOverTLS(t, serviceID, serviceSecret, domainName, queueName, map[string]interface{}{
		"type":   "tls-test-message",
		"secure": true,
	})

	messages := s.consumeMessagesWithHMACOverTLS(t, serviceID, serviceSecret, domainName, queueName)

	if len(messages) == 0 {
		t.Error("Expected to consume message over TLS")
	}

	t.Logf("✅ Complete HMAC flow over TLS successful: published %s, consumed %d messages", messageID, len(messages))
}

// ============================================================================
// TLS-SPECIFIC HELPER METHODS - Ajout après les méthodes existantes
// ============================================================================

// createServiceAccountTLS creates service account using TLS
func (s *tlsTestServer) createServiceAccountTLS(t *testing.T, name string, permissions []string) (secret, serviceID string) {
	createReq := model.ServiceAccountCreateRequest{
		Name:        name,
		Permissions: permissions,
		IPWhitelist: []string{},
	}

	body, _ := json.Marshal(createReq)
	url := s.tlsServer.URL + "/api/services"

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer mock-jwt-token")

	resp, err := s.clientWithTLS.Do(req)
	if err != nil {
		t.Fatalf("Failed to make TLS request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to create service account over TLS. Status: %d, Body: %s", resp.StatusCode, bodyBytes)
	}

	var response struct {
		*model.ServiceAccountView
		Message string `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode TLS response: %v", err)
	}

	return response.Secret, response.ID
}

// createDomainWithHMACOverTLS creates domain via HMAC over TLS
func (s *tlsTestServer) createDomainWithHMACOverTLS(t *testing.T, serviceID, secret, domainName string) {
	domainReq := map[string]interface{}{
		"name": domainName,
		"schema": map[string]interface{}{
			"fields": map[string]string{},
		},
	}

	body, _ := json.Marshal(domainReq)
	path := "/api/domains"
	timestamp := time.Now().Format(time.RFC3339)
	signature := s.generateHMACSignature("POST", path, string(body), timestamp, secret)

	url := s.tlsServer.URL + path
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-ID", serviceID)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", signature)

	resp, err := s.clientWithTLS.Do(req)
	if err != nil {
		t.Fatalf("Failed to create domain over TLS: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to create domain via HMAC over TLS. Status: %d, Body: %s", resp.StatusCode, bodyBytes)
	}
}

// createQueueWithHMACOverTLS creates queue via HMAC over TLS
func (s *tlsTestServer) createQueueWithHMACOverTLS(t *testing.T, serviceID, secret, domainName, queueName string) {
	queueReq := map[string]interface{}{
		"name": queueName,
		"config": map[string]interface{}{
			"isPersistent": true,
			"maxSize":      1000,
			"ttl":          "1h",
		},
	}

	body, _ := json.Marshal(queueReq)
	path := fmt.Sprintf("/api/domains/%s/queues", domainName)
	timestamp := time.Now().Format(time.RFC3339)
	signature := s.generateHMACSignature("POST", path, string(body), timestamp, secret)

	url := s.tlsServer.URL + path
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-ID", serviceID)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", signature)

	resp, err := s.clientWithTLS.Do(req)
	if err != nil {
		t.Fatalf("Failed to create queue over TLS: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to create queue via HMAC over TLS. Status: %d, Body: %s", resp.StatusCode, bodyBytes)
	}
}

// publishMessageWithHMACOverTLS publishes message via HMAC over TLS
func (s *tlsTestServer) publishMessageWithHMACOverTLS(t *testing.T, serviceID, secret, domainName, queueName string, message map[string]interface{}) string {
	body, _ := json.Marshal(message)
	path := fmt.Sprintf("/api/domains/%s/queues/%s/messages", domainName, queueName)
	timestamp := time.Now().Format(time.RFC3339)
	signature := s.generateHMACSignature("POST", path, string(body), timestamp, secret)

	url := s.tlsServer.URL + path
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-ID", serviceID)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", signature)

	resp, err := s.clientWithTLS.Do(req)
	if err != nil {
		t.Fatalf("Failed to publish message over TLS: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to publish message via HMAC over TLS. Status: %d, Body: %s", resp.StatusCode, bodyBytes)
	}

	var response map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&response)
	return response["messageId"].(string)
}

// consumeMessagesWithHMACOverTLS consumes messages via HMAC over TLS
func (s *tlsTestServer) consumeMessagesWithHMACOverTLS(t *testing.T, serviceID, secret, domainName, queueName string) []map[string]interface{} {
	path := fmt.Sprintf("/api/domains/%s/queues/%s/messages", domainName, queueName)
	timestamp := time.Now().Format(time.RFC3339)
	signature := s.generateHMACSignature("GET", path, "", timestamp, secret)

	url := s.tlsServer.URL + path + "?max=10"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("X-Service-ID", serviceID)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", signature)

	resp, err := s.clientWithTLS.Do(req)
	if err != nil {
		t.Fatalf("Failed to consume messages over TLS: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to consume messages via HMAC over TLS. Status: %d, Body: %s", resp.StatusCode, bodyBytes)
	}

	var response struct {
		Messages []map[string]interface{} `json:"messages"`
	}
	json.NewDecoder(resp.Body).Decode(&response)
	return response.Messages
}

// testHMACOverHTTPShouldFail tests HMAC over HTTP should fail when RequireTLS=true
func (s *tlsTestServer) testHMACOverHTTPShouldFail(t *testing.T, serviceID, secret, domainName string) {
	// Create an HTTP test request (simulating what would happen if someone tried HTTP)
	domainReq := map[string]interface{}{
		"name": domainName + "-should-fail",
	}

	body, _ := json.Marshal(domainReq)
	timestamp := time.Now().Format(time.RFC3339)
	signature := s.generateHMACSignature("POST", "/api/domains", string(body), timestamp, secret)

	// Use httptest.NewRequest to simulate HTTP (no TLS)
	req := httptest.NewRequest("POST", "/api/domains", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-ID", serviceID)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", signature)
	// Note: req.TLS will be nil, simulating HTTP

	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	// Should return 404 (security by obscurity)
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for HMAC over HTTP when RequireTLS=true, got %d. Body: %s", w.Code, w.Body.String())
	} else {
		t.Logf("✅ HMAC over HTTP correctly rejected with 404")
	}
}

// tlsVersionToString helper function
func tlsVersionToString(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("Unknown (%d)", version)
	}
}

// E2E test that validates the complete HMAC/JWT/Hybrid routing system
func TestE2E_CompleteWorkflow(t *testing.T) {
	// ====================================
	// SETUP: Create complete test server
	// ====================================

	server := setupCompleteTestServer(t)
	defer server.cleanup()

	// ====================================
	// STEP 1: Create service account via Admin API
	// ====================================

	t.Log("=== STEP 1: Creating service account ===")

	serviceSecret, serviceID := server.createServiceAccount(t, "e2e-test-service", []string{"publish:*", "consume:*", "manage:*"})

	t.Logf("Created service: %s with secret: %s", serviceID, serviceSecret[:8]+"...")

	// ====================================
	// STEP 2: Test Hybrid routes (Domain/Queue creation)
	// ====================================

	t.Log("=== STEP 2: Testing Hybrid routes (Domain/Queue creation) ===")

	// Test domain creation with HMAC auth
	domainName := server.createDomainWithHMAC(t, serviceID, serviceSecret, "e2e-test-domain")
	t.Logf("Created domain via HMAC: %s", domainName)

	// Test queue creation with HMAC auth
	queueName := server.createQueueWithHMAC(t, serviceID, serviceSecret, domainName, "e2e-test-queue")
	t.Logf("Created queue via HMAC: %s", queueName)

	// ====================================
	// STEP 3: Test HMAC-only routes (Core Business)
	// ====================================

	t.Log("=== STEP 3: Testing HMAC-only routes (Messages) ===")

	// Test message publishing
	messageID := server.publishMessageWithHMAC(t, serviceID, serviceSecret, domainName, queueName, map[string]interface{}{
		"type":      "e2e-test",
		"content":   "Hello from E2E test",
		"timestamp": time.Now().Unix(),
	})
	t.Logf("Published message via HMAC: %s", messageID)

	// Test message consumption
	messages := server.consumeMessagesWithHMAC(t, serviceID, serviceSecret, domainName, queueName)
	t.Logf("Consumed %d messages via HMAC", len(messages))

	// Verify message content
	if len(messages) == 0 {
		t.Error("Expected to consume at least 1 message")
	} else {
		t.Logf("First message content: %+v", messages[0])
	}

	// ====================================
	// STEP 4: Test JWT/Public routes (Management)
	// ====================================

	t.Log("=== STEP 4: Testing JWT/Public routes (Management) ===")

	// Test public health check
	server.testHealthCheck(t)

	// Test domain listing (should work with auth disabled)
	domains := server.listDomains(t)
	t.Logf("Listed %d domains via management API", len(domains))

	// Test stats (should work with auth disabled)
	stats := server.getStats(t)
	t.Logf("Retrieved stats: %d domains, %d queues", stats["domains"], stats["queues"])

	// ====================================
	// STEP 5: Test Consumer Group auto-management (HMAC)
	// ====================================

	t.Log("=== STEP 5: Testing Consumer Group auto-management ===")

	// Create consumer group first (via JWT/management)
	groupID := server.createConsumerGroup(t, domainName, queueName, "e2e-consumer-group")
	t.Logf("Created consumer group: %s", groupID)

	// Add consumer via HMAC (service auto-scaling)
	consumerID := server.addConsumerToGroupWithHMAC(t, serviceID, serviceSecret, domainName, queueName, groupID, "e2e-consumer-1")
	t.Logf("Added consumer via HMAC: %s", consumerID)

	// Remove consumer via HMAC
	server.removeSelfFromGroupWithHMAC(t, serviceID, serviceSecret, domainName, queueName, groupID)
	t.Logf("Removed consumer via HMAC: %s", consumerID)

	// ====================================
	// STEP 6: Validate authentication isolation
	// ====================================

	t.Log("=== STEP 6: Testing authentication isolation ===")

	// Test HMAC route without auth headers (should fail)
	server.testHMACRouteWithoutAuth(t, domainName, queueName)

	// Test HMAC route with invalid signature (should fail)
	server.testHMACRouteWithInvalidSignature(t, serviceID, domainName, queueName)

	t.Log("=== E2E TEST COMPLETED SUCCESSFULLY ===")
}

// completeTestServer represents a full test server setup
type completeTestServer struct {
	handler     *Handler
	router      *mux.Router
	logger      outbound.Logger
	serviceRepo outbound.ServiceRepository
	tempDir     string
}

// setupCompleteTestServer creates a complete test server with all services
func setupCompleteTestServer(t *testing.T) *completeTestServer {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "gortms-e2e-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create logger
	logger := &mockLogger{}

	// Create repositories
	serviceRepo, err := storage.NewSecureServiceRepository(filepath.Join(tempDir, "services.db"), logger)
	if err != nil {
		t.Fatalf("Failed to create service repository: %v", err)
	}

	// Create config
	cfg := config.DefaultConfig()
	cfg.Security.EnableAuthentication = true

	// Create mock services (minimal implementations for testing)
	// authService := createMockAuthService()
	authService := &mockAuthService{users: make(map[string]*model.User)}
	messageService := &mockMessageService{messages: make(map[string][]*model.Message)}
	domainService := &mockDomainService{domains: make(map[string]*model.Domain)}
	queueService := &mockQueueService{queues: make(map[string]map[string]*model.Queue)}
	routingService := &mockRoutingService{}
	statsService := &mockStatsService{}
	consumerGroupService := &mockConsumerGroupService{groups: make(map[string]*model.ConsumerGroup)}
	consumerGroupRepo := &mockConsumerGroupRepo{}

	// Create handler
	handler := NewHandler(
		logger,
		cfg,
		embed.FS{},
		authService,
		messageService,
		domainService,
		queueService,
		routingService,
		statsService,
		nil, // resourceMonitor
		consumerGroupService,
		consumerGroupRepo,
		serviceRepo,
		nil,
	)

	// Setup routes
	router := mux.NewRouter()
	handler.SetupRoutes(router)

	return &completeTestServer{
		handler:     handler,
		router:      router,
		logger:      logger,
		serviceRepo: serviceRepo,
		tempDir:     tempDir,
	}
}

func (s *completeTestServer) cleanup() {
	os.RemoveAll(s.tempDir)
}

// creates a service account and returns the secret and ID
func (s *completeTestServer) createServiceAccount(t *testing.T, name string, permissions []string) (secret, serviceID string) {
	createReq := model.ServiceAccountCreateRequest{
		Name:        name,
		Permissions: permissions,
		IPWhitelist: []string{}, // No IP restrictions for testing
	}

	body, _ := json.Marshal(createReq)
	req := httptest.NewRequest("POST", "/api/services", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer mock-jwt-token")
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Failed to create service account. Status: %d, Body: %s", w.Code, w.Body.String())
	}

	var response struct {
		*model.ServiceAccountView
		Message string `json:"message"`
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode create service response: %v", err)
	}

	return response.Secret, response.ID
}

// creates HMAC signature for request
func (s *completeTestServer) generateHMACSignature(method, path, body, timestamp, secret string) string {
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s", method, path, body, timestamp)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(canonicalRequest))
	signature := hex.EncodeToString(h.Sum(nil))
	return fmt.Sprintf("sha256=%s", signature)
}

// creates a domain using HMAC authentication
func (s *completeTestServer) createDomainWithHMAC(t *testing.T, serviceID, secret, domainName string) string {
	domainReq := map[string]interface{}{
		"name": domainName,
		"schema": map[string]interface{}{
			"fields": map[string]string{},
		},
	}

	body, _ := json.Marshal(domainReq)
	timestamp := time.Now().Format(time.RFC3339)
	signature := s.generateHMACSignature("POST", "/api/domains", string(body), timestamp, secret)

	req := httptest.NewRequest("POST", "/api/domains", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-ID", serviceID)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", signature)
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Failed to create domain via HMAC. Status: %d, Body: %s", w.Code, w.Body.String())
	}

	return domainName
}

// creates a queue using HMAC authentication
func (s *completeTestServer) createQueueWithHMAC(t *testing.T, serviceID, secret, domainName, queueName string) string {
	queueReq := map[string]interface{}{
		"name": queueName,
		"config": map[string]interface{}{
			"isPersistent": true,
			"maxSize":      1000,
			"ttl":          "1h",
		},
	}

	body, _ := json.Marshal(queueReq)
	path := fmt.Sprintf("/api/domains/%s/queues", domainName)
	timestamp := time.Now().Format(time.RFC3339)
	signature := s.generateHMACSignature("POST", path, string(body), timestamp, secret)

	req := httptest.NewRequest("POST", path, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-ID", serviceID)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", signature)
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Failed to create queue via HMAC. Status: %d, Body: %s", w.Code, w.Body.String())
	}

	return queueName
}

// publishes a message using HMAC authentication
func (s *completeTestServer) publishMessageWithHMAC(t *testing.T, serviceID, secret, domainName, queueName string, message map[string]interface{}) string {
	body, _ := json.Marshal(message)
	path := fmt.Sprintf("/api/domains/%s/queues/%s/messages", domainName, queueName)
	timestamp := time.Now().Format(time.RFC3339)
	signature := s.generateHMACSignature("POST", path, string(body), timestamp, secret)

	req := httptest.NewRequest("POST", path, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-ID", serviceID)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", signature)
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to publish message via HMAC. Status: %d, Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)
	return response["messageId"].(string)
}

// consumes messages using HMAC authentication
func (s *completeTestServer) consumeMessagesWithHMAC(t *testing.T, serviceID, secret, domainName, queueName string) []map[string]interface{} {
	path := fmt.Sprintf("/api/domains/%s/queues/%s/messages?max=10", domainName, queueName)
	timestamp := time.Now().Format(time.RFC3339)
	signature := s.generateHMACSignature("GET", fmt.Sprintf("/api/domains/%s/queues/%s/messages", domainName, queueName), "", timestamp, secret)

	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("X-Service-ID", serviceID)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", signature)
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to consume messages via HMAC. Status: %d, Body: %s", w.Code, w.Body.String())
	}

	var response struct {
		Messages []map[string]interface{} `json:"messages"`
	}
	json.NewDecoder(w.Body).Decode(&response)
	return response.Messages
}

// tests public health endpoint
func (s *completeTestServer) testHealthCheck(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Health check failed. Status: %d", w.Code)
	}
}

// tests domain listing via management API
func (s *completeTestServer) listDomains(t *testing.T) []map[string]interface{} {
	req := httptest.NewRequest("GET", "/api/domains", nil)
	req.Header.Set("Authorization", "Bearer mock-jwt-token")
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Failed to list domains. Status: %d, Body: %s", w.Code, w.Body.String())
		return nil
	}

	var response struct {
		Domains []map[string]interface{} `json:"domains"`
	}
	json.NewDecoder(w.Body).Decode(&response)
	return response.Domains
}

// tests stats endpoint
func (s *completeTestServer) getStats(t *testing.T) map[string]interface{} {
	req := httptest.NewRequest("GET", "/api/stats", nil)
	req.Header.Set("Authorization", "Bearer mock-jwt-token")
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Failed to get stats. Status: %d, Body: %s", w.Code, w.Body.String())
		return nil
	}

	var stats map[string]interface{}
	json.NewDecoder(w.Body).Decode(&stats)
	return stats
}

// creates a consumer group via management API
func (s *completeTestServer) createConsumerGroup(t *testing.T, domainName, queueName, groupID string) string {
	groupReq := map[string]interface{}{
		"groupId": groupID,
		"ttl":     "1h",
	}

	body, _ := json.Marshal(groupReq)
	path := fmt.Sprintf("/api/domains/%s/queues/%s/consumer-groups", domainName, queueName)
	req := httptest.NewRequest("POST", path, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer mock-jwt-token")
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Failed to create consumer group. Status: %d, Body: %s", w.Code, w.Body.String())
	}

	return groupID
}

// adds consumer via HMAC (service auto-scaling)
func (s *completeTestServer) addConsumerToGroupWithHMAC(t *testing.T, serviceID, secret, domainName, queueName, groupID, consumerID string) string {
	consumerReq := map[string]interface{}{
		"consumerID": consumerID,
	}

	body, _ := json.Marshal(consumerReq)
	path := fmt.Sprintf("/api/domains/%s/queues/%s/consumer-groups/%s/consumers", domainName, queueName, groupID)
	timestamp := time.Now().Format(time.RFC3339)
	signature := s.generateHMACSignature("POST", path, string(body), timestamp, secret)

	req := httptest.NewRequest("POST", path, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-ID", serviceID)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", signature)
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to add consumer via HMAC. Status: %d, Body: %s", w.Code, w.Body.String())
	}

	return consumerID
}

// removes self consumer via HMAC
func (s *completeTestServer) removeSelfFromGroupWithHMAC(t *testing.T, serviceID, secret, domainName, queueName, groupID string) {
	path := fmt.Sprintf("/api/domains/%s/queues/%s/consumer-groups/%s/consumers/self", domainName, queueName, groupID)
	timestamp := time.Now().Format(time.RFC3339)
	signature := s.generateHMACSignature("DELETE", path, "", timestamp, secret)

	req := httptest.NewRequest("DELETE", path, nil)
	req.Header.Set("X-Service-ID", serviceID)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", signature)
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Failed to remove consumer via HMAC. Status: %d, Body: %s", w.Code, w.Body.String())
	}
}

// tests that HMAC routes reject requests without auth
func (s *completeTestServer) testHMACRouteWithoutAuth(t *testing.T, domainName, queueName string) {
	path := fmt.Sprintf("/api/domains/%s/queues/%s/messages", domainName, queueName)
	req := httptest.NewRequest("POST", path, strings.NewReader(`{"test": true}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for HMAC route without auth, got %d", w.Code)
	}
}

// tests that HMAC routes reject invalid signatures
func (s *completeTestServer) testHMACRouteWithInvalidSignature(t *testing.T, serviceID, domainName, queueName string) {
	path := fmt.Sprintf("/api/domains/%s/queues/%s/messages", domainName, queueName)
	req := httptest.NewRequest("POST", path, strings.NewReader(`{"test": true}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-ID", serviceID)
	req.Header.Set("X-Timestamp", time.Now().Format(time.RFC3339))
	req.Header.Set("X-Signature", "sha256=invalid-signature")
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for invalid HMAC signature, got %d", w.Code)
	}
}

// ============================================================================
// MOCK IMPLEMENTATIONS

// ============================================================================
type MockCryptoService struct {
	mock.Mock
}

func (m *MockCryptoService) GenerateTLSCertificate(hostname string) (certPEM, keyPEM []byte, err error) {
	args := m.Called(hostname)
	return args.Get(0).([]byte), args.Get(1).([]byte), args.Error(2)
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

// Mock AuthService for testing
type mockAuthService struct {
	users map[string]*model.User
	mu    sync.RWMutex
}

func (m *mockAuthService) CreateUserWithHash(username, passwordHash string, salt [16]byte, role model.UserRole) (*model.User, error) {
	return nil, nil
}

// ValidatePassword implements inbound.AuthService.
func (m *mockAuthService) UpdatePassword(user *model.User, old, new string) error {
	return nil
}

// UpdateUser implements inbound.AuthService.
func (m *mockAuthService) UpdateUser(userID string, updates inbound.UpdateUserRequest, isAdmin bool) (*model.User, error) {
	return &model.User{}, nil
}

func (s *mockAuthService) GenerateToken(user *model.User, issuedAt time.Time) (string, error) {
	return "testuser", nil
}

type mockLogger struct {
	t *testing.T
}

func (m *mockLogger) Error(msg string, args ...any) {
	if m.t != nil {
		m.t.Logf("ERROR: %s %v", msg, args)
	}
}
func (m *mockLogger) Warn(msg string, args ...any) {
	if m.t != nil {
		m.t.Logf("WARN: %s %v", msg, args)
	}
}
func (m *mockLogger) Info(msg string, args ...any) {
	if m.t != nil {
		m.t.Logf("INFO: %s %v", msg, args) // ← Maintenant ça affiche !
	}
}
func (m *mockLogger) Debug(msg string, args ...any) {
	if m.t != nil {
		m.t.Logf("DEBUG: %s %v", msg, args)
	}
}
func (m *mockLogger) UpdateLevel(logLvl string) {}
func (m *mockLogger) Shutdown()                 {}

func (m *mockAuthService) Login(username, password string) (*model.User, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, exists := m.users[username]
	if !exists {
		return nil, "", fmt.Errorf("user not found")
	}

	// Simple password check for testing
	if password != "test-password" {
		return nil, "", fmt.Errorf("invalid password")
	}

	return user, "mock-jwt-token", nil
}

func (m *mockAuthService) ValidateToken(token string) (*model.User, error) {
	if token == "mock-jwt-token" {
		return &model.User{Username: "test-user", Role: model.RoleAdmin}, nil
	}
	return nil, fmt.Errorf("invalid token")
}

func (m *mockAuthService) CreateUser(username, password string, role model.UserRole) (*model.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	user := &model.User{
		Username: username,
		Role:     role,
	}
	m.users[username] = user
	return user, nil
}

func (m *mockAuthService) GetUser(username string) (*model.User, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, exists := m.users[username]
	return user, exists
}

func (m *mockAuthService) ListUsers() ([]*model.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	users := make([]*model.User, 0, len(m.users))
	for _, user := range m.users {
		users = append(users, user)
	}
	return users, nil
}

func (m *mockAuthService) BootstrapAdmin() (*model.User, string, error) {
	admin := &model.User{
		Username: "admin",
		Role:     model.RoleAdmin,
	}
	m.mu.Lock()
	m.users["admin"] = admin
	m.mu.Unlock()

	return admin, "bootstrap-password", nil
}

// mockMessageService implements inbound.MessageService
type mockMessageService struct {
	messages map[string][]*model.Message // key: domainName/queueName
	mu       sync.RWMutex
}

func (m *mockMessageService) PublishMessage(domainName, queueName string, message *model.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s/%s", domainName, queueName)
	m.messages[key] = append(m.messages[key], message)
	return nil
}

func (m *mockMessageService) SubscribeToQueue(domainName, queueName string, handler model.MessageHandler) (string, error) {
	return "mock-subscription-id", nil
}

func (m *mockMessageService) UnsubscribeFromQueue(domainName, queueName string, subscriptionID string) error {
	return nil
}

func (m *mockMessageService) ConsumeMessageWithGroup(ctx context.Context, domainName, queueName, groupID string, options *inbound.ConsumeOptions) (*model.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s/%s", domainName, queueName)
	messages := m.messages[key]

	if len(messages) == 0 {
		return nil, nil
	}

	// Return first message and remove it
	message := messages[0]
	m.messages[key] = messages[1:]
	return message, nil
}

func (m *mockMessageService) GetMessagesAfterIndex(ctx context.Context, domainName, queueName string, startIndex int64, limit int) ([]*model.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := fmt.Sprintf("%s/%s", domainName, queueName)
	messages := m.messages[key]

	if startIndex >= int64(len(messages)) {
		return []*model.Message{}, nil
	}

	end := startIndex + int64(limit)
	if end > int64(len(messages)) {
		end = int64(len(messages))
	}

	return messages[startIndex:end], nil
}

// mockDomainService implements inbound.DomainService
type mockDomainService struct {
	domains map[string]*model.Domain
	mu      sync.RWMutex
}

func (m *mockDomainService) CreateDomain(ctx context.Context, config *model.DomainConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	domain := &model.Domain{
		Name:   config.Name,
		Schema: config.Schema,
	}
	m.domains[config.Name] = domain
	return nil
}

func (m *mockDomainService) GetDomain(ctx context.Context, name string) (*model.Domain, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	domain, exists := m.domains[name]
	if !exists {
		return nil, fmt.Errorf("domain not found")
	}
	return domain, nil
}

func (m *mockDomainService) DeleteDomain(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.domains, name)
	return nil
}

func (m *mockDomainService) ListDomains(ctx context.Context) ([]*model.Domain, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	domains := make([]*model.Domain, 0, len(m.domains))
	for _, domain := range m.domains {
		domains = append(domains, domain)
	}
	return domains, nil
}

// mockQueueService implements inbound.QueueService
type mockQueueService struct {
	queues map[string]map[string]*model.Queue // domain -> queue -> Queue
	mu     sync.RWMutex
}

func (m *mockQueueService) CreateQueue(ctx context.Context, domainName, queueName string, config *model.QueueConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.queues[domainName] == nil {
		m.queues[domainName] = make(map[string]*model.Queue)
	}

	queue := &model.Queue{
		Name:       queueName,
		DomainName: domainName,
		Config:     *config,
	}
	m.queues[domainName][queueName] = queue
	return nil
}

func (m *mockQueueService) GetQueue(ctx context.Context, domainName, queueName string) (*model.Queue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	domain, exists := m.queues[domainName]
	if !exists {
		return nil, fmt.Errorf("domain not found")
	}

	queue, exists := domain[queueName]
	if !exists {
		return nil, fmt.Errorf("queue not found")
	}
	return queue, nil
}

func (m *mockQueueService) DeleteQueue(ctx context.Context, domainName, queueName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if domain, exists := m.queues[domainName]; exists {
		delete(domain, queueName)
	}
	return nil
}

func (m *mockQueueService) ListQueues(ctx context.Context, domainName string) ([]*model.Queue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	domain, exists := m.queues[domainName]
	if !exists {
		return []*model.Queue{}, nil
	}

	queues := make([]*model.Queue, 0, len(domain))
	for _, queue := range domain {
		queues = append(queues, queue)
	}
	return queues, nil
}

func (m *mockQueueService) GetChannelQueue(ctx context.Context, domainName, queueName string) (model.QueueHandler, error) {
	return &mockQueueHandler{}, nil
}

func (m *mockQueueService) StopDomainQueues(ctx context.Context, domainName string) error {
	return nil
}

func (m *mockQueueService) Cleanup() {}

// mockQueueHandler implements model.QueueHandler
type mockQueueHandler struct{}

func (m *mockQueueHandler) Stop()                                           {}
func (m *mockQueueHandler) Start(ctx context.Context)                       {}
func (m *mockQueueHandler) RemoveConsumerGroup(groupID string)              {}
func (m *mockQueueHandler) RequestMessages(groupID string, count int) error { return nil }
func (m *mockQueueHandler) GetQueue() *model.Queue                          { return &model.Queue{} }
func (m *mockQueueHandler) Dequeue(ctx context.Context) (*model.Message, error) {
	return &model.Message{}, nil
}
func (m *mockQueueHandler) Enqueue(context.Context, *model.Message) error { return nil }
func (m *mockQueueHandler) ConsumeMessage(groupID string, timeout time.Duration) (*model.Message, error) {
	return &model.Message{}, nil
}
func (m *mockQueueHandler) AddConsumerGroup(groupID string, lastIndex int64) error { return nil }
func (m *mockQueueHandler) PublishMessage(message *model.Message) error            { return nil }
func (m *mockQueueHandler) Subscribe(handler model.MessageHandler) (string, error) {
	return "mock-sub", nil
}
func (m *mockQueueHandler) Unsubscribe(subscriptionID string) error { return nil }
func (m *mockQueueHandler) Shutdown()                               {}

// mockRoutingService implements inbound.RoutingService
type mockRoutingService struct{}

func (m *mockRoutingService) AddRoutingRule(ctx context.Context, domainName string, rule *model.RoutingRule) error {
	return nil
}

func (m *mockRoutingService) RemoveRoutingRule(ctx context.Context, domainName string, sourceQueue, destQueue string) error {
	return nil
}

func (m *mockRoutingService) ListRoutingRules(ctx context.Context, domainName string) ([]*model.RoutingRule, error) {
	return []*model.RoutingRule{}, nil
}

// mockStatsService implements inbound.StatsService
type mockStatsService struct{}

func (m *mockStatsService) GetStats(ctx context.Context) (any, error) {
	return map[string]interface{}{
		"domains": 1,
		"queues":  1,
		"messages": map[string]interface{}{
			"published": 1,
			"consumed":  0,
		},
	}, nil
}

func (m *mockStatsService) TrackMessagePublished(domainName, queueName string) {}
func (m *mockStatsService) TrackMessageConsumed(domainName, queueName string)  {}
func (m *mockStatsService) GetStatsWithAggregation(ctx context.Context, period, granularity string) (any, error) {
	return m.GetStats(ctx)
}
func (m *mockStatsService) RecordDomainCreated(name string)                      {}
func (m *mockStatsService) RecordDomainDeleted(name string)                      {}
func (m *mockStatsService) RecordQueueCreated(domain, queue string)              {}
func (m *mockStatsService) RecordQueueDeleted(domain, queue string)              {}
func (m *mockStatsService) RecordRoutingRuleCreated(domain, source, dest string) {}
func (m *mockStatsService) RecordDomainActive(name string, queueCount int)       {}

// mockConsumerGroupService implements inbound.ConsumerGroupService
type mockConsumerGroupService struct {
	groups map[string]*model.ConsumerGroup
	mu     sync.RWMutex
}

func (m *mockConsumerGroupService) ListConsumerGroups(ctx context.Context, domainName, queueName string) ([]*model.ConsumerGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := make([]*model.ConsumerGroup, 0)
	key := fmt.Sprintf("%s/%s/", domainName, queueName)
	for id, group := range m.groups {
		if strings.HasPrefix(id, key) {
			groups = append(groups, group)
		}
	}
	return groups, nil
}

func (m *mockConsumerGroupService) ListAllGroups(ctx context.Context) ([]*model.ConsumerGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := make([]*model.ConsumerGroup, 0, len(m.groups))
	for _, group := range m.groups {
		groups = append(groups, group)
	}
	return groups, nil
}

func (m *mockConsumerGroupService) GetGroupDetails(ctx context.Context, domainName, queueName, groupID string) (*model.ConsumerGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := fmt.Sprintf("%s/%s/%s", domainName, queueName, groupID)
	group, exists := m.groups[key]
	if !exists {
		return nil, fmt.Errorf("group not found")
	}
	return group, nil
}

func (m *mockConsumerGroupService) CreateConsumerGroup(ctx context.Context, domainName, queueName, groupID string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s/%s/%s", domainName, queueName, groupID)
	m.groups[key] = &model.ConsumerGroup{
		GroupID:      groupID,
		DomainName:   domainName,
		QueueName:    queueName,
		TTL:          ttl,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		ConsumerIDs:  []string{},
	}
	return nil
}

func (m *mockConsumerGroupService) DeleteConsumerGroup(ctx context.Context, domainName, queueName, groupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s/%s/%s", domainName, queueName, groupID)
	delete(m.groups, key)
	return nil
}

func (m *mockConsumerGroupService) UpdateConsumerGroupTTL(ctx context.Context, domainName, queueName, groupID string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s/%s/%s", domainName, queueName, groupID)
	if group, exists := m.groups[key]; exists {
		group.TTL = ttl
	}
	return nil
}

func (m *mockConsumerGroupService) GetPendingMessages(ctx context.Context, domainName, queueName, groupID string) ([]*model.Message, error) {
	return []*model.Message{}, nil
}

// mockConsumerGroupRepo implements outbound.ConsumerGroupRepository
type mockConsumerGroupRepo struct {
	positions map[string]int64         // key: domain/queue/group -> position
	consumers map[string][]string      // key: domain/queue/group -> []consumerID
	ttls      map[string]time.Duration // key: domain/queue/group -> ttl
	mu        sync.RWMutex
}

func (m *mockConsumerGroupRepo) StorePosition(ctx context.Context, domainName, queueName, groupID string, index int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.positions == nil {
		m.positions = make(map[string]int64)
	}

	key := fmt.Sprintf("%s/%s/%s", domainName, queueName, groupID)
	m.positions[key] = index
	return nil
}

func (m *mockConsumerGroupRepo) GetPosition(ctx context.Context, domainName, queueName, groupID string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := fmt.Sprintf("%s/%s/%s", domainName, queueName, groupID)
	position, exists := m.positions[key]
	if !exists {
		return 0, nil
	}
	return position, nil
}

func (m *mockConsumerGroupRepo) RegisterConsumer(ctx context.Context, domainName, queueName, groupID, consumerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.consumers == nil {
		m.consumers = make(map[string][]string)
	}

	key := fmt.Sprintf("%s/%s/%s", domainName, queueName, groupID)
	m.consumers[key] = append(m.consumers[key], consumerID)
	return nil
}

func (m *mockConsumerGroupRepo) RemoveConsumer(ctx context.Context, domainName, queueName, groupID, consumerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s/%s/%s", domainName, queueName, groupID)
	consumers := m.consumers[key]

	for i, id := range consumers {
		if id == consumerID {
			m.consumers[key] = append(consumers[:i], consumers[i+1:]...)
			break
		}
	}
	return nil
}

func (m *mockConsumerGroupRepo) ListGroups(ctx context.Context, domainName, queueName string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	prefix := fmt.Sprintf("%s/%s/", domainName, queueName)
	groups := make([]string, 0)

	for key := range m.positions {
		if strings.HasPrefix(key, prefix) {
			groupID := strings.TrimPrefix(key, prefix)
			groups = append(groups, groupID)
		}
	}
	return groups, nil
}

func (m *mockConsumerGroupRepo) DeleteGroup(ctx context.Context, domainName, queueName, groupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s/%s/%s", domainName, queueName, groupID)
	delete(m.positions, key)
	delete(m.consumers, key)
	delete(m.ttls, key)
	return nil
}

func (m *mockConsumerGroupRepo) CleanupStaleGroups(ctx context.Context, olderThan time.Duration) error {
	return nil
}

func (m *mockConsumerGroupRepo) SetGroupTTL(ctx context.Context, domainName, queueName, groupID string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ttls == nil {
		m.ttls = make(map[string]time.Duration)
	}

	key := fmt.Sprintf("%s/%s/%s", domainName, queueName, groupID)
	m.ttls[key] = ttl
	return nil
}

func (m *mockConsumerGroupRepo) UpdateLastActivity(ctx context.Context, domainName, queueName, groupID string) error {
	return nil
}
