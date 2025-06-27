package inbound

import (
	"context"

	"github.com/ajkula/GoRTMS/domain/model"
)

// CreateAccountRequestOptions contains options for creating an account request
type CreateAccountRequestOptions struct {
	Username      string         `json:"username"`
	Password      string         `json:"password"`
	RequestedRole model.UserRole `json:"requestedRole"`
}

// ReviewAccountRequestOptions contains options for reviewing an account request
type ReviewAccountRequestOptions struct {
	Approve      bool            `json:"approve"`
	ApprovedRole *model.UserRole `json:"approvedRole,omitempty"` // Can be different from requested role
	RejectReason string          `json:"rejectReason,omitempty"` // Required if Approve is false
	ReviewedBy   string          `json:"reviewedBy"`             // Admin username performing the review
}

// AccountRequestService defines operations for managing account requests
type AccountRequestService interface {
	// CreateAccountRequest creates a new account request
	CreateAccountRequest(ctx context.Context, options *CreateAccountRequestOptions) (*model.AccountRequest, error)

	// GetAccountRequest retrieves an account request by ID
	GetAccountRequest(ctx context.Context, requestID string) (*model.AccountRequest, error)

	// ListAccountRequests lists all account requests with optional status filter
	ListAccountRequests(ctx context.Context, status *model.AccountRequestStatus) ([]*model.AccountRequest, error)

	// ReviewAccountRequest approves or rejects an account request
	ReviewAccountRequest(ctx context.Context, requestID string, options *ReviewAccountRequestOptions) (*model.AccountRequest, error)

	// DeleteAccountRequest removes an account request (typically after processing)
	DeleteAccountRequest(ctx context.Context, requestID string) error

	// CheckUsernameAvailability checks if a username is available for new requests
	CheckUsernameAvailability(ctx context.Context, username string) error

	// SyncPendingRequests synchronizes pending requests with the message queue
	SyncPendingRequests(ctx context.Context) error
}
