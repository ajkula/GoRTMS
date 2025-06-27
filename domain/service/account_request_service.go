package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/inbound"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

type accountRequestService struct {
	repo           outbound.AccountRequestRepository
	userRepo       outbound.UserRepository
	crypto         outbound.CryptoService
	messageService inbound.MessageService
	authService    inbound.AuthService
	logger         outbound.Logger
}

func NewAccountRequestService(
	repo outbound.AccountRequestRepository,
	userRepo outbound.UserRepository,
	crypto outbound.CryptoService,
	messageService inbound.MessageService,
	authService inbound.AuthService,
	logger outbound.Logger,
) inbound.AccountRequestService {
	return &accountRequestService{
		repo:           repo,
		userRepo:       userRepo,
		crypto:         crypto,
		messageService: messageService,
		authService:    authService,
		logger:         logger,
	}
}

func (s *accountRequestService) CreateAccountRequest(ctx context.Context, options *inbound.CreateAccountRequestOptions) (*model.AccountRequest, error) {
	s.logger.Info("Creating account request", "username", options.Username, "role", options.RequestedRole)

	// inputs validation
	if options.Username == "" {
		return nil, model.ErrUsernameAlreadyTaken
	}
	if options.Password == "" {
		return nil, model.ErrInvalidRequestedRole
	}
	if options.RequestedRole != model.RoleUser && options.RequestedRole != model.RoleAdmin {
		return nil, model.ErrInvalidRequestedRole
	}

	// check username availability
	if err := s.CheckUsernameAvailability(ctx, options.Username); err != nil {
		return nil, err
	}

	// salt and hash password
	var salt [16]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return nil, err
	}

	passwordHash := s.crypto.HashPassword(options.Password, salt)

	request := &model.AccountRequest{
		ID:            uuid.New().String(),
		Username:      options.Username,
		RequestedRole: options.RequestedRole,
		Status:        model.AccountRequestPending,
		CreatedAt:     time.Now(),
		PasswordHash:  passwordHash,
		Salt:          salt,
	}

	if err := s.repo.Store(ctx, request); err != nil {
		s.logger.Error("Failed to store account request", "error", err)
		return nil, err
	}

	// sends to SYSTEM queue
	if err := s.sendToSystemQueue(ctx, request); err != nil {
		s.logger.Error("Failed to send request to system queue", "error", err, "requestID", request.ID)
		// noop
	}

	s.logger.Info("Account request created successfully", "requestID", request.ID, "username", request.Username)
	return request, nil
}

func (s *accountRequestService) GetAccountRequest(ctx context.Context, requestID string) (*model.AccountRequest, error) {
	return s.repo.GetByID(ctx, requestID)
}

func (s *accountRequestService) ListAccountRequests(ctx context.Context, status *model.AccountRequestStatus) ([]*model.AccountRequest, error) {
	return s.repo.List(ctx, status)
}

func (s *accountRequestService) ReviewAccountRequest(ctx context.Context, requestID string, options *inbound.ReviewAccountRequestOptions) (*model.AccountRequest, error) {
	s.logger.Info("Reviewing account request", "requestID", requestID, "approve", options.Approve, "reviewedBy", options.ReviewedBy)

	request, err := s.repo.GetByID(ctx, requestID)
	if err != nil {
		return nil, err
	}

	// can request be reviewed
	if !request.CanBeReviewed() {
		return nil, model.ErrAccountRequestAlreadyReviewed
	}

	// updates request based on review decision
	now := time.Now()
	request.ReviewedAt = &now
	request.ReviewedBy = options.ReviewedBy

	if options.Approve {
		request.Status = model.AccountRequestApproved

		// approved role or default to requested role
		if options.ApprovedRole != nil {
			request.ApprovedRole = options.ApprovedRole
		} else {
			request.ApprovedRole = &request.RequestedRole
		}

		// create the user account
		if err := s.createUserFromRequest(ctx, request); err != nil {
			s.logger.Error("Failed to create user from approved request", "error", err, "requestID", requestID)
			return nil, err
		}

		s.logger.Info("Account request approved and user created", "requestID", requestID, "username", request.Username, "role", *request.ApprovedRole)
	} else {
		// rejection logic
		request.Status = model.AccountRequestRejected
		request.RejectReason = options.RejectReason

		s.logger.Info("Account request rejected", "requestID", requestID, "reason", options.RejectReason)
	}

	if err := s.repo.Store(ctx, request); err != nil {
		return nil, err
	}

	return request, nil
}

