package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
)

// Mock implementations for testing
type mockCryptoService struct{}

func (m *mockCryptoService) Encrypt(data []byte, key [32]byte) ([]byte, []byte, error) {
	// Simple XOR encryption for testing
	encrypted := make([]byte, len(data))
	for i, b := range data {
		encrypted[i] = b ^ byte(i%256)
	}
	return encrypted, []byte("test-nonce"), nil
}

func (m *mockCryptoService) Decrypt(encrypted []byte, nonce []byte, key [32]byte) ([]byte, error) {
	// Reverse XOR encryption
	decrypted := make([]byte, len(encrypted))
	for i, b := range encrypted {
		decrypted[i] = b ^ byte(i%256)
	}
	return decrypted, nil
}

func (m *mockCryptoService) DeriveKey(machineID string) [32]byte {
	return [32]byte{1, 2, 3, 4} // Simple test key
}

func (m *mockCryptoService) GenerateSalt() [32]byte {
	return [32]byte{5, 6, 7, 8} // Simple test salt
}

func (m *mockCryptoService) HashPassword(password string, salt [16]byte) string {
	return "hashed_" + password
}

func (m *mockCryptoService) VerifyPassword(password, hash string, salt [16]byte) bool {
	return hash == "hashed_"+password
}

type mockMachineIDService struct{}

func (m *mockMachineIDService) GetMachineID() (string, error) {
	return "test-machine-id", nil
}

func createTestRepository(t *testing.T) (*secureAccountRequestRepository, string) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "account_request_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	filePath := filepath.Join(tempDir, "test_account_requests.db")

	repo, err := NewSecureAccountRequestRepository(
		filePath,
		&mockCryptoService{},
		&mockMachineIDService{},
		&mockLogger{},
	)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	return repo.(*secureAccountRequestRepository), tempDir
}

