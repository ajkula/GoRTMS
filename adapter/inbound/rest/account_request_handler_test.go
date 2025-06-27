package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/inbound"
)

// Mock AccountRequestService for testing
type mockAccountRequestService struct {
	requests map[string]*model.AccountRequest
}

func (m *mockAccountRequestService) CreateAccountRequest(ctx context.Context, options *inbound.CreateAccountRequestOptions) (*model.AccountRequest, error) {
	// Check for conflicts
	for _, req := range m.requests {
		if req.Username == options.Username {
			return nil, model.ErrAccountRequestAlreadyExists
		}
	}

	request := &model.AccountRequest{
		ID:            "req-" + options.Username,
		Username:      options.Username,
		RequestedRole: options.RequestedRole,
		Status:        model.AccountRequestPending,
		CreatedAt:     time.Now(),
		PasswordHash:  "hashed_" + options.Password,
	}

	m.requests[request.ID] = request
	return request, nil
}

func (m *mockAccountRequestService) GetAccountRequest(ctx context.Context, requestID string) (*model.AccountRequest, error) {
	if req, exists := m.requests[requestID]; exists {
		return req, nil
	}
	return nil, model.ErrAccountRequestNotFound
}

func (m *mockAccountRequestService) ListAccountRequests(ctx context.Context, status *model.AccountRequestStatus) ([]*model.AccountRequest, error) {
	var result []*model.AccountRequest
	for _, req := range m.requests {
		if status == nil || req.Status == *status {
			result = append(result, req)
		}
	}
	return result, nil
}

func (m *mockAccountRequestService) ReviewAccountRequest(ctx context.Context, requestID string, options *inbound.ReviewAccountRequestOptions) (*model.AccountRequest, error) {
	req, exists := m.requests[requestID]
	if !exists {
		return nil, model.ErrAccountRequestNotFound
	}

	if !req.CanBeReviewed() {
		return nil, model.ErrAccountRequestAlreadyReviewed
	}

	now := time.Now()
	req.ReviewedAt = &now
	req.ReviewedBy = options.ReviewedBy

	if options.Approve {
		req.Status = model.AccountRequestApproved
		if options.ApprovedRole != nil {
			req.ApprovedRole = options.ApprovedRole
		} else {
			req.ApprovedRole = &req.RequestedRole
		}
	} else {
		req.Status = model.AccountRequestRejected
		req.RejectReason = options.RejectReason
	}

	return req, nil
}

func (m *mockAccountRequestService) DeleteAccountRequest(ctx context.Context, requestID string) error {
	if _, exists := m.requests[requestID]; !exists {
		return model.ErrAccountRequestNotFound
	}
	delete(m.requests, requestID)
	return nil
}

func (m *mockAccountRequestService) CheckUsernameAvailability(ctx context.Context, username string) error {
	for _, req := range m.requests {
		if req.Username == username {
			return model.ErrAccountRequestAlreadyExists
		}
	}
	return nil
}

func (m *mockAccountRequestService) SyncPendingRequests(ctx context.Context) error {
	return nil
}

func createTestHandler() *AccountRequestHandler {
	return &AccountRequestHandler{
		accountRequestService: &mockAccountRequestService{
			requests: make(map[string]*model.AccountRequest),
		},
		authService: &mockAuthService{
			users: make(map[string]*model.User),
		},
		logger: &mockLogger{},
	}
}

func TestAccountRequestHandler_CreateAccountRequest(t *testing.T) {
	handler := createTestHandler()

	t.Run("successful creation", func(t *testing.T) {
		requestBody := CreateAccountRequestRequest{
			Username:      "testuser",
			Password:      "password123",
			RequestedRole: model.RoleUser,
		}

		jsonBody, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", "/api/account-requests", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handler.CreateAccountRequest(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("Expected status %d, got %d", http.StatusCreated, rr.Code)
		}

		var response AccountRequestApiResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Request.Username != requestBody.Username {
			t.Errorf("Expected username %s, got %s", requestBody.Username, response.Request.Username)
		}

		if response.Request.Status != model.AccountRequestPending {
			t.Errorf("Expected status %s, got %s", model.AccountRequestPending, response.Request.Status)
		}
	})

	t.Run("invalid request body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/account-requests", bytes.NewBuffer([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handler.CreateAccountRequest(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("missing username", func(t *testing.T) {
		requestBody := CreateAccountRequestRequest{
			Password:      "password123",
			RequestedRole: model.RoleUser,
		}

		jsonBody, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", "/api/account-requests", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handler.CreateAccountRequest(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})
}

func TestAccountRequestHandler_ListAccountRequests(t *testing.T) {
	handler := createTestHandler()

	// Create test requests
	service := handler.accountRequestService.(*mockAccountRequestService)
	service.requests["req-1"] = &model.AccountRequest{
		ID:            "req-1",
		Username:      "user1",
		RequestedRole: model.RoleUser,
		Status:        model.AccountRequestPending,
		CreatedAt:     time.Now(),
	}
	service.requests["req-2"] = &model.AccountRequest{
		ID:            "req-2",
		Username:      "user2",
		RequestedRole: model.RoleAdmin,
		Status:        model.AccountRequestApproved,
		CreatedAt:     time.Now(),
	}

	t.Run("list all requests", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/account-requests", nil)
		rr := httptest.NewRecorder()

		handler.ListAccountRequests(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response AccountRequestListResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Count != 2 {
			t.Errorf("Expected 2 requests, got %d", response.Count)
		}
	})

	t.Run("list pending requests", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/account-requests?status=pending", nil)
		rr := httptest.NewRecorder()

		handler.ListAccountRequests(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response AccountRequestListResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Count != 1 {
			t.Errorf("Expected 1 pending request, got %d", response.Count)
		}

		if response.Requests[0].Status != model.AccountRequestPending {
			t.Errorf("Expected pending status, got %s", response.Requests[0].Status)
		}
	})
}

func TestAccountRequestHandler_ReviewAccountRequest(t *testing.T) {
	handler := createTestHandler()

	// Create test request
	service := handler.accountRequestService.(*mockAccountRequestService)
	service.requests["req-review"] = &model.AccountRequest{
		ID:            "req-review",
		Username:      "reviewuser",
		RequestedRole: model.RoleUser,
		Status:        model.AccountRequestPending,
		CreatedAt:     time.Now(),
	}

	t.Run("approve request", func(t *testing.T) {
		reviewBody := ReviewAccountRequestRequest{
			Approve: true,
		}

		jsonBody, _ := json.Marshal(reviewBody)
		req := httptest.NewRequest("POST", "/api/admin/account-requests/req-review/review", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		// Add user to context
		user := &model.User{Username: "admin", Role: model.RoleAdmin}
		ctx := context.WithValue(req.Context(), UserContextKey, user)
		req = req.WithContext(ctx)

		// Add mux vars
		req = mux.SetURLVars(req, map[string]string{"requestId": "req-review"})

		rr := httptest.NewRecorder()
		handler.ReviewAccountRequest(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response AccountRequestApiResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Request.Status != model.AccountRequestApproved {
			t.Errorf("Expected approved status, got %s", response.Request.Status)
		}
	})

	t.Run("reject without reason", func(t *testing.T) {
		reviewBody := ReviewAccountRequestRequest{
			Approve: false,
			// Missing RejectReason
		}

		jsonBody, _ := json.Marshal(reviewBody)
		req := httptest.NewRequest("POST", "/api/admin/account-requests/req-review/review", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		req = mux.SetURLVars(req, map[string]string{"requestId": "req-review"})

		rr := httptest.NewRecorder()
		handler.ReviewAccountRequest(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})
}
