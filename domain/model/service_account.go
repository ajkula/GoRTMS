package model

import (
	"strings"
	"time"
)

// represents a service account for HMAC authentication
type ServiceAccount struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Secret      string    `json:"-"`
	IsDisclosed bool      `json:"isDisclosed"`
	Permissions []string  `json:"permissions"`
	IPWhitelist []string  `json:"ipWhitelist,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	LastUsed    time.Time `json:"lastUsed"`
	Enabled     bool      `json:"enabled"`
}

// checks if service has specific permission
func (s *ServiceAccount) HasPermission(permission string) bool {
	for _, p := range s.Permissions {
		if p == permission || p == "*" {
			return true
		}

		// Support wildcard patterns like "publish:*" (action wildcard)
		if strings.HasSuffix(p, ":*") {
			prefix := strings.TrimSuffix(p, "*")
			if strings.HasPrefix(permission, prefix) {
				return true
			}
		}

		// Support wildcard patterns like "*:tasks" (domain wildcard)
		if strings.HasPrefix(p, "*:") {
			suffix := strings.TrimPrefix(p, "*:")
			if strings.HasSuffix(permission, ":"+suffix) {
				return true
			}
		}
	}
	return false
}

// returns a view of the service account safe for API responses
func (s *ServiceAccount) ToPublicView() *ServiceAccountView {
	view := &ServiceAccountView{
		ID:          s.ID,
		Name:        s.Name,
		IsDisclosed: s.IsDisclosed,
		Permissions: s.Permissions,
		IPWhitelist: s.IPWhitelist,
		CreatedAt:   s.CreatedAt,
		LastUsed:    s.LastUsed,
		Enabled:     s.Enabled,
	}

	// Mask secret if already disclosed
	if s.IsDisclosed {
		view.Secret = "••••••••••••••••"
	}

	return view
}

// represents the public view of a service account
type ServiceAccountView struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Secret      string    `json:"secret,omitempty"`
	IsDisclosed bool      `json:"isDisclosed"`
	Permissions []string  `json:"permissions"`
	IPWhitelist []string  `json:"ipWhitelist,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	LastUsed    time.Time `json:"lastUsed"`
	Enabled     bool      `json:"enabled"`
}

// represents a request to create a service account
type ServiceAccountCreateRequest struct {
	Name        string   `json:"name" validate:"required,min=3,max=50"`
	Permissions []string `json:"permissions" validate:"required,min=1"`
	IPWhitelist []string `json:"ipWhitelist,omitempty"`
}

// represents a request to update service permissions
type ServiceAccountUpdateRequest struct {
	Permissions []string `json:"permissions" validate:"required,min=1"`
	IPWhitelist []string `json:"ipWhitelist,omitempty"`
	Enabled     *bool    `json:"enabled,omitempty"`
}
