package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/gorilla/mux"
)

func TestServiceHandler_CreateService(t *testing.T) {
	// Setup
	logger := &mockLogger{}
	repo := createTestRepository(t, logger)
	handler := NewServiceHandler(repo, logger)

	// Test request
	createReq := model.ServiceAccountCreateRequest{
		Name:        "Test Payment Service",
		Permissions: []string{"publish:orders", "consume:payments"},
		IPWhitelist: []string{"127.0.0.1"},
	}

	body, _ := json.Marshal(createReq)
	req := httptest.NewRequest("POST", "/api/admin/services", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.CreateService(w, req)

	// Verify response
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response struct {
		*model.ServiceAccountView
		Message string `json:"message"`
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify secret is visible in creation response
	if response.Secret == "" || response.Secret == "••••••••••••••••" {
		t.Error("Expected secret to be visible in creation response")
	}

	if !response.IsDisclosed {
		t.Error("Expected isDisclosed to be true after creation")
	}

	if !strings.Contains(response.Message, "SAVE THIS SECRET NOW") {
		t.Error("Expected warning message about secret visibility")
	}

	// Verify service is properly stored
	services, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("Failed to list services: %v", err)
	}

	if len(services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(services))
	}

	storedService := services[0]
	if !storedService.IsDisclosed {
		t.Error("Expected stored service to be marked as disclosed")
	}
}

func TestServiceHandler_GetService_SecretMasked(t *testing.T) {
	// Setup
	logger := &mockLogger{}
	repo := createTestRepository(t, logger)
	handler := NewServiceHandler(repo, logger)

	// Create a service with disclosed secret
	service := &model.ServiceAccount{
		ID:          "test-service-001",
		Name:        "Test Service",
		Secret:      "super-secret-key",
		IsDisclosed: true, // Already disclosed
		Permissions: []string{"publish:orders"},
		Enabled:     true,
	}

	err := repo.Create(context.Background(), service)
	if err != nil {
		t.Fatalf("Failed to create test service: %v", err)
	}

	// Create request
	req := httptest.NewRequest("GET", "/api/admin/services/test-service-001", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "test-service-001"})
	w := httptest.NewRecorder()

	// Execute
	handler.GetService(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response model.ServiceAccountView
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify secret is masked
	if response.Secret != "••••••••••••••••" {
		t.Errorf("Expected masked secret, got: %s", response.Secret)
	}

	if !response.IsDisclosed {
		t.Error("Expected isDisclosed to be true")
	}
}

