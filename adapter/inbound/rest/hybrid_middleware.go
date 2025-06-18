package rest

import (
	"net/http"

	"github.com/ajkula/GoRTMS/config"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

type HybridMiddleware struct {
	hmacMiddleware *HMACMiddleware
	jwtMiddleware  *AuthMiddleware
	logger         outbound.Logger
	config         *config.Config
}

func NewHybridMiddleware(
	config *config.Config,
	hmacMiddleware *HMACMiddleware,
	jwtMiddleware *AuthMiddleware,
	logger outbound.Logger,
) *HybridMiddleware {
	return &HybridMiddleware{
		hmacMiddleware: hmacMiddleware,
		jwtMiddleware:  jwtMiddleware,
		logger:         logger,
		config:         config,
	}
}

// intelligently routes to HMAC or JWT based on request headers
func (h *HybridMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.config.Security.EnableAuthentication {
			next.ServeHTTP(w, r)
			return
		}

		// Check if this is an HMAC request
		if h.isHMACRequest(r) {
			h.logger.Debug("Routing to HMAC middleware", "path", r.URL.Path, "method", r.Method)
			h.hmacMiddleware.Middleware(next).ServeHTTP(w, r)
			return
		}

		// Default to JWT middleware
		h.logger.Debug("Routing to JWT middleware", "path", r.URL.Path, "method", r.Method)
		h.jwtMiddleware.Middleware(next).ServeHTTP(w, r)
	})
}

// determines if the request contains HMAC authentication headers
func (h *HybridMiddleware) isHMACRequest(r *http.Request) bool {
	// Check for the presence of HMAC headers
	serviceID := r.Header.Get("X-Service-ID")
	timestamp := r.Header.Get("X-Timestamp")
	signature := r.Header.Get("X-Signature")

	// All three headers must be present for HMAC authentication
	hasAllHeaders := serviceID != "" && timestamp != "" && signature != ""

	if hasAllHeaders {
		h.logger.Debug("HMAC headers detected",
			"serviceID", serviceID,
			"hasTimestamp", timestamp != "",
			"hasSignature", signature != "")
	}

	return hasAllHeaders
}

// returns the authentication method used for the request
func (h *HybridMiddleware) GetAuthenticationMethod(r *http.Request) string {
	if h.isHMACRequest(r) {
		return "HMAC"
	}
	return "JWT"
}

// returns whether HMAC middleware is enabled
func (h *HybridMiddleware) IsHMACEnabled() bool {
	return h.hmacMiddleware != nil && h.config.Security.EnableAuthentication
}

// returns whether JWT middleware is enabled
func (h *HybridMiddleware) IsJWTEnabled() bool {
	return h.jwtMiddleware != nil && h.config.Security.EnableAuthentication
}

// updates the enabled status from underlying middlewares
func (h *HybridMiddleware) UpdateConfig(cfg *config.Config) {
	if h.hmacMiddleware != nil {
		h.hmacMiddleware.UpdateConfig(cfg)
	}
	if h.jwtMiddleware != nil {
		h.jwtMiddleware.UpdateConfig(cfg)
	}
}
