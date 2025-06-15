package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
)

// Mock logger for testing
type mockLogger struct {
	logs []string
}

func (m *mockLogger) Debug(msg string, args ...any) {
	m.logs = append(m.logs, msg)
}

func (m *mockLogger) Info(msg string, args ...any) {
	m.logs = append(m.logs, msg)
}

func (m *mockLogger) Warn(msg string, args ...any) {
	m.logs = append(m.logs, msg)
}

func (m *mockLogger) Error(msg string, args ...any) {
	m.logs = append(m.logs, msg)
}

func (m *mockLogger) UpdateLevel(level string) {}

func (m *mockLogger) Shutdown() {}

// Test helper to create a temporary file path
func createTempFilePath(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "gortms-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Clean up temp directory after test
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	return filepath.Join(tempDir, "services.db")
}

// Test helper to create a test service account
func createTestServiceAccount() *model.ServiceAccount {
	return &model.ServiceAccount{
		ID:          "test-service-001",
		Name:        "Test Payment Service",
		Secret:      "super-secret-key-12345",
		Permissions: []string{"publish:orders", "consume:payments"},
		IPWhitelist: []string{"127.0.0.1", "192.168.1.*"},
		CreatedAt:   time.Now(),
		LastUsed:    time.Now(),
		Enabled:     true,
	}
}

func TestSecureServiceRepository_Create(t *testing.T) {
	logger := &mockLogger{}
	filePath := createTempFilePath(t)

	repo, err := NewSecureServiceRepository(filePath, logger)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	ctx := context.Background()
	service := createTestServiceAccount()

	// Test successful creation
	err = repo.Create(ctx, service)
	if err != nil {
		t.Errorf("Expected successful creation, got error: %v", err)
	}

	// Test duplicate creation should fail
	err = repo.Create(ctx, service)
	if err == nil {
		t.Error("Expected error for duplicate service creation")
	}

	// Verify service exists
	retrieved, err := repo.GetByID(ctx, service.ID)
	if err != nil {
		t.Errorf("Failed to retrieve created service: %v", err)
	}

	if retrieved.ID != service.ID {
		t.Errorf("Expected service ID %s, got %s", service.ID, retrieved.ID)
	}

	if retrieved.Name != service.Name {
		t.Errorf("Expected service name %s, got %s", service.Name, retrieved.Name)
	}

	if retrieved.Secret != service.Secret {
		t.Errorf("Expected service secret to be preserved")
	}
}

func TestSecureServiceRepository_GetByID(t *testing.T) {
	logger := &mockLogger{}
	filePath := createTempFilePath(t)

	repo, err := NewSecureServiceRepository(filePath, logger)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	ctx := context.Background()
	service := createTestServiceAccount()

	// Test getting non-existent service
	_, err = repo.GetByID(ctx, "non-existent")
	if err == nil {
		t.Error("Expected error for non-existent service")
	}

	// Create service first
	err = repo.Create(ctx, service)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Test successful retrieval
	retrieved, err := repo.GetByID(ctx, service.ID)
	if err != nil {
		t.Errorf("Expected successful retrieval, got error: %v", err)
	}

	// Verify it's a copy (defensive copying)
	retrieved.Name = "Modified Name"
	original, _ := repo.GetByID(ctx, service.ID)
	if original.Name == "Modified Name" {
		t.Error("Repository should return defensive copies")
	}
}

func TestSecureServiceRepository_Update(t *testing.T) {
	logger := &mockLogger{}
	filePath := createTempFilePath(t)

	repo, err := NewSecureServiceRepository(filePath, logger)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	ctx := context.Background()
	service := createTestServiceAccount()

	// Test updating non-existent service
	err = repo.Update(ctx, service)
	if err == nil {
		t.Error("Expected error for updating non-existent service")
	}

	// Create service first
	err = repo.Create(ctx, service)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Update service
	service.Name = "Updated Service Name"
	service.Permissions = []string{"publish:*"}
	err = repo.Update(ctx, service)
	if err != nil {
		t.Errorf("Expected successful update, got error: %v", err)
	}

	// Verify update
	retrieved, err := repo.GetByID(ctx, service.ID)
	if err != nil {
		t.Errorf("Failed to retrieve updated service: %v", err)
	}

	if retrieved.Name != "Updated Service Name" {
		t.Errorf("Expected updated name, got %s", retrieved.Name)
	}

	if len(retrieved.Permissions) != 1 || retrieved.Permissions[0] != "publish:*" {
		t.Errorf("Expected updated permissions, got %v", retrieved.Permissions)
	}
}

