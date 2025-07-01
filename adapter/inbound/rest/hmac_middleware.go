package rest

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ajkula/GoRTMS/config"
	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

const ServiceContextKey contextKey = "service"

type HMACMiddleware struct {
	serviceRepo     outbound.ServiceRepository
	logger          outbound.Logger
	config          *config.Config
	timestampWindow time.Duration
}

func NewHMACMiddleware(serviceRepo outbound.ServiceRepository, logger outbound.Logger, config *config.Config) *HMACMiddleware {
	timestampWindow := 5 * time.Minute

	if config.Security.HMAC.TimestampWindow != "" {
		if duration, err := time.ParseDuration(config.Security.HMAC.TimestampWindow); err == nil {
			timestampWindow = duration
		}
	}

	return &HMACMiddleware{
		serviceRepo:     serviceRepo,
		logger:          logger,
		config:          config,
		timestampWindow: timestampWindow,
	}
}

// updates the enabled status from config
func (m *HMACMiddleware) UpdateConfig(config *config.Config) {
	m.config = config
}

// // manually sets the enabled status
// func (m *HMACMiddleware) SetEnabled(enabled bool) {
// 	m.enabled = enabled
// }

// validates HMAC authentication
func (m *HMACMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.config.Security.EnableAuthentication {
			next.ServeHTTP(w, r)
			return
		}

		// Extract HMAC headers
		serviceID := r.Header.Get("X-Service-ID")
		timestamp := r.Header.Get("X-Timestamp")
		signature := r.Header.Get("X-Signature")

		if serviceID == "" || timestamp == "" || signature == "" {
			m.unauthorized(w, "missing HMAC headers")
			return
		}

		// Check RequireTLS guard
		if m.config.Security.HMAC.RequireTLS && r.TLS == nil {
			// Log explicit message for admin
			m.logger.Warn("HMAC request rejected: RequireTLS enabled but request over HTTP",
				"path", r.URL.Path,
				"method", r.Method,
				"remoteAddr", r.RemoteAddr,
				"serviceID", serviceID)

			// Return 404 security by obscurity
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("404 page not found"))
			return
		}

		// Validate timestamp window
		if !m.isTimestampValid(timestamp) {
			m.unauthorized(w, "timestamp outside valid window")
			return
		}

		// Get service account
		service, err := m.serviceRepo.GetByID(r.Context(), serviceID)
		if err != nil {
			m.logger.Warn("Service not found", "serviceID", serviceID, "error", err)
			m.unauthorized(w, "invalid service")
			return
		}

		if !service.Enabled {
			m.unauthorized(w, "service disabled")
			return
		}

		// Read request body for signature validation
		body, err := m.readBody(r)
		if err != nil {
			m.logger.Error("Failed to read request body", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Validate HMAC signature
		if !m.validateSignature(r.Method, r.URL.Path, body, timestamp, service.Secret, signature) {
			m.unauthorized(w, "invalid signature")
			return
		}

		// Check IP whitelist if configured
		if len(service.IPWhitelist) > 0 && !m.isIPAllowed(r.RemoteAddr, service.IPWhitelist) {
			m.forbidden(w, "IP not whitelisted")
			return
		}

		// Check permissions for the specific action
		permission := m.extractPermission(r.Method, r.URL.Path)
		if permission != "" && !service.HasPermission(permission) {
			m.forbidden(w, fmt.Sprintf("insufficient permissions for %s", permission))
			return
		}

		// Update last used timestamp (async to avoid blocking)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			m.serviceRepo.UpdateLastUsed(ctx, serviceID)
		}()

		// Add service to context
		ctx := context.WithValue(r.Context(), ServiceContextKey, service)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// reads and restores the request body
func (m *HMACMiddleware) readBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return []byte{}, nil
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	// Restore body for next handlers
	r.Body = io.NopCloser(bytes.NewBuffer(body))
	return body, nil
}

