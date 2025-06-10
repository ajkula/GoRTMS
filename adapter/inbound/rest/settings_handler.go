package rest

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ajkula/GoRTMS/config"
)

type SettingsResponse struct {
	Config   *config.PublicConfig `json:"config"`
	FilePath string               `json:"filePath"`
	Message  string               `json:"message,omitempty"`
}

type SettingsUpdateRequest struct {
	Config        *config.PublicConfig `json:"config"`
	RestartNeeded bool                 `json:"restartNeeded,omitempty"`
}

var (
	globalConfigPath string
	configMutex      sync.RWMutex
)

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Getting current settings")

	// Get current config from global instance
	currentConfig := h.getCurrentConfig()
	publicConfig := currentConfig.ToPublic()

	response := SettingsResponse{
		Config:   publicConfig,
		FilePath: h.getConfigFilePath(),
		Message:  "Settings retrieved successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode settings response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Updating settings")

	var req SettingsUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode settings request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Config == nil {
		http.Error(w, "Config is required", http.StatusBadRequest)
		return
	}

	currentConfig := h.getCurrentConfig()
	newConfig := &config.Config{}
	*newConfig = *currentConfig
	newConfig.MergeFromPublic(req.Config)

	// Validate the configuration
	if err := h.validateConfigUpdate(newConfig); err != nil {
		h.logger.Error("Configuration validation failed", "error", err)
		http.Error(w, fmt.Sprintf("Invalid configuration: %v", err), http.StatusBadRequest)
		return
	}

	// Determine if restart is needed
	restartNeeded := h.requiresRestart(newConfig)

	configPath := h.getConfigFilePath()
	if err := config.SaveConfig(newConfig, configPath); err != nil {
		h.logger.Error("Failed to save configuration", "error", err)
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	// Update runtime configuration (no restart)
	if !restartNeeded {
		if err := h.updateRuntimeConfig(newConfig); err != nil {
			h.logger.Error("Failed to update runtime configuration", "error", err)
			h.logger.Warn("Configuration saved to file but runtime update failed")
		}
	}

	h.logger.Info("Settings updated successfully",
		"restart_needed", restartNeeded,
		"config_path", configPath)

	publicConfig := newConfig.ToPublic()
	response := SettingsResponse{
		Config:   publicConfig,
		FilePath: configPath,
		Message:  "Settings updated successfully",
	}

	// Add restart notice if needed
	if restartNeeded {
		response.Message = "Settings updated successfully. Server restart required for some changes to take effect."
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode settings response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) resetSettings(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Resetting settings to defaults")

	defaultConfig := config.DefaultConfig()

	configPath := h.getConfigFilePath()
	if err := config.SaveConfig(defaultConfig, configPath); err != nil {
		h.logger.Error("Failed to save default configuration", "error", err)
		http.Error(w, "Failed to reset configuration", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Settings reset to defaults", "config_path", configPath)

	publicResponse := defaultConfig.ToPublic()
	response := SettingsResponse{
		Config:   publicResponse,
		FilePath: configPath,
		Message:  "Settings reset to defaults. Server restart recommended.",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode reset response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// validates the configuration update
func (h *Handler) validateConfigUpdate(cfg *config.Config) error {
	// Use existing validation from config package
	if err := config.ValidateConfig(cfg); err != nil {
		return err
	}

	currentConfig := h.getCurrentConfig()

	// Check for port conflicts with currently running services
	if cfg.HTTP.Enabled && cfg.HTTP.Port != currentConfig.HTTP.Port {
		if h.isPortInUse(cfg.HTTP.Port) {
			return fmt.Errorf("HTTP port %d is already in use", cfg.HTTP.Port)
		}
	}

	if cfg.GRPC.Enabled && cfg.GRPC.Port != currentConfig.GRPC.Port {
		if h.isPortInUse(cfg.GRPC.Port) {
			return fmt.Errorf("gRPC port %d is already in use", cfg.GRPC.Port)
		}
	}

	// Validate paths exist
	if cfg.Storage.Engine == "file" {
		if err := h.validateStoragePath(cfg.Storage.Path); err != nil {
			return fmt.Errorf("storage path validation failed: %v", err)
		}
	}

	return nil
}

// determines if configuration changes require a server restart
func (h *Handler) requiresRestart(newConfig *config.Config) bool {
	currentConfig := h.getCurrentConfig()

	// Changes that require restart
	if newConfig.HTTP.Port != currentConfig.HTTP.Port ||
		newConfig.HTTP.Address != currentConfig.HTTP.Address ||
		newConfig.HTTP.TLS != currentConfig.HTTP.TLS ||
		newConfig.GRPC.Enabled != currentConfig.GRPC.Enabled ||
		newConfig.GRPC.Port != currentConfig.GRPC.Port ||
		newConfig.GRPC.Address != currentConfig.GRPC.Address ||
		newConfig.AMQP.Enabled != currentConfig.AMQP.Enabled ||
		newConfig.MQTT.Enabled != currentConfig.MQTT.Enabled ||
		newConfig.Storage.Engine != currentConfig.Storage.Engine ||
		newConfig.Cluster.Enabled != currentConfig.Cluster.Enabled {
		return true
	}

	return false
}

// updates configuration that can be changed at runtime
func (h *Handler) updateRuntimeConfig(newConfig *config.Config) error {
	// Update log level
	if err := h.updateLogLevel(newConfig.General.LogLevel); err != nil {
		return fmt.Errorf("failed to update log level: %v", err)
	}

	// Update CORS settings
	if newConfig.HTTP.CORS.Enabled != h.getCurrentConfig().HTTP.CORS.Enabled {
		if err := h.updateCORSSettings(newConfig); err != nil {
			return fmt.Errorf("failed to update CORS settings: %v", err)
		}
	}

	// Update storage retention
	if newConfig.Storage.RetentionDays != h.getCurrentConfig().Storage.RetentionDays {
		h.updateStorageRetention(newConfig.Storage.RetentionDays)
	}

	// Store the new config globally
	h.setCurrentConfig(newConfig)

	return nil
}

// Helper methods

func (h *Handler) getCurrentConfig() *config.Config {
	// Return current config from handler
	return h.config
}

func (h *Handler) getConfigFilePath() string {
	configMutex.RLock()
	defer configMutex.RUnlock()

	if globalConfigPath != "" {
		return globalConfigPath
	}

	// Fallback to default
	return "./config.yaml"
}

// Updates the configuration instance
func (h *Handler) setCurrentConfig(cfg *config.Config) {
	h.config = cfg

	h.logger.Info("Configuration updated in handler")
}

// SetGlobalConfigPath sets the config file path
func SetGlobalConfigPath(path string) {
	configMutex.Lock()
	defer configMutex.Unlock()
	globalConfigPath = path
}

func (h *Handler) isPortInUse(port int) bool {
	address := fmt.Sprintf(":%d", port)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return true
	}

	listener.Close()
	return false
}

func (h *Handler) validateStoragePath(path string) error {
	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %v", err)
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err == nil {
		// Path exists, check if it's a directory
		if !info.IsDir() {
			return fmt.Errorf("storage path exists but is not a directory: %s", absPath)
		}

		// Check if directory is writable
		testFile := filepath.Join(absPath, ".write_test")
		file, err := os.Create(testFile)
		if err != nil {
			return fmt.Errorf("storage path is not writable: %v", err)
		}
		file.Close()
		os.Remove(testFile)

		return nil
	}

	if os.IsNotExist(err) {
		// Path doesn't exist, try to create it
		if err := os.MkdirAll(absPath, 0755); err != nil {
			return fmt.Errorf("cannot create storage directory: %v", err)
		}

		h.logger.Info("Created storage directory", "path", absPath)
		return nil
	}

	return fmt.Errorf("cannot access storage path: %v", err)
}

func (h *Handler) updateLogLevel(level string) error {
	// Validate log level
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}

	normalizedLevel := strings.ToLower(level)
	if !validLevels[normalizedLevel] {
		return fmt.Errorf("invalid log level: %s", level)
	}

	h.config.General.LogLevel = normalizedLevel

	h.logger.UpdateLevel(normalizedLevel)
	h.logger.Info("Log level updated", "new_level", normalizedLevel)

	return nil
}

func (h *Handler) updateCORSSettings(cors *config.Config) error {
	// needs a CORS middleware that can be reconfigured.
	// Or restart the HTTP handlers with new settings

	// Update current config
	h.config.HTTP.CORS = cors.HTTP.CORS

	h.logger.Info("CORS settings updated",
		"enabled", cors.HTTP.CORS.Enabled,
		"allowed_origins", len(cors.HTTP.CORS.AllowedOrigins))

	return nil
}

func (h *Handler) updateStorageRetention(days int) {
	h.config.Storage.RetentionDays = days
	h.logger.Info("Storage retention updated", "retention_days", days)
}
