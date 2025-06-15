package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ajkula/GoRTMS/adapter/outbound/storage"
	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
	"github.com/gorilla/mux"
)

// ServiceHandler handles service account management operations
type ServiceHandler struct {
	serviceRepo outbound.ServiceRepository
	logger      outbound.Logger
}

// NewServiceHandler creates a new service handler
func NewServiceHandler(serviceRepo outbound.ServiceRepository, logger outbound.Logger) *ServiceHandler {
	return &ServiceHandler{
		serviceRepo: serviceRepo,
		logger:      logger,
	}
}

// CreateService creates a new service account with secret disclosed once
func (h *ServiceHandler) CreateService(w http.ResponseWriter, r *http.Request) {
	var req model.ServiceAccountCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if err := h.validateCreateRequest(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Generate unique service ID
	serviceID := h.generateServiceID(req.Name)

	// Create service account
	service := &model.ServiceAccount{
		ID:          serviceID,
		Name:        req.Name,
		Secret:      storage.GenerateServiceSecret(),
		IsDisclosed: false, // Initially not disclosed
		Permissions: req.Permissions,
		IPWhitelist: req.IPWhitelist,
		CreatedAt:   time.Now(),
		LastUsed:    time.Time{}, // Never used yet
		Enabled:     true,
	}

	// Save to repository
	if err := h.serviceRepo.Create(r.Context(), service); err != nil {
		h.logger.Error("Failed to create service account", "error", err, "serviceID", serviceID)
		http.Error(w, "Failed to create service account", http.StatusInternalServerError)
		return
	}

	// Mark as disclosed and update
	service.IsDisclosed = true
	if err := h.serviceRepo.Update(r.Context(), service); err != nil {
		h.logger.Error("Failed to mark service as disclosed", "error", err, "serviceID", serviceID)
		// Continue anyway - service is created
	}

	h.logger.Info("Service account created", "serviceID", serviceID, "name", req.Name)

	// Prepare response with secret visible (ONLY TIME)
	response := struct {
		*model.ServiceAccountView
		Message string `json:"message"`
	}{
		ServiceAccountView: &model.ServiceAccountView{
			ID:          service.ID,
			Name:        service.Name,
			Secret:      service.Secret, // ✅ VISIBLE ONLY HERE
			IsDisclosed: true,
			Permissions: service.Permissions,
			IPWhitelist: service.IPWhitelist,
			CreatedAt:   service.CreatedAt,
			LastUsed:    service.LastUsed,
			Enabled:     service.Enabled,
		},
		Message: "SAVE THIS SECRET NOW - It will never be shown again!",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// ListServices returns all service accounts (with secrets masked)
func (h *ServiceHandler) ListServices(w http.ResponseWriter, r *http.Request) {
	services, err := h.serviceRepo.List(r.Context())
	if err != nil {
		h.logger.Error("Failed to list service accounts", "error", err)
		http.Error(w, "Failed to retrieve service accounts", http.StatusInternalServerError)
		return
	}

	// Convert to public views (secrets masked)
	views := make([]*model.ServiceAccountView, len(services))
	for i, service := range services {
		views[i] = service.ToPublicView()
	}

	response := struct {
		Services []*model.ServiceAccountView `json:"services"`
		Count    int                         `json:"count"`
	}{
		Services: views,
		Count:    len(views),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetService returns a specific service account (with secret masked)
func (h *ServiceHandler) GetService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serviceID := vars["id"]

	service, err := h.serviceRepo.GetByID(r.Context(), serviceID)
	if err != nil {
		h.logger.Warn("Service not found", "serviceID", serviceID, "error", err)
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	// Return public view (secret masked if disclosed)
	view := service.ToPublicView()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(view)
}

// DeleteService removes a service account
func (h *ServiceHandler) DeleteService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serviceID := vars["id"]

	// Check if service exists
	_, err := h.serviceRepo.GetByID(r.Context(), serviceID)
	if err != nil {
		h.logger.Warn("Service not found for deletion", "serviceID", serviceID, "error", err)
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	// Delete service
	if err := h.serviceRepo.Delete(r.Context(), serviceID); err != nil {
		h.logger.Error("Failed to delete service account", "error", err, "serviceID", serviceID)
		http.Error(w, "Failed to delete service account", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Service account deleted", "serviceID", serviceID)

	response := struct {
		Message   string `json:"message"`
		ServiceID string `json:"serviceId"`
	}{
		Message:   "Service account deleted successfully",
		ServiceID: serviceID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// RotateSecret generates a new secret for a service account
func (h *ServiceHandler) RotateSecret(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serviceID := vars["id"]

	// Get existing service
	service, err := h.serviceRepo.GetByID(r.Context(), serviceID)
	if err != nil {
		h.logger.Warn("Service not found for secret rotation", "serviceID", serviceID, "error", err)
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	// Generate new secret
	service.Secret = storage.GenerateServiceSecret()
	service.IsDisclosed = false // Reset disclosure status

	// Update service
	if err := h.serviceRepo.Update(r.Context(), service); err != nil {
		h.logger.Error("Failed to rotate service secret", "error", err, "serviceID", serviceID)
		http.Error(w, "Failed to rotate secret", http.StatusInternalServerError)
		return
	}

	// Mark as disclosed
	service.IsDisclosed = true
	if err := h.serviceRepo.Update(r.Context(), service); err != nil {
		h.logger.Error("Failed to mark rotated secret as disclosed", "error", err, "serviceID", serviceID)
		// Continue anyway
	}

	h.logger.Info("Service secret rotated", "serviceID", serviceID)

	// Prepare response with new secret visible (ONLY TIME)
	response := struct {
		*model.ServiceAccountView
		Message string `json:"message"`
		Rotated bool   `json:"rotated"`
	}{
		ServiceAccountView: &model.ServiceAccountView{
			ID:          service.ID,
			Name:        service.Name,
			Secret:      service.Secret, // ✅ NEW SECRET VISIBLE ONLY HERE
			IsDisclosed: true,
			Permissions: service.Permissions,
			IPWhitelist: service.IPWhitelist,
			CreatedAt:   service.CreatedAt,
			LastUsed:    service.LastUsed,
			Enabled:     service.Enabled,
		},
		Message: "NEW SECRET GENERATED - Save it now! Old secret is invalid.",
		Rotated: true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdatePermissions updates service account permissions and settings
func (h *ServiceHandler) UpdatePermissions(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serviceID := vars["id"]

	var req model.ServiceAccountUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get existing service
	service, err := h.serviceRepo.GetByID(r.Context(), serviceID)
	if err != nil {
		h.logger.Warn("Service not found for permission update", "serviceID", serviceID, "error", err)
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	// Update fields
	service.Permissions = req.Permissions
	service.IPWhitelist = req.IPWhitelist
	if req.Enabled != nil {
		service.Enabled = *req.Enabled
	}

	// Save changes
	if err := h.serviceRepo.Update(r.Context(), service); err != nil {
		h.logger.Error("Failed to update service permissions", "error", err, "serviceID", serviceID)
		http.Error(w, "Failed to update service", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Service permissions updated", "serviceID", serviceID, "permissions", service.Permissions)

	// Return updated view (secret masked)
	view := service.ToPublicView()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(view)
}

// validateCreateRequest validates service creation request
func (h *ServiceHandler) validateCreateRequest(req *model.ServiceAccountCreateRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("service name is required")
	}

	if len(req.Name) < 3 || len(req.Name) > 50 {
		return fmt.Errorf("service name must be between 3 and 50 characters")
	}

	if len(req.Permissions) == 0 {
		return fmt.Errorf("at least one permission is required")
	}

	// Validate permission format
	for _, perm := range req.Permissions {
		if !h.isValidPermission(perm) {
			return fmt.Errorf("invalid permission format: %s", perm)
		}
	}

	return nil
}

// isValidPermission validates permission format
func (h *ServiceHandler) isValidPermission(permission string) bool {
	// Allow global wildcard
	if permission == "*" {
		return true
	}

	// Allow action:domain or action:*
	parts := strings.Split(permission, ":")
	if len(parts) != 2 {
		return false
	}

	action := parts[0]
	domain := parts[1]

	// Valid actions
	validActions := []string{"publish", "consume", "manage"}
	isValidAction := false
	for _, validAction := range validActions {
		if action == validAction {
			isValidAction = true
			break
		}
	}

	if !isValidAction {
		return false
	}

	// Domain can be * or alphanumeric with hyphens
	if domain == "*" {
		return true
	}

	// Simple domain name validation
	if len(domain) < 1 || len(domain) > 50 {
		return false
	}

	return true
}

// generateServiceID creates a unique service ID
func (h *ServiceHandler) generateServiceID(name string) string {
	// Clean name and create base ID
	cleaned := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	cleaned = strings.Trim(cleaned, "-")

	// Add timestamp suffix for uniqueness
	timestamp := time.Now().Format("060102-150405")

	return fmt.Sprintf("%s-%s", cleaned, timestamp)
}
