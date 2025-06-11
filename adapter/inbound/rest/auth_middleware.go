package rest

import (
	"context"
	"net/http"
	"strings"

	"github.com/ajkula/GoRTMS/config"
	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/inbound"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

type contextKey string

const UserContextKey contextKey = "user"

type AuthMiddleware struct {
	authService inbound.AuthService
	logger      outbound.Logger
	enabled     bool
}

func NewAuthMiddleware(authService inbound.AuthService, logger outbound.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		authService: authService,
		logger:      logger,
		enabled:     config.DefaultConfig().Security.EnableAuthentication,
	}
}

func (m *AuthMiddleware) RefreshEnabled() {
	m.enabled = config.DefaultConfig().Security.EnableAuthentication
}

func (m *AuthMiddleware) SetEnabled(enabled bool) {
	m.enabled = enabled
}

func (m *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.enabled {
			next.ServeHTTP(w, r)
			return
		}

		if m.isPublicRoute(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		token := m.extractToken(r)
		if token == "" {
			m.unauthorized(w, "missing token")
			return
		}

		user, err := m.authService.ValidateToken(token)
		if err != nil {
			m.unauthorized(w, err.Error())
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *AuthMiddleware) RequireRole(role model.UserRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !m.enabled {
				next.ServeHTTP(w, r)
				return
			}

			user := m.GetUserFromContext(r.Context())
			if user == nil {
				m.forbidden(w, "user not found in context")
				return
			}

			if user.Role != role && user.Role != model.RoleAdmin {
				m.forbidden(w, "insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (m *AuthMiddleware) GetUserFromContext(ctx context.Context) *model.User {
	if user, ok := ctx.Value(UserContextKey).(*model.User); ok {
		return user
	}
	return nil
}

func (m *AuthMiddleware) isPublicRoute(path string) bool {
	publicRoutes := []string{
		"/api/auth/login",
		"/api/auth/bootstrap",
		"/api/health",
	}

	for _, route := range publicRoutes {
		if strings.HasPrefix(path, route) {
			return true
		}
	}

	if strings.HasPrefix(path, "/web/") || path == "/" {
		return true
	}

	return false
}

func (m *AuthMiddleware) extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}

	return parts[1]
}

func (m *AuthMiddleware) unauthorized(w http.ResponseWriter, message string) {
	m.logger.Warn("Unauthorized access", "message", message)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"unauthorized","message":"` + message + `"}`))
}

func (m *AuthMiddleware) forbidden(w http.ResponseWriter, message string) {
	m.logger.Warn("Forbidden access", "message", message)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(`{"error":"forbidden","message":"` + message + `"}`))
}