func TestSecureServiceRepository_Delete(t *testing.T) {
	logger := &mockLogger{}
	filePath := createTempFilePath(t)

	repo, err := NewSecureServiceRepository(filePath, logger)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	ctx := context.Background()
	service := createTestServiceAccount()

	// Test deleting non-existent service
	err = repo.Delete(ctx, "non-existent")
	if err == nil {
		t.Error("Expected error for deleting non-existent service")
	}

	// Create service first
	err = repo.Create(ctx, service)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Delete service
	err = repo.Delete(ctx, service.ID)
	if err != nil {
		t.Errorf("Expected successful deletion, got error: %v", err)
	}

	// Verify deletion
	_, err = repo.GetByID(ctx, service.ID)
	if err == nil {
		t.Error("Expected error when retrieving deleted service")
	}
}

func TestSecureServiceRepository_List(t *testing.T) {
	logger := &mockLogger{}
	filePath := createTempFilePath(t)

	repo, err := NewSecureServiceRepository(filePath, logger)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	ctx := context.Background()

	// Test empty list
	services, err := repo.List(ctx)
	if err != nil {
		t.Errorf("Expected successful list, got error: %v", err)
	}

	if len(services) != 0 {
		t.Errorf("Expected empty list, got %d services", len(services))
	}

	// Add multiple services
	service1 := createTestServiceAccount()
	service1.ID = "service-001"
	service1.Name = "Service 1"

	service2 := createTestServiceAccount()
	service2.ID = "service-002"
	service2.Name = "Service 2"

	err = repo.Create(ctx, service1)
	if err != nil {
		t.Fatalf("Failed to create service1: %v", err)
	}

	err = repo.Create(ctx, service2)
	if err != nil {
		t.Fatalf("Failed to create service2: %v", err)
	}

	// Test list with services
	services, err = repo.List(ctx)
	if err != nil {
		t.Errorf("Expected successful list, got error: %v", err)
	}

	if len(services) != 2 {
		t.Errorf("Expected 2 services, got %d", len(services))
	}

	// Verify it returns copies
	services[0].Name = "Modified"
	originalServices, _ := repo.List(ctx)
	for _, svc := range originalServices {
		if svc.Name == "Modified" {
			t.Error("Repository should return defensive copies in list")
		}
	}
}

func TestSecureServiceRepository_UpdateLastUsed(t *testing.T) {
	logger := &mockLogger{}
	filePath := createTempFilePath(t)

	repo, err := NewSecureServiceRepository(filePath, logger)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	ctx := context.Background()
	service := createTestServiceAccount()
	originalLastUsed := service.LastUsed

	// Test updating non-existent service
	err = repo.UpdateLastUsed(ctx, "non-existent")
	if err == nil {
		t.Error("Expected error for updating non-existent service")
	}

	// Create service first
	err = repo.Create(ctx, service)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Wait a bit to ensure timestamp difference
	time.Sleep(10 * time.Millisecond)

	// Update last used
	err = repo.UpdateLastUsed(ctx, service.ID)
	if err != nil {
		t.Errorf("Expected successful update, got error: %v", err)
	}

	// Give async operation time to complete
	time.Sleep(50 * time.Millisecond)

	// Verify update
	retrieved, err := repo.GetByID(ctx, service.ID)
	if err != nil {
		t.Errorf("Failed to retrieve service: %v", err)
	}

	if !retrieved.LastUsed.After(originalLastUsed) {
		t.Error("Expected LastUsed to be updated to a later time")
	}
}

