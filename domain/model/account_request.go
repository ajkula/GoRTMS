package model

import (
	"time"
)

// AccountRequestStatus represents the status of an account request
type AccountRequestStatus string

const (
	// AccountRequestPending indicates the request is awaiting review
	AccountRequestPending AccountRequestStatus = "pending"

	// AccountRequestApproved indicates the request has been approved
	AccountRequestApproved AccountRequestStatus = "approved"

	// AccountRequestRejected indicates the request has been rejected
	AccountRequestRejected AccountRequestStatus = "rejected"
)

// AccountRequest represents a user account creation request
type AccountRequest struct {
	ID            string               `json:"id"`            // Unique identifier for the request
	Username      string               `json:"username"`      // Requested username
	RequestedRole UserRole             `json:"requestedRole"` // Role requested by the user
	Status        AccountRequestStatus `json:"status"`        // Current status of the request
	CreatedAt     time.Time            `json:"createdAt"`     // Request creation timestamp
	ReviewedAt    *time.Time           `json:"reviewedAt"`    // Review timestamp (nil if not reviewed)
	ReviewedBy    string               `json:"reviewedBy"`    // Username of the admin who reviewed
	ApprovedRole  *UserRole            `json:"approvedRole"`  // Role actually granted (may differ from requested)
	RejectReason  string               `json:"rejectReason"`  // Reason for rejection (empty if not rejected)
	PasswordHash  string               `json:"passwordHash"`  // Hashed password provided during request
	Salt          [16]byte             `json:"salt"`          // Salt used for password hashing
}

// AccountRequestDatabase represents the storage structure for account requests
type AccountRequestDatabase struct {
	Requests map[string]*AccountRequest `json:"requests"` // Map of requests by ID
	Salt     [32]byte                   `json:"salt"`     // Database encryption salt
}

// AccountRequestResponse represents the API response for account requests
type AccountRequestResponse struct {
	ID            string               `json:"id"`
	Username      string               `json:"username"`
	RequestedRole UserRole             `json:"requestedRole"`
	Status        AccountRequestStatus `json:"status"`
	CreatedAt     time.Time            `json:"createdAt"`
	ReviewedAt    *time.Time           `json:"reviewedAt,omitempty"`
	ReviewedBy    string               `json:"reviewedBy,omitempty"`
	ApprovedRole  *UserRole            `json:"approvedRole,omitempty"`
	RejectReason  string               `json:"rejectReason,omitempty"`
}

// ToResponse converts an AccountRequest to its API response format
func (ar *AccountRequest) ToResponse() *AccountRequestResponse {
	return &AccountRequestResponse{
		ID:            ar.ID,
		Username:      ar.Username,
		RequestedRole: ar.RequestedRole,
		Status:        ar.Status,
		CreatedAt:     ar.CreatedAt,
		ReviewedAt:    ar.ReviewedAt,
		ReviewedBy:    ar.ReviewedBy,
		ApprovedRole:  ar.ApprovedRole,
		RejectReason:  ar.RejectReason,
	}
}

// IsReviewed returns true if the request has been reviewed (approved or rejected)
func (ar *AccountRequest) IsReviewed() bool {
	return ar.Status != AccountRequestPending
}

// CanBeReviewed returns true if the request can be reviewed
func (ar *AccountRequest) CanBeReviewed() bool {
	return ar.Status == AccountRequestPending
}