func (s *accountRequestService) DeleteAccountRequest(ctx context.Context, requestID string) error {
	s.logger.Info("Deleting account request", "requestID", requestID)
	return s.repo.Delete(ctx, requestID)
}

func (s *accountRequestService) CheckUsernameAvailability(ctx context.Context, username string) error {
	if _, exists := s.authService.GetUser(username); exists {
		return model.ErrUsernameAlreadyTaken
	}

	// checking for pending requests
	existingRequest, err := s.repo.GetByUsername(ctx, username)
	if err == nil && existingRequest != nil {
		return model.ErrAccountRequestAlreadyExists
	}

	return nil
}

func (s *accountRequestService) SyncPendingRequests(ctx context.Context) error {
	s.logger.Info("Synchronizing pending requests with system queue")

	pendingRequests, err := s.repo.GetPendingRequests(ctx)
	if err != nil {
		return err
	}

	for _, request := range pendingRequests {
		if err := s.sendToSystemQueue(ctx, request); err != nil {
			s.logger.Error("Failed to sync request to system queue", "error", err, "requestID", request.ID)
			// noop
		}
	}

	s.logger.Info("Synchronized pending requests", "count", len(pendingRequests))
	return nil
}

// sends an account request notification to the SYSTEM queue
func (s *accountRequestService) sendToSystemQueue(ctx context.Context, request *model.AccountRequest) error {
	notification := map[string]any{
		"type":          "account_request",
		"requestID":     request.ID,
		"username":      request.Username,
		"requestedRole": request.RequestedRole,
		"status":        request.Status,
		"createdAt":     request.CreatedAt,
	}

	payload, err := json.Marshal(notification)
	if err != nil {
		return err
	}

	// message creation
	message := &model.Message{
		ID:        "account-req-" + request.ID,
		Payload:   payload,
		Headers:   map[string]string{"Content-Type": "application/json"},
		Metadata:  map[string]any{"source": "account_request_service"},
		Timestamp: time.Now(),
	}

	// sends to SYSTEM domain, _account_requests queue
	return s.messageService.PublishMessage("SYSTEM", "_account_requests", message)
}

// creates an actual user account from an approved request
func (s *accountRequestService) createUserFromRequest(ctx context.Context, request *model.AccountRequest) error {
	if request.Status != model.AccountRequestApproved || request.ApprovedRole == nil {
		return model.ErrAccountRequestInvalidStatus
	}

	s.logger.Info("Creating user from approved account request",
		"username", request.Username,
		"requestID", request.ID,
		"approvedRole", *request.ApprovedRole)

	// Uses the pre-hashed password from the request
	user, err := s.authService.CreateUserWithHash(
		request.Username,
		request.PasswordHash,
		request.Salt,
		*request.ApprovedRole,
	)

	if err != nil {
		s.logger.Error("Failed to create user from account request",
			"error", err,
			"username", request.Username,
			"requestID", request.ID)
		return err
	}

	// This is a bit of a hack since AuthService.CreateUser expects a plain password
	// I'll need to add a method to AuthService for creating users with pre-hashed passwords
	// For now, create user directly through the user repository
	db, err := s.userRepo.Load()
	if err != nil && err != model.ErrUserDatabaseNotFound {
		return err
	}

	if db == nil {
		db = &model.UserDatabase{
			Users: make(map[string]*model.User),
			Salt:  s.crypto.GenerateSalt(),
		}
	}

	db.Users[user.Username] = user
	return s.userRepo.Save(db)
}
