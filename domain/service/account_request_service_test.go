package service

import (
	"context"
	"testing"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/inbound"
	"github.com/stretchr/testify/mock"
)

// Mock implementations for testing
type mockAccountRequestRepository struct {
	requests  map[string]*model.AccountRequest
	usernames map[string]*model.AccountRequest
}

func (m *mockAccountRequestRepository) Save(ctx context.Context, db *model.AccountRequestDatabase) error {
	return nil
}

func (m *mockAccountRequestRepository) Load(ctx context.Context) (*model.AccountRequestDatabase, error) {
	return &model.AccountRequestDatabase{Requests: m.requests}, nil
}

func (m *mockAccountRequestRepository) Exists() bool {
	return len(m.requests) > 0
}

func (m *mockAccountRequestRepository) Store(ctx context.Context, request *model.AccountRequest) error {
	m.requests[request.ID] = request
	m.usernames[request.Username] = request
	return nil
}

func (m *mockAccountRequestRepository) GetByID(ctx context.Context, requestID string) (*model.AccountRequest, error) {
	if req, exists := m.requests[requestID]; exists {
		return req, nil
	}
	return nil, model.ErrAccountRequestNotFound
}

func (m *mockAccountRequestRepository) GetByUsername(ctx context.Context, username string) (*model.AccountRequest, error) {
	if req, exists := m.usernames[username]; exists {
		return req, nil
	}
	return nil, model.ErrAccountRequestNotFound
}

func (m *mockAccountRequestRepository) List(ctx context.Context, status *model.AccountRequestStatus) ([]*model.AccountRequest, error) {
	var result []*model.AccountRequest
	for _, req := range m.requests {
		if status == nil || req.Status == *status {
			result = append(result, req)
		}
	}
	return result, nil
}

func (m *mockAccountRequestRepository) Delete(ctx context.Context, requestID string) error {
	if req, exists := m.requests[requestID]; exists {
		delete(m.requests, requestID)
		delete(m.usernames, req.Username)
		return nil
	}
	return model.ErrAccountRequestNotFound
}

func (m *mockAccountRequestRepository) GetPendingRequests(ctx context.Context) ([]*model.AccountRequest, error) {
	status := model.AccountRequestPending
	return m.List(ctx, &status)
}

type mockUserRepository struct {
	db *model.UserDatabase
}

func (m *mockUserRepository) Save(db *model.UserDatabase) error {
	m.db = db
	return nil
}

func (m *mockUserRepository) Load() (*model.UserDatabase, error) {
	if m.db != nil {
		return m.db, nil
	}
	return nil, model.ErrUserDatabaseNotFound
}

func (m *mockUserRepository) Exists() bool {
	return m.db != nil
}

type mockMessageService struct {
	publishedMessages []*model.Message
}

func (m *mockMessageService) PublishMessage(domainName, queueName string, message *model.Message) error {
	m.publishedMessages = append(m.publishedMessages, message)
	return nil
}

func (m *mockMessageService) SubscribeToQueue(domainName, queueName string, handler model.MessageHandler) (string, error) {
	return "sub-id", nil
}

func (m *mockMessageService) UnsubscribeFromQueue(domainName, queueName, subscriptionID string) error {
	return nil
}

func (m *mockMessageService) ConsumeMessageWithGroup(ctx context.Context, domainName, queueName, groupID string, options *inbound.ConsumeOptions) (*model.Message, error) {
	return nil, nil
}

func (m *mockMessageService) GetMessagesAfterIndex(ctx context.Context, domainName, queueName string, startIndex int64, limit int) ([]*model.Message, error) {
	return nil, nil
}

type mockAuthService struct {
	users map[string]*model.User
}

func (m *mockAuthService) CreateUserWithHash(username, passwordHash string, salt [16]byte, role model.UserRole) (*model.User, error) {
	user := &model.User{
		ID:           "mock-user-" + username,
		Username:     username,
		PasswordHash: passwordHash,
		Salt:         salt,
		Role:         role,
		CreatedAt:    time.Now(),
		Enabled:      true,
	}

	m.users[username] = user
	return user, nil
}

func (m *mockAuthService) Login(username, password string) (*model.User, string, error) {
	return nil, "", nil
}

func (m *mockAuthService) ValidateToken(token string) (*model.User, error) {
	return nil, nil
}

func (m *mockAuthService) CreateUser(username, password string, role model.UserRole) (*model.User, error) {
	return nil, nil
}

func (m *mockAuthService) UpdateUser(userID string, updates inbound.UpdateUserRequest, isAdmin bool) (*model.User, error) {
	return nil, nil
}

func (m *mockAuthService) GetUser(username string) (*model.User, bool) {
	user, exists := m.users[username]
	return user, exists
}

