package rest

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/inbound"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

type AccountRequestHandler struct {
	accountRequestService inbound.AccountRequestService
	authService           inbound.AuthService
	logger                outbound.Logger
}

type CreateAccountRequestRequest struct {
	Username      string         `json:"username"`
	Password      string         `json:"password"`
	RequestedRole model.UserRole `json:"requestedRole"`
}

type ReviewAccountRequestRequest struct {
	Approve      bool            `json:"approve"`
	ApprovedRole *model.UserRole `json:"approvedRole,omitempty"`
	RejectReason string          `json:"rejectReason,omitempty"`
}

type AccountRequestApiResponse struct {
	Request *model.AccountRequestResponse `json:"request"`
	Message string                        `json:"message,omitempty"`
}

type AccountRequestListResponse struct {
	Requests []*model.AccountRequestResponse `json:"requests"`
	Count    int                             `json:"count"`
}

func NewAccountRequestHandler(
	accountRequestService inbound.AccountRequestService,
	authService inbound.AuthService,
	logger outbound.Logger,
) *AccountRequestHandler {
	return &AccountRequestHandler{
		accountRequestService: accountRequestService,
		authService:           authService,
		logger:                logger,
	}
}

// handles account request creation (public endpoint)
func (h *AccountRequestHandler) CreateAccountRequest(w http.ResponseWriter, r *http.Request) {
	var req CreateAccountRequestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode account request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// required fields
	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	// default role if not specified
	if req.RequestedRole == "" {
		req.RequestedRole = model.RoleUser
	}

	if req.RequestedRole != model.RoleUser && req.RequestedRole != model.RoleAdmin {
		http.Error(w, "Invalid role requested", http.StatusBadRequest)
		return
	}

	options := &inbound.CreateAccountRequestOptions{
		Username:      req.Username,
		Password:      req.Password,
		RequestedRole: req.RequestedRole,
	}

	request, err := h.accountRequestService.CreateAccountRequest(r.Context(), options)
	if err != nil {
		h.logger.Error("Failed to create account request", "error", err, "username", req.Username)

		switch err {
		case model.ErrUsernameAlreadyTaken:
			http.Error(w, "Username is already taken", http.StatusConflict)
		case model.ErrAccountRequestAlreadyExists:
			http.Error(w, "Account request already exists for this username", http.StatusConflict)
		case model.ErrInvalidRequestedRole:
			http.Error(w, "Invalid role requested", http.StatusBadRequest)
		default:
			http.Error(w, "Failed to create account request", http.StatusInternalServerError)
		}
		return
	}

	h.logger.Info("Account request created", "requestID", request.ID, "username", request.Username)

	response := AccountRequestApiResponse{
		Request: request.ToResponse(),
		Message: "Account request submitted successfully. An admin will review your request.",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// lists all account requests (admin only)
func (h *AccountRequestHandler) ListAccountRequests(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	statusParam := query.Get("status")

	var statusFilter *model.AccountRequestStatus
	if statusParam != "" {
		status := model.AccountRequestStatus(statusParam)
		// validate status
		if status != model.AccountRequestPending &&
			status != model.AccountRequestApproved &&
			status != model.AccountRequestRejected {
			http.Error(w, "Invalid status filter", http.StatusBadRequest)
			return
		}
		statusFilter = &status
	}

	requests, err := h.accountRequestService.ListAccountRequests(r.Context(), statusFilter)
	if err != nil {
		h.logger.Error("Failed to list account requests", "error", err)
		http.Error(w, "Failed to retrieve account requests", http.StatusInternalServerError)
		return
	}

	responses := make([]*model.AccountRequestResponse, len(requests))
	for i, req := range requests {
		responses[i] = req.ToResponse()
	}

	response := AccountRequestListResponse{
		Requests: responses,
		Count:    len(responses),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// retrieves a specific account request (admin only)
func (h *AccountRequestHandler) GetAccountRequest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	requestID := vars["requestId"]

	if requestID == "" {
		http.Error(w, "Request ID is required", http.StatusBadRequest)
		return
	}

	request, err := h.accountRequestService.GetAccountRequest(r.Context(), requestID)
	if err != nil {
		if err == model.ErrAccountRequestNotFound {
			http.Error(w, "Account request not found", http.StatusNotFound)
		} else {
			h.logger.Error("Failed to get account request", "error", err, "requestID", requestID)
			http.Error(w, "Failed to retrieve account request", http.StatusInternalServerError)
		}
		return
	}

	response := AccountRequestApiResponse{
		Request: request.ToResponse(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// approves or rejects an account request (admin only)
func (h *AccountRequestHandler) ReviewAccountRequest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	requestID := vars["requestId"]

	if requestID == "" {
		http.Error(w, "Request ID is required", http.StatusBadRequest)
		return
	}

	var req ReviewAccountRequestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode review request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if !req.Approve && req.RejectReason == "" {
		http.Error(w, "Reject reason is required when rejecting a request", http.StatusBadRequest)
		return
	}

	// Get reviewer from context
	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// review options
	options := &inbound.ReviewAccountRequestOptions{
		Approve:      req.Approve,
		ApprovedRole: req.ApprovedRole,
		RejectReason: req.RejectReason,
		ReviewedBy:   user.Username,
	}

	reviewedRequest, err := h.accountRequestService.ReviewAccountRequest(r.Context(), requestID, options)
	if err != nil {
		h.logger.Error("Failed to review account request", "error", err, "requestID", requestID)

		switch err {
		case model.ErrAccountRequestNotFound:
			http.Error(w, "Account request not found", http.StatusNotFound)
		case model.ErrAccountRequestAlreadyReviewed:
			http.Error(w, "Account request has already been reviewed", http.StatusConflict)
		default:
			http.Error(w, "Failed to review account request", http.StatusInternalServerError)
		}
		return
	}

	var message string
	if req.Approve {
		message = "Account request approved successfully. User account has been created."
	} else {
		message = "Account request rejected successfully."
	}

	h.logger.Info("Account request reviewed",
		"requestID", requestID,
		"approved", req.Approve,
		"reviewedBy", user.Username)

	response := AccountRequestApiResponse{
		Request: reviewedRequest.ToResponse(),
		Message: message,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// deletes an account request (admin only)
func (h *AccountRequestHandler) DeleteAccountRequest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	requestID := vars["requestId"]

	if requestID == "" {
		http.Error(w, "Request ID is required", http.StatusBadRequest)
		return
	}

	err := h.accountRequestService.DeleteAccountRequest(r.Context(), requestID)
	if err != nil {
		if err == model.ErrAccountRequestNotFound {
			http.Error(w, "Account request not found", http.StatusNotFound)
		} else {
			h.logger.Error("Failed to delete account request", "error", err, "requestID", requestID)
			http.Error(w, "Failed to delete account request", http.StatusInternalServerError)
		}
		return
	}

	h.logger.Info("Account request deleted", "requestID", requestID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Account request deleted successfully",
	})
}
