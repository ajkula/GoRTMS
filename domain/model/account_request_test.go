package model

import (
	"testing"
	"time"
)

func TestAccountRequest_ToResponse(t *testing.T) {
	// test with minimal data
	t.Run("minimal account request", func(t *testing.T) {
		request := &AccountRequest{
			ID:            "req-123",
			Username:      "testuser",
			RequestedRole: RoleUser,
			Status:        AccountRequestPending,
			CreatedAt:     time.Date(2025, 6, 27, 10, 0, 0, 0, time.UTC),
		}

		response := request.ToResponse()

		if response.ID != request.ID {
			t.Errorf("Expected ID %s, got %s", request.ID, response.ID)
		}
		if response.Username != request.Username {
			t.Errorf("Expected username %s, got %s", request.Username, response.Username)
		}
		if response.RequestedRole != request.RequestedRole {
			t.Errorf("Expected role %s, got %s", request.RequestedRole, response.RequestedRole)
		}
		if response.Status != request.Status {
			t.Errorf("Expected status %s, got %s", request.Status, response.Status)
		}
		if !response.CreatedAt.Equal(request.CreatedAt) {
			t.Errorf("Expected created at %v, got %v", request.CreatedAt, response.CreatedAt)
		}
	})

	// test with complete reviewed data
	t.Run("complete reviewed account request", func(t *testing.T) {
		reviewedAt := time.Date(2025, 6, 27, 11, 0, 0, 0, time.UTC)
		approvedRole := RoleAdmin

		request := &AccountRequest{
			ID:            "req-456",
			Username:      "adminuser",
			RequestedRole: RoleUser,
			Status:        AccountRequestApproved,
			CreatedAt:     time.Date(2025, 6, 27, 10, 0, 0, 0, time.UTC),
			ReviewedAt:    &reviewedAt,
			ReviewedBy:    "admin",
			ApprovedRole:  &approvedRole,
		}

		response := request.ToResponse()

		if response.ReviewedAt == nil {
			t.Error("Expected ReviewedAt to be non-nil")
		} else if !response.ReviewedAt.Equal(reviewedAt) {
			t.Errorf("Expected reviewed at %v, got %v", reviewedAt, *response.ReviewedAt)
		}
		if response.ReviewedBy != "admin" {
			t.Errorf("Expected reviewed by 'admin', got %s", response.ReviewedBy)
		}
		if response.ApprovedRole == nil {
			t.Error("Expected ApprovedRole to be non-nil")
		} else if *response.ApprovedRole != approvedRole {
			t.Errorf("Expected approved role %s, got %s", approvedRole, *response.ApprovedRole)
		}
	})

	// rejected request
	t.Run("rejected account request", func(t *testing.T) {
		reviewedAt := time.Date(2025, 6, 27, 11, 0, 0, 0, time.UTC)
		rejectReason := "Invalid credentials provided"

		request := &AccountRequest{
			ID:            "req-789",
			Username:      "invaliduser",
			RequestedRole: RoleUser,
			Status:        AccountRequestRejected,
			CreatedAt:     time.Date(2025, 6, 27, 10, 0, 0, 0, time.UTC),
			ReviewedAt:    &reviewedAt,
			ReviewedBy:    "admin",
			RejectReason:  rejectReason,
		}

		response := request.ToResponse()

		if response.Status != AccountRequestRejected {
			t.Errorf("Expected status %s, got %s", AccountRequestRejected, response.Status)
		}
		if response.RejectReason != rejectReason {
			t.Errorf("Expected reject reason '%s', got '%s'", rejectReason, response.RejectReason)
		}
	})
}

func TestAccountRequest_IsReviewed(t *testing.T) {
	tests := []struct {
		name     string
		status   AccountRequestStatus
		expected bool
	}{
		{
			name:     "pending request is not reviewed",
			status:   AccountRequestPending,
			expected: false,
		},
		{
			name:     "approved request is reviewed",
			status:   AccountRequestApproved,
			expected: true,
		},
		{
			name:     "rejected request is reviewed",
			status:   AccountRequestRejected,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &AccountRequest{
				Status: tt.status,
			}

			result := request.IsReviewed()
			if result != tt.expected {
				t.Errorf("Expected IsReviewed() to return %v for status %s, got %v",
					tt.expected, tt.status, result)
			}
		})
	}
}

func TestAccountRequest_CanBeReviewed(t *testing.T) {
	tests := []struct {
		name     string
		status   AccountRequestStatus
		expected bool
	}{
		{
			name:     "pending request can be reviewed",
			status:   AccountRequestPending,
			expected: true,
		},
		{
			name:     "approved request cannot be reviewed again",
			status:   AccountRequestApproved,
			expected: false,
		},
		{
			name:     "rejected request cannot be reviewed again",
			status:   AccountRequestRejected,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &AccountRequest{
				Status: tt.status,
			}

			result := request.CanBeReviewed()
			if result != tt.expected {
				t.Errorf("Expected CanBeReviewed() to return %v for status %s, got %v",
					tt.expected, tt.status, result)
			}
		})
	}
}

func TestAccountRequestStatus_Constants(t *testing.T) {
	// tests that status constants have expected values
	expectedStatuses := map[AccountRequestStatus]string{
		AccountRequestPending:  "pending",
		AccountRequestApproved: "approved",
		AccountRequestRejected: "rejected",
	}

	for status, expectedValue := range expectedStatuses {
		if string(status) != expectedValue {
			t.Errorf("Expected status %s to have value '%s', got '%s'",
				status, expectedValue, string(status))
		}
	}
}

func TestAccountRequest_StatusTransitions(t *testing.T) {
	// typical workflow: create pending -> approve/reject
	t.Run("typical approval workflow", func(t *testing.T) {
		request := &AccountRequest{
			ID:            "req-workflow-1",
			Username:      "workflowuser",
			RequestedRole: RoleUser,
			Status:        AccountRequestPending,
			CreatedAt:     time.Now(),
		}

		// initially should be reviewable but not reviewed
		if !request.CanBeReviewed() {
			t.Error("New pending request should be reviewable")
		}
		if request.IsReviewed() {
			t.Error("New pending request should not be reviewed")
		}

		// approval
		reviewTime := time.Now()
		approvedRole := RoleUser
		request.Status = AccountRequestApproved
		request.ReviewedAt = &reviewTime
		request.ReviewedBy = "admin"
		request.ApprovedRole = &approvedRole

		// after approval should be reviewed but not reviewable
		if request.CanBeReviewed() {
			t.Error("Approved request should not be reviewable")
		}
		if !request.IsReviewed() {
			t.Error("Approved request should be reviewed")
		}
	})

	t.Run("typical rejection workflow", func(t *testing.T) {
		request := &AccountRequest{
			ID:            "req-workflow-2",
			Username:      "rejecteduser",
			RequestedRole: RoleAdmin,
			Status:        AccountRequestPending,
			CreatedAt:     time.Now(),
		}

		// rejection
		reviewTime := time.Now()
		request.Status = AccountRequestRejected
		request.ReviewedAt = &reviewTime
		request.ReviewedBy = "admin"
		request.RejectReason = "Insufficient justification for admin role"

		// after rejection should be reviewed but not reviewable
		if request.CanBeReviewed() {
			t.Error("Rejected request should not be reviewable")
		}
		if !request.IsReviewed() {
			t.Error("Rejected request should be reviewed")
		}
	})
}