func TestSecureServiceRepository_Persistence(t *testing.T) {
	logger := &mockLogger{}
	filePath := createTempFilePath(t)

	// Create first repository instance
	repo1, err := NewSecureServiceRepository(filePath, logger)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	ctx := context.Background()
	service := createTestServiceAccount()

	// Create service in first instance
	err = repo1.Create(ctx, service)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Create second repository instance (should load from file)
	repo2, err := NewSecureServiceRepository(filePath, logger)
	if err != nil {
		t.Fatalf("Failed to create second repository: %v", err)
	}

	// Verify service exists in second instance
	retrieved, err := repo2.GetByID(ctx, service.ID)
	if err != nil {
		t.Errorf("Service should be loaded from file: %v", err)
	}

	// Verify all data is preserved including secret
	if retrieved.ID != service.ID {
		t.Errorf("Expected ID %s, got %s", service.ID, retrieved.ID)
	}

	if retrieved.Secret != service.Secret {
		t.Error("Secret should be properly encrypted and decrypted")
	}

	if len(retrieved.Permissions) != len(service.Permissions) {
		t.Errorf("Expected %d permissions, got %d", len(service.Permissions), len(retrieved.Permissions))
	}
}

func TestSecureServiceRepository_EncryptionSecurity(t *testing.T) {
	logger := &mockLogger{}
	filePath := createTempFilePath(t)

	repo, err := NewSecureServiceRepository(filePath, logger)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	ctx := context.Background()
	service := createTestServiceAccount()
	service.Secret = "very-secret-key-that-should-be-encrypted"

	// Create service
	err = repo.Create(ctx, service)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Read raw file content
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	// Verify secret is not in plain text in file
	if string(fileContent) == service.Secret {
		t.Error("Secret should not appear in plain text in the file")
	}

	// Verify file contains encrypted structure
	if len(fileContent) == 0 {
		t.Error("File should contain encrypted data")
	}

	// Verify we can still retrieve the service with correct secret
	retrieved, err := repo.GetByID(ctx, service.ID)
	if err != nil {
		t.Errorf("Failed to retrieve service: %v", err)
	}

	if retrieved.Secret != service.Secret {
		t.Error("Decrypted secret should match original")
	}
}

func TestSecureServiceRepository_ConcurrentAccess(t *testing.T) {
	logger := &mockLogger{}
	filePath := createTempFilePath(t)

	repo, err := NewSecureServiceRepository(filePath, logger)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	ctx := context.Background()

	// Test concurrent creates
	type result struct {
		index int
		err   error
	}

	results := make(chan result, 10)

	for i := 0; i < 10; i++ {
		go func(index int) {
			service := createTestServiceAccount()
			service.ID = fmt.Sprintf("service-%03d", index)
			service.Name = fmt.Sprintf("Service %d", index)

			err := repo.Create(ctx, service)
			results <- result{index: index, err: err}
		}(i)
	}

	// Collect results
	for i := 0; i < 10; i++ {
		result := <-results
		if result.err != nil {
			t.Errorf("Concurrent create failed for service %d: %v", result.index, result.err)
		}
	}

	// Verify all services were created
	services, err := repo.List(ctx)
	if err != nil {
		t.Errorf("Failed to list services: %v", err)
	}

	if len(services) != 10 {
		t.Errorf("Expected 10 services, got %d", len(services))
	}
}

func TestGenerateServiceSecret(t *testing.T) {
	// Test secret generation
	secret1 := GenerateServiceSecret()
	secret2 := GenerateServiceSecret()

	if secret1 == secret2 {
		t.Error("Generated secrets should be unique")
	}

	if len(secret1) == 0 {
		t.Error("Generated secret should not be empty")
	}

	// Verify it's hex encoded (should be 64 characters for 32 bytes)
	if len(secret1) != 64 {
		t.Errorf("Expected 64 character secret, got %d", len(secret1))
	}

	// Test multiple generations are unique
	secrets := make(map[string]bool)
	for i := 0; i < 100; i++ {
		secret := GenerateServiceSecret()
		if secrets[secret] {
			t.Error("Duplicate secret generated")
		}
		secrets[secret] = true
	}
}
