package rest

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/inbound"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

type AuthHandler struct {
	authService inbound.AuthService
	logger      outbound.Logger
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	User  *model.UserResponse `json:"user"`
	Token string              `json:"token"`
}

type CreateUserRequest struct {
	Username string         `json:"username"`
	Password string         `json:"password"`
	Role     model.UserRole `json:"role"`
}

type BootstrapResponse struct {
	Admin    *model.UserResponse `json:"admin"`
	Password string              `json:"password"`
	Message  string              `json:"message"`
}

func NewAuthHandler(authService inbound.AuthService, logger outbound.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		logger:      logger,
	}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode login request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password required", http.StatusBadRequest)
		return
	}

	user, token, err := h.authService.Login(req.Username, req.Password)
	if err != nil {
		h.logger.Warn("Login failed", "username", req.Username, "error", err)
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	userRes := user.ToResponse()
	h.logger.Info("User logged in", "userRes", userRes)

	response := LoginResponse{
		User:  userRes,
		Token: token,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *AuthHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode create user request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password required", http.StatusBadRequest)
		return
	}

	if req.Role == "" {
		req.Role = model.RoleUser
	}

	user, err := h.authService.CreateUser(req.Username, req.Password, req.Role)
	if err != nil {
		h.logger.Error("Failed to create user", "username", req.Username, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.logger.Info("User created", "username", user.Username, "role", user.Role)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user.ToResponse())
}

func (h *AuthHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.authService.ListUsers()

	if err != nil {
		h.logger.Error("Failed to list users", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	usersResponse := make([]*model.UserResponse, 0)
	for _, user := range users {
		usersResponse = append(usersResponse, user.ToResponse())
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(usersResponse)
}

func (h *AuthHandler) Bootstrap(w http.ResponseWriter, r *http.Request) {
	// check if users exist
	users, err := h.authService.ListUsers()
	if err != nil {
		h.logger.Error("Bootstrap check failed", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if len(users) > 0 {
		h.logger.Warn("Bootstrap attempted but users already exist",
			"Host", r.Host,
			"Body", r.Body,
			"Header", r.Header,
			"RequestURI", r.RequestURI,
		)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "bootstrap_not_needed",
			"message": "Users already exist. Bootstrap not needed.",
		})
		return
	}

	admin, password, err := h.authService.BootstrapAdmin()
	if err != nil {
		h.logger.Error("Bootstrap failed", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.logger.Info("Admin bootstrapped", "username", admin.Username)

	response := BootstrapResponse{
		Admin:    admin.ToResponse(),
		Password: password,
		Message:  "Admin account created. Save this password - it will not be shown again!",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *AuthHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}
	userResponse := user.ToResponse()
	h.logger.Warn("GetProfile", "user", userResponse, "Context", r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userResponse)
}

// extracts user from context
func GetUserFromContext(ctx context.Context) *model.User {
	user, ok := ctx.Value(UserContextKey).(*model.User)
	if ok {
		return user
	}
	return nil
}