// checks if timestamp is within the allowed window
func (m *HMACMiddleware) isTimestampValid(timestampStr string) bool {
	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		m.logger.Warn("Invalid timestamp format", "timestamp", timestampStr, "error", err)
		return false
	}

	now := time.Now()
	diff := now.Sub(timestamp).Abs()

	if diff > m.timestampWindow {
		m.logger.Warn("Timestamp outside window",
			"timestamp", timestamp,
			"now", now,
			"diff", diff,
			"window", m.timestampWindow)
		return false
	}

	return true
}

// validates the HMAC signature
func (m *HMACMiddleware) validateSignature(method, path string, body []byte, timestamp, secret, providedSignature string) bool {
	expectedSignature := m.generateSignature(method, path, body, timestamp, secret)

	// Use constant-time comparison to prevent timing attacks
	return hmac.Equal([]byte(expectedSignature), []byte(providedSignature))
}

// creates HMAC-SHA256 signature
func (m *HMACMiddleware) generateSignature(method, path string, body []byte, timestamp, secret string) string {
	// Create canonical request string
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s", method, path, string(body), timestamp)

	// Generate HMAC-SHA256
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(canonicalRequest))
	signature := hex.EncodeToString(h.Sum(nil))

	return fmt.Sprintf("sha256=%s", signature)
}

// determines required permission based on HTTP method and path
func (m *HMACMiddleware) extractPermission(method, path string) string {
	// Parse path to extract domain and operation
	parts := strings.Split(strings.Trim(path, "/"), "/")

	// Expected format: api/domains/{domain}/queues/{queue}/messages
	if len(parts) >= 5 && parts[0] == "api" && parts[1] == "domains" && parts[3] == "queues" {
		domain := parts[2]

		switch method {
		case "POST":
			if strings.HasSuffix(path, "/messages") {
				return fmt.Sprintf("publish:%s", domain)
			}
			if strings.Contains(path, "/consumers") {
				return fmt.Sprintf("manage:%s", domain)
			}
		case "GET":
			if strings.HasSuffix(path, "/messages") {
				return fmt.Sprintf("consume:%s", domain)
			}
		case "DELETE":
			if strings.Contains(path, "/consumers") {
				return fmt.Sprintf("manage:%s", domain)
			}
		}
	}

	return ""
}

// checks if the client IP is in the whitelist
func (m *HMACMiddleware) isIPAllowed(remoteAddr string, whitelist []string) bool {
	// Extract IP from "IP:port" format
	clientIP := remoteAddr
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		clientIP = remoteAddr[:idx]
	}

	// Remove brackets from IPv6 addresses
	clientIP = strings.Trim(clientIP, "[]")

	for _, allowedIP := range whitelist {
		// Simple IP matching (could be enhanced with CIDR support)
		if clientIP == allowedIP || allowedIP == "*" {
			return true
		}

		// Basic wildcard support
		if strings.HasSuffix(allowedIP, "*") {
			prefix := strings.TrimSuffix(allowedIP, "*")
			if strings.HasPrefix(clientIP, prefix) {
				return true
			}
		}
	}

	return false
}

// extracts service from request context
func (m *HMACMiddleware) GetServiceFromContext(ctx context.Context) *model.ServiceAccount {
	service, ok := ctx.Value(ServiceContextKey).(*model.ServiceAccount)
	if ok {
		return service
	}
	return nil
}

// sends 401 response
func (m *HMACMiddleware) unauthorized(w http.ResponseWriter, message string) {
	m.logger.Warn("HMAC authentication failed", "message", message)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(fmt.Sprintf(`{"error":"unauthorized","message":"%s"}`, message)))
}

// sends 403 response
func (m *HMACMiddleware) forbidden(w http.ResponseWriter, message string) {
	m.logger.Warn("HMAC authorization failed", "message", message)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(fmt.Sprintf(`{"error":"forbidden","message":"%s"}`, message)))
}