func (m *mockAuthService) ListUsers() ([]*model.User, error) {
	return nil, nil
}

func (m *mockAuthService) BootstrapAdmin() (*model.User, string, error) {
	return nil, "", nil
}

func (m *mockAuthService) GenerateToken(user *model.User, issuedAt time.Time) (string, error) {
	return "token", nil
}

func (m *mockAuthService) UpdatePassword(user *model.User, old, new string) error {
	return nil
}

// type mockLogger struct{}

// func (m *mockLogger) Error(msg string, args ...any) {}
// func (m *mockLogger) Warn(msg string, args ...any)  {}
// func (m *mockLogger) Info(msg string, args ...any)  {}
// func (m *mockLogger) Debug(msg string, args ...any) {}
// func (m *mockLogger) UpdateLevel(logLvl string)     {}
// func (m *mockLogger) Shutdown()                     {}

func createTestService() *accountRequestService {
	// Create and configure the mock crypto service
	mockCrypto := &MockCryptoService{}

	// Configure necessary stubs
	mockCrypto.On("HashPassword", mock.AnythingOfType("string"), mock.AnythingOfType("[16]uint8")).Return("mocked_hash_password")
	mockCrypto.On("GenerateSalt").Return([32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32})

	return &accountRequestService{
		repo: &mockAccountRequestRepository{
			requests:  make(map[string]*model.AccountRequest),
			usernames: make(map[string]*model.AccountRequest),
		},
		userRepo:       &mockUserRepository{},
		crypto:         mockCrypto,
		messageService: &mockMessageService{},
		authService: &mockAuthService{
			users: make(map[string]*model.User),
		},
		logger: &mockLogger{},
	}
}

func TestAccountRequestService_CreateAccountRequest(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		service := createTestService()
		ctx := context.Background()

		options := &inbound.CreateAccountRequestOptions{
			Username:      "testuser",
			Password:      "password123",
			RequestedRole: model.RoleUser,
		}

		request, err := service.CreateAccountRequest(ctx, options)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if request == nil {
			t.Fatal("Expected request to be non-nil")
		}

		if request.Username != options.Username {
			t.Errorf("Expected username %s, got %s", options.Username, request.Username)
		}

		if request.RequestedRole != options.RequestedRole {
			t.Errorf("Expected role %s, got %s", options.RequestedRole, request.RequestedRole)
		}

		if request.Status != model.AccountRequestPending {
			t.Errorf("Expected status %s, got %s", model.AccountRequestPending, request.Status)
		}

		if request.ID == "" {
			t.Error("Expected non-empty request ID")
		}

		if request.PasswordHash == "" {
			t.Error("Expected non-empty password hash")
		}
	})

	t.Run("duplicate username rejection", func(t *testing.T) {
		service := createTestService()
		ctx := context.Background()

		// Create first request
		options1 := &inbound.CreateAccountRequestOptions{
			Username:      "duplicate",
			Password:      "password123",
			RequestedRole: model.RoleUser,
		}

		_, err := service.CreateAccountRequest(ctx, options1)
		if err != nil {
			t.Fatalf("First request should succeed, got %v", err)
		}

		// Try to create second request with same username
		options2 := &inbound.CreateAccountRequestOptions{
			Username:      "duplicate",
			Password:      "password456",
			RequestedRole: model.RoleAdmin,
		}

		_, err = service.CreateAccountRequest(ctx, options2)
		if err != model.ErrAccountRequestAlreadyExists {
			t.Errorf("Expected %v, got %v", model.ErrAccountRequestAlreadyExists, err)
		}
	})

	t.Run("invalid role rejection", func(t *testing.T) {
		service := createTestService()
		ctx := context.Background()

		options := &inbound.CreateAccountRequestOptions{
			Username:      "testuser",
			Password:      "password123",
			RequestedRole: model.UserRole("invalid_role"),
		}

		_, err := service.CreateAccountRequest(ctx, options)
		if err != model.ErrInvalidRequestedRole {
			t.Errorf("Expected %v, got %v", model.ErrInvalidRequestedRole, err)
		}
	})
}