func TestSecureAccountRequestRepository_StoreAndLoad(t *testing.T) {
	repo, tempDir := createTestRepository(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()

	// Create test request
	request := &model.AccountRequest{
		ID:            "test-123",
		Username:      "testuser",
		RequestedRole: model.RoleUser,
		Status:        model.AccountRequestPending,
		CreatedAt:     time.Now(),
		PasswordHash:  "hashed_password",
		Salt:          [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
	}

	// Test Store
	err := repo.Store(ctx, request)
	if err != nil {
		t.Fatalf("Failed to store request: %v", err)
	}

	// Verify file exists
	if !repo.Exists() {
		t.Error("Expected file to exist after Store")
	}

	// Test GetByID
	retrievedRequest, err := repo.GetByID(ctx, "test-123")
	if err != nil {
		t.Fatalf("Failed to get request by ID: %v", err)
	}

	if retrievedRequest.ID != request.ID {
		t.Errorf("Expected ID %s, got %s", request.ID, retrievedRequest.ID)
	}

	if retrievedRequest.Username != request.Username {
		t.Errorf("Expected username %s, got %s", request.Username, retrievedRequest.Username)
	}

	if retrievedRequest.Status != request.Status {
		t.Errorf("Expected status %s, got %s", request.Status, retrievedRequest.Status)
	}
}

func TestSecureAccountRequestRepository_GetByUsername(t *testing.T) {
	repo, tempDir := createTestRepository(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()

	// Create test request
	request := &model.AccountRequest{
		ID:            "test-456",
		Username:      "searchuser",
		RequestedRole: model.RoleAdmin,
		Status:        model.AccountRequestPending,
		CreatedAt:     time.Now(),
		PasswordHash:  "hashed_password",
		Salt:          [16]byte{},
	}

	// Store request
	err := repo.Store(ctx, request)
	if err != nil {
		t.Fatalf("Failed to store request: %v", err)
	}

	// Test GetByUsername
	retrievedRequest, err := repo.GetByUsername(ctx, "searchuser")
	if err != nil {
		t.Fatalf("Failed to get request by username: %v", err)
	}

	if retrievedRequest.ID != request.ID {
		t.Errorf("Expected ID %s, got %s", request.ID, retrievedRequest.ID)
	}

	// Test non-existent username
	_, err = repo.GetByUsername(ctx, "nonexistent")
	if err != model.ErrAccountRequestNotFound {
		t.Errorf("Expected %v, got %v", model.ErrAccountRequestNotFound, err)
	}
}

func TestSecureAccountRequestRepository_List(t *testing.T) {
	repo, tempDir := createTestRepository(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()

	// Create test requests with different statuses
	requests := []*model.AccountRequest{
		{
			ID:            "pending-1",
			Username:      "pending1",
			RequestedRole: model.RoleUser,
			Status:        model.AccountRequestPending,
			CreatedAt:     time.Now(),
		},
		{
			ID:            "pending-2",
			Username:      "pending2",
			RequestedRole: model.RoleUser,
			Status:        model.AccountRequestPending,
			CreatedAt:     time.Now(),
		},
		{
			ID:            "approved-1",
			Username:      "approved1",
			RequestedRole: model.RoleUser,
			Status:        model.AccountRequestApproved,
			CreatedAt:     time.Now(),
		},
	}

	// Store all requests
	for _, req := range requests {
		if err := repo.Store(ctx, req); err != nil {
			t.Fatalf("Failed to store request %s: %v", req.ID, err)
		}
	}

	// Test List all
	allRequests, err := repo.List(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to list all requests: %v", err)
	}

	if len(allRequests) != 3 {
		t.Errorf("Expected 3 requests, got %d", len(allRequests))
	}

	// Test List with status filter
	pendingStatus := model.AccountRequestPending
	pendingRequests, err := repo.List(ctx, &pendingStatus)
	if err != nil {
		t.Fatalf("Failed to list pending requests: %v", err)
	}

	if len(pendingRequests) != 2 {
		t.Errorf("Expected 2 pending requests, got %d", len(pendingRequests))
	}

	for _, req := range pendingRequests {
		if req.Status != model.AccountRequestPending {
			t.Errorf("Expected pending status, got %s", req.Status)
		}
	}
}

func TestSecureAccountRequestRepository_Delete(t *testing.T) {
	repo, tempDir := createTestRepository(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()

	// Create and store test request
	request := &model.AccountRequest{
		ID:            "delete-test",
		Username:      "deleteuser",
		RequestedRole: model.RoleUser,
		Status:        model.AccountRequestPending,
		CreatedAt:     time.Now(),
	}

	err := repo.Store(ctx, request)
	if err != nil {
		t.Fatalf("Failed to store request: %v", err)
	}

	// Verify request exists
	_, err = repo.GetByID(ctx, "delete-test")
	if err != nil {
		t.Fatalf("Request should exist before deletion: %v", err)
	}

	// Delete request
	err = repo.Delete(ctx, "delete-test")
	if err != nil {
		t.Fatalf("Failed to delete request: %v", err)
	}

	// Verify request is gone
	_, err = repo.GetByID(ctx, "delete-test")
	if err != model.ErrAccountRequestNotFound {
		t.Errorf("Expected %v after deletion, got %v", model.ErrAccountRequestNotFound, err)
	}

	// Test deleting non-existent request
	err = repo.Delete(ctx, "non-existent")
	if err != model.ErrAccountRequestNotFound {
		t.Errorf("Expected %v when deleting non-existent request, got %v", model.ErrAccountRequestNotFound, err)
	}
}

func TestSecureAccountRequestRepository_GetPendingRequests(t *testing.T) {
	repo, tempDir := createTestRepository(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()

	// Create requests with mixed statuses
	requests := []*model.AccountRequest{
		{
			ID:        "pending-a",
			Username:  "pendinga",
			Status:    model.AccountRequestPending,
			CreatedAt: time.Now(),
		},
		{
			ID:        "approved-a",
			Username:  "approveda",
			Status:    model.AccountRequestApproved,
			CreatedAt: time.Now(),
		},
		{
			ID:        "pending-b",
			Username:  "pendingb",
			Status:    model.AccountRequestPending,
			CreatedAt: time.Now(),
		},
	}

	// Store all requests
	for _, req := range requests {
		if err := repo.Store(ctx, req); err != nil {
			t.Fatalf("Failed to store request %s: %v", req.ID, err)
		}
	}

	// Get pending requests
	pendingRequests, err := repo.GetPendingRequests(ctx)
	if err != nil {
		t.Fatalf("Failed to get pending requests: %v", err)
	}

	// Should only return the 2 pending requests
	if len(pendingRequests) != 2 {
		t.Errorf("Expected 2 pending requests, got %d", len(pendingRequests))
	}

	// Verify all returned requests are pending
	for _, req := range pendingRequests {
		if req.Status != model.AccountRequestPending {
			t.Errorf("Expected pending status, got %s for request %s", req.Status, req.ID)
		}
	}
}

func TestSecureAccountRequestRepository_FileNotFound(t *testing.T) {
	repo, tempDir := createTestRepository(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()

	// Try to load non-existent database
	_, err := repo.Load(ctx)
	if err != model.ErrAccountRequestDatabaseNotFound {
		t.Errorf("Expected %v for non-existent file, got %v", model.ErrAccountRequestDatabaseNotFound, err)
	}

	// Exists should return false
	if repo.Exists() {
		t.Error("Expected Exists() to return false for non-existent file")
	}

	// GetByID should handle non-existent database gracefully
	_, err = repo.GetByID(ctx, "any-id")
	if err != model.ErrAccountRequestDatabaseNotFound {
		t.Errorf("Expected %v when database doesn't exist, got %v", model.ErrAccountRequestDatabaseNotFound, err)
	}
}