func TestServiceHandler_RotateSecret(t *testing.T) {
	// Setup
	logger := &mockLogger{}
	repo := createTestRepository(t, logger)
	handler := NewServiceHandler(repo, logger)

	// Create initial service
	service := &model.ServiceAccount{
		ID:          "test-service-001",
		Name:        "Test Service",
		Secret:      "old-secret-key",
		IsDisclosed: true,
		Permissions: []string{"publish:orders"},
		Enabled:     true,
	}

	err := repo.Create(context.Background(), service)
	if err != nil {
		t.Fatalf("Failed to create test service: %v", err)
	}

	// Create rotation request
	req := httptest.NewRequest("POST", "/api/admin/services/test-service-001/rotate-secret", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "test-service-001"})
	w := httptest.NewRecorder()

	// Execute
	handler.RotateSecret(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response struct {
		*model.ServiceAccountView
		Message string `json:"message"`
		Rotated bool   `json:"rotated"`
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify new secret is visible
	if response.Secret == "" || response.Secret == "••••••••••••••••" {
		t.Error("Expected new secret to be visible in rotation response")
	}

	if response.Secret == "old-secret-key" {
		t.Error("Expected new secret to be different from old secret")
	}

	if !response.IsDisclosed {
		t.Error("Expected isDisclosed to be true after rotation")
	}

	if !response.Rotated {
		t.Error("Expected rotated flag to be true")
	}

	if !strings.Contains(response.Message, "NEW SECRET GENERATED") {
		t.Error("Expected message about new secret generation")
	}

	// Verify service is updated in storage
	retrievedService, err := repo.GetByID(context.Background(), "test-service-001")
	if err != nil {
		t.Fatalf("Failed to retrieve service: %v", err)
	}

	if retrievedService.Secret == "old-secret-key" {
		t.Error("Expected service secret to be updated in storage")
	}

	if !retrievedService.IsDisclosed {
		t.Error("Expected service to be marked as disclosed in storage")
	}
}

func TestServiceHandler_ListServices_SecretsAlwaysMasked(t *testing.T) {
	// Setup
	logger := &mockLogger{}
	repo := createTestRepository(t, logger)
	handler := NewServiceHandler(repo, logger)

	// Create services with different disclosure states
	services := []*model.ServiceAccount{
		{
			ID:          "service-001",
			Name:        "Service 1",
			Secret:      "secret-1",
			IsDisclosed: true,
			Permissions: []string{"publish:orders"},
			Enabled:     true,
		},
		{
			ID:          "service-002",
			Name:        "Service 2",
			Secret:      "secret-2",
			IsDisclosed: false, // Not yet disclosed
			Permissions: []string{"consume:payments"},
			Enabled:     true,
		},
	}

	for _, service := range services {
		err := repo.Create(context.Background(), service)
		if err != nil {
			t.Fatalf("Failed to create test service: %v", err)
		}
	}

	// Create request
	req := httptest.NewRequest("GET", "/api/admin/services", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.ListServices(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response struct {
		Services []*model.ServiceAccountView `json:"services"`
		Count    int                         `json:"count"`
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Count != 2 {
		t.Errorf("Expected 2 services, got %d", response.Count)
	}

	// Verify secrets are masked for disclosed services, empty for non-disclosed
	for _, serviceView := range response.Services {
		if serviceView.IsDisclosed && serviceView.Secret != "••••••••••••••••" {
			t.Errorf("Expected masked secret for disclosed service %s, got: %s", serviceView.ID, serviceView.Secret)
		}
		if !serviceView.IsDisclosed && serviceView.Secret != "" {
			t.Errorf("Expected empty secret for non-disclosed service %s, got: %s", serviceView.ID, serviceView.Secret)
		}
	}
}

func TestServiceHandler_DeleteService(t *testing.T) {
	// Setup
	logger := &mockLogger{}
	repo := createTestRepository(t, logger)
	handler := NewServiceHandler(repo, logger)

	// Create test service
	service := &model.ServiceAccount{
		ID:          "test-service-001",
		Name:        "Test Service",
		Secret:      "secret-key",
		IsDisclosed: true,
		Permissions: []string{"publish:orders"},
		Enabled:     true,
	}

	err := repo.Create(context.Background(), service)
	if err != nil {
		t.Fatalf("Failed to create test service: %v", err)
	}

	// Create deletion request
	req := httptest.NewRequest("DELETE", "/api/admin/services/test-service-001", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "test-service-001"})
	w := httptest.NewRecorder()

	// Execute
	handler.DeleteService(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify service is deleted
	_, err = repo.GetByID(context.Background(), "test-service-001")
	if err == nil {
		t.Error("Expected service to be deleted")
	}
}

func TestServiceHandler_ValidateCreateRequest(t *testing.T) {
	handler := &ServiceHandler{}

	testCases := []struct {
		name      string
		request   model.ServiceAccountCreateRequest
		expectErr bool
	}{
		{
			name: "Valid request",
			request: model.ServiceAccountCreateRequest{
				Name:        "Valid Service",
				Permissions: []string{"publish:orders", "consume:*"},
			},
			expectErr: false,
		},
		{
			name: "Empty name",
			request: model.ServiceAccountCreateRequest{
				Name:        "",
				Permissions: []string{"publish:orders"},
			},
			expectErr: true,
		},
		{
			name: "Name too short",
			request: model.ServiceAccountCreateRequest{
				Name:        "AB",
				Permissions: []string{"publish:orders"},
			},
			expectErr: true,
		},
		{
			name: "No permissions",
			request: model.ServiceAccountCreateRequest{
				Name:        "Valid Service",
				Permissions: []string{},
			},
			expectErr: true,
		},
		{
			name: "Invalid permission format",
			request: model.ServiceAccountCreateRequest{
				Name:        "Valid Service",
				Permissions: []string{"invalid-permission"},
			},
			expectErr: true,
		},
		{
			name: "Global wildcard permission",
			request: model.ServiceAccountCreateRequest{
				Name:        "Admin Service",
				Permissions: []string{"*"},
			},
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := handler.validateCreateRequest(&tc.request)
			hasErr := err != nil

			if hasErr != tc.expectErr {
				t.Errorf("Expected error: %v, got error: %v (%s)", tc.expectErr, hasErr, err)
			}
		})
	}
}