func TestAccountRequestService_ReviewAccountRequest(t *testing.T) {
	t.Run("successful approval", func(t *testing.T) {
		service := createTestService()
		ctx := context.Background()

		// Create a pending request first
		createOptions := &inbound.CreateAccountRequestOptions{
			Username:      "testuser",
			Password:      "password123",
			RequestedRole: model.RoleUser,
		}

		request, err := service.CreateAccountRequest(ctx, createOptions)
		if err != nil {
			t.Fatalf("Failed to create test request: %v", err)
		}

		// Review and approve
		reviewOptions := &inbound.ReviewAccountRequestOptions{
			Approve:    true,
			ReviewedBy: "admin",
		}

		reviewedRequest, err := service.ReviewAccountRequest(ctx, request.ID, reviewOptions)
		if err != nil {
			t.Fatalf("Expected no error during review, got %v", err)
		}

		if reviewedRequest.Status != model.AccountRequestApproved {
			t.Errorf("Expected status %s, got %s", model.AccountRequestApproved, reviewedRequest.Status)
		}

		if reviewedRequest.ReviewedBy != "admin" {
			t.Errorf("Expected reviewed by 'admin', got %s", reviewedRequest.ReviewedBy)
		}

		if reviewedRequest.ReviewedAt == nil {
			t.Error("Expected ReviewedAt to be set")
		}

		if reviewedRequest.ApprovedRole == nil {
			t.Error("Expected ApprovedRole to be set")
		} else if *reviewedRequest.ApprovedRole != model.RoleUser {
			t.Errorf("Expected approved role %s, got %s", model.RoleUser, *reviewedRequest.ApprovedRole)
		}
	})

	t.Run("successful rejection", func(t *testing.T) {
		service := createTestService()
		ctx := context.Background()

		// Create a pending request first
		createOptions := &inbound.CreateAccountRequestOptions{
			Username:      "testuser2",
			Password:      "password123",
			RequestedRole: model.RoleAdmin,
		}

		request, err := service.CreateAccountRequest(ctx, createOptions)
		if err != nil {
			t.Fatalf("Failed to create test request: %v", err)
		}

		// Review and reject
		reviewOptions := &inbound.ReviewAccountRequestOptions{
			Approve:      false,
			RejectReason: "Insufficient justification for admin role",
			ReviewedBy:   "admin",
		}

		reviewedRequest, err := service.ReviewAccountRequest(ctx, request.ID, reviewOptions)
		if err != nil {
			t.Fatalf("Expected no error during review, got %v", err)
		}

		if reviewedRequest.Status != model.AccountRequestRejected {
			t.Errorf("Expected status %s, got %s", model.AccountRequestRejected, reviewedRequest.Status)
		}

		if reviewedRequest.RejectReason != reviewOptions.RejectReason {
			t.Errorf("Expected reject reason '%s', got '%s'", reviewOptions.RejectReason, reviewedRequest.RejectReason)
		}
	})

	t.Run("cannot review already reviewed request", func(t *testing.T) {
		service := createTestService()
		ctx := context.Background()

		// Create and approve a request
		createOptions := &inbound.CreateAccountRequestOptions{
			Username:      "testuser3",
			Password:      "password123",
			RequestedRole: model.RoleUser,
		}

		request, _ := service.CreateAccountRequest(ctx, createOptions)

		reviewOptions := &inbound.ReviewAccountRequestOptions{
			Approve:    true,
			ReviewedBy: "admin",
		}

		_, err := service.ReviewAccountRequest(ctx, request.ID, reviewOptions)
		if err != nil {
			t.Fatalf("First review should succeed: %v", err)
		}

		// Try to review again
		_, err = service.ReviewAccountRequest(ctx, request.ID, reviewOptions)
		if err != model.ErrAccountRequestAlreadyReviewed {
			t.Errorf("Expected %v, got %v", model.ErrAccountRequestAlreadyReviewed, err)
		}
	})
}

func TestAccountRequestService_CheckUsernameAvailability(t *testing.T) {
	t.Run("available username", func(t *testing.T) {
		service := createTestService()
		ctx := context.Background()

		err := service.CheckUsernameAvailability(ctx, "available_user")
		if err != nil {
			t.Errorf("Expected username to be available, got error: %v", err)
		}
	})

	t.Run("username taken by existing user", func(t *testing.T) {
		service := createTestService()
		ctx := context.Background()

		// Add existing user to mock auth service
		authService := service.authService.(*mockAuthService)
		authService.users["existing_user"] = &model.User{Username: "existing_user"}

		err := service.CheckUsernameAvailability(ctx, "existing_user")
		if err != model.ErrUsernameAlreadyTaken {
			t.Errorf("Expected %v, got %v", model.ErrUsernameAlreadyTaken, err)
		}
	})

	t.Run("username taken by pending request", func(t *testing.T) {
		service := createTestService()
		ctx := context.Background()

		// Create a pending request
		createOptions := &inbound.CreateAccountRequestOptions{
			Username:      "pending_user",
			Password:      "password123",
			RequestedRole: model.RoleUser,
		}

		_, err := service.CreateAccountRequest(ctx, createOptions)
		if err != nil {
			t.Fatalf("Failed to create test request: %v", err)
		}

		// Check availability of same username
		err = service.CheckUsernameAvailability(ctx, "pending_user")
		if err != model.ErrAccountRequestAlreadyExists {
			t.Errorf("Expected %v, got %v", model.ErrAccountRequestAlreadyExists, err)
		}
	})
}
