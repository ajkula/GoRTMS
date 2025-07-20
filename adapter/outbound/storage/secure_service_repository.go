package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	mathrand "math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ajkula/GoRTMS/adapter/outbound/crypto"
	"github.com/ajkula/GoRTMS/adapter/outbound/machineid"
	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

// implements ServiceRepository with encrypted storage
type SecureServiceRepository struct {
	crypto         outbound.CryptoService
	key            [32]byte
	filePath       string
	services       map[string]*model.ServiceAccount
	mutex          sync.RWMutex
	pendingUpdates map[string]*time.Timer
	updateMutex    sync.Mutex
	logger         outbound.Logger
}

// represents the encrypted data structure
type serviceStorageData struct {
	Services map[string]*encryptedServiceAccount `json:"services"`
	Version  string                              `json:"version"`
	Nonce    string                              `json:"nonce"`
}

// represents a service account with encrypted secret
type encryptedServiceAccount struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	EncryptedSecret string    `json:"encrypted_secret"`
	SecretNonce     string    `json:"secret_nonce"`
	Permissions     []string  `json:"permissions"`
	IPWhitelist     []string  `json:"ip_whitelist,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	LastUsed        time.Time `json:"last_used"`
	Enabled         bool      `json:"enabled"`
}

// creates a new secure service repository
func NewSecureServiceRepository(filePath string, logger outbound.Logger) (*SecureServiceRepository, error) {
	cryptoService := crypto.NewAESCryptoService()

	// Get machine ID for key derivation
	machineID, err := machineid.NewHardwareMachineID().GetMachineID()
	if err != nil {
		return nil, fmt.Errorf("failed to get machine ID: %w", err)
	}

	// Derive encryption key from machine ID
	key := cryptoService.DeriveKey(machineID)

	repo := &SecureServiceRepository{
		crypto:         cryptoService,
		key:            key,
		filePath:       filePath,
		services:       make(map[string]*model.ServiceAccount),
		pendingUpdates: make(map[string]*time.Timer),
		logger:         logger,
	}

	// Load existing services from file
	if err := repo.load(); err != nil {
		logger.Warn("Failed to load services from file, starting with empty repository", "error", err)
	}

	return repo, nil
}

// retrieves a service account by ID
func (r *SecureServiceRepository) GetByID(ctx context.Context, serviceID string) (*model.ServiceAccount, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	service, exists := r.services[serviceID]
	if !exists {
		return nil, fmt.Errorf("service account not found: %s", serviceID)
	}

	// Return a copy to prevent external modifications
	serviceCopy := *service
	return &serviceCopy, nil
}

// creates a new service account
func (r *SecureServiceRepository) Create(ctx context.Context, service *model.ServiceAccount) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Check if service already exists
	if _, exists := r.services[service.ID]; exists {
		return fmt.Errorf("service account already exists: %s", service.ID)
	}

	// Store the service
	serviceCopy := *service
	r.services[service.ID] = &serviceCopy

	// Persist to file
	if err := r.save(); err != nil {
		// Rollback
		delete(r.services, service.ID)
		return fmt.Errorf("failed to persist service account: %w", err)
	}

	r.logger.Info("Service account created", "serviceID", service.ID, "name", service.Name)
	return nil
}

// updates an existing service account
func (r *SecureServiceRepository) Update(ctx context.Context, service *model.ServiceAccount) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Check if service exists
	if _, exists := r.services[service.ID]; !exists {
		return fmt.Errorf("service account not found: %s", service.ID)
	}

	// Store the updated service
	serviceCopy := *service
	r.services[service.ID] = &serviceCopy

	// Persist to file
	if err := r.save(); err != nil {
		return fmt.Errorf("failed to persist service account update: %w", err)
	}

	r.logger.Info("Service account updated", "serviceID", service.ID)
	return nil
}

// deletes a service account
func (r *SecureServiceRepository) Delete(ctx context.Context, serviceID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Check if service exists
	if _, exists := r.services[serviceID]; !exists {
		return fmt.Errorf("service account not found: %s", serviceID)
	}

	// Delete the service
	delete(r.services, serviceID)

	// Persist to file
	if err := r.save(); err != nil {
		return fmt.Errorf("failed to persist service account deletion: %w", err)
	}

	r.logger.Info("Service account deleted", "serviceID", serviceID)
	return nil
}

// returns all service accounts
func (r *SecureServiceRepository) List(ctx context.Context) ([]*model.ServiceAccount, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	services := make([]*model.ServiceAccount, 0, len(r.services))
	for _, service := range r.services {
		// Return copies to prevent external modifications
		serviceCopy := *service
		services = append(services, &serviceCopy)
	}

	return services, nil
}

// updates the last used timestamp for a service
func (r *SecureServiceRepository) UpdateLastUsed(ctx context.Context, serviceID string) error {
	r.mutex.Lock()
	service, exists := r.services[serviceID]
	if !exists {
		r.mutex.Unlock()
		return fmt.Errorf("service account not found: %s", serviceID)
	}

	// Update last used timestamp
	service.LastUsed = time.Now()
	r.mutex.Unlock()

	// Debounce fs io
	r.updateMutex.Lock()
	defer r.updateMutex.Unlock()

	//cancel existing timer
	if timer, exists := r.pendingUpdates[serviceID]; exists {
		timer.Stop()
	}

	// new timer debounced
	r.pendingUpdates[serviceID] = time.AfterFunc(1*time.Second, func() {
		r.flushLastUsed(serviceID)
	})

	return nil
}

func (r *SecureServiceRepository) flushLastUsed(serviceID string) {
	r.mutex.RLock()
	services := make(map[string]*model.ServiceAccount, len(r.services))
	for id, svc := range r.services {
		svcCopy := *svc
		services[id] = &svcCopy
	}
	r.mutex.RUnlock()

	go func() {
		if err := r.saveServices(services); err != nil {
			r.logger.Error("failed to persist debounced last update", "serviceID", serviceID, "error", err)
		}

		// cloeanup
		r.updateMutex.Lock()
		delete(r.pendingUpdates, serviceID)
		r.updateMutex.Unlock()
	}()
}

func (r *SecureServiceRepository) saveServices(services map[string]*model.ServiceAccount) error {
	// Convert service accounts to encrypted service accounts
	encryptedServices := make(map[string]*encryptedServiceAccount)
	for id, service := range services {
		// Encrypt the service secret
		encryptedSecret, secretNonce, err := r.crypto.Encrypt([]byte(service.Secret), r.key)
		if err != nil {
			return fmt.Errorf("failed to encrypt secret for service %s: %w", id, err)
		}

		encryptedService := &encryptedServiceAccount{
			ID:              service.ID,
			Name:            service.Name,
			EncryptedSecret: hex.EncodeToString(encryptedSecret),
			SecretNonce:     hex.EncodeToString(secretNonce),
			Permissions:     service.Permissions,
			IPWhitelist:     service.IPWhitelist,
			CreatedAt:       service.CreatedAt,
			LastUsed:        service.LastUsed,
			Enabled:         service.Enabled,
		}

		encryptedServices[id] = encryptedService
	}

	// Create storage data structure
	storageData := serviceStorageData{
		Services: encryptedServices,
		Version:  "1.0",
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(storageData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal services data: %w", err)
	}

	// Encrypt the entire data
	encryptedData, nonce, err := r.crypto.Encrypt(jsonData, r.key)
	if err != nil {
		return fmt.Errorf("failed to encrypt services data: %w", err)
	}

	// Create outer structure with encrypted data and nonce
	outerData := struct {
		EncryptedServices string `json:"encrypted_services"`
		Nonce             string `json:"nonce"`
		Version           string `json:"version"`
	}{
		EncryptedServices: hex.EncodeToString(encryptedData),
		Nonce:             hex.EncodeToString(nonce),
		Version:           "1.0",
	}

	// Marshal outer structure
	finalData, err := json.MarshalIndent(outerData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal outer data: %w", err)
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(r.filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to file
	if err := os.WriteFile(r.filePath, finalData, 0600); err != nil {
		return fmt.Errorf("failed to write services file: %w", err)
	}

	return nil
}

// reads and decrypts services from file
func (r *SecureServiceRepository) load() error {
	// Check if file exists
	if _, err := os.Stat(r.filePath); os.IsNotExist(err) {
		r.logger.Info("Services file does not exist, starting with empty repository", "path", r.filePath)
		return nil
	}

	// Read encrypted file
	encryptedData, err := os.ReadFile(r.filePath)
	if err != nil {
		return fmt.Errorf("failed to read services file: %w", err)
	}

	// First, parse the outer structure to get the nonce
	var tempData struct {
		EncryptedServices string `json:"encrypted_services"`
		Nonce             string `json:"nonce"`
		Version           string `json:"version"`
	}

	if err := json.Unmarshal(encryptedData, &tempData); err != nil {
		return fmt.Errorf("failed to parse encrypted services file: %w", err)
	}

	// Decode hex-encoded encrypted data and nonce
	encryptedServices, err := hex.DecodeString(tempData.EncryptedServices)
	if err != nil {
		return fmt.Errorf("failed to decode encrypted services: %w", err)
	}

	nonce, err := hex.DecodeString(tempData.Nonce)
	if err != nil {
		return fmt.Errorf("failed to decode nonce: %w", err)
	}

	// Decrypt data
	decryptedData, err := r.crypto.Decrypt(encryptedServices, nonce, r.key)
	if err != nil {
		return fmt.Errorf("failed to decrypt services data: %w", err)
	}

	// Parse JSON
	var storageData serviceStorageData
	if err := json.Unmarshal(decryptedData, &storageData); err != nil {
		return fmt.Errorf("failed to parse services data: %w", err)
	}

	// Convert encrypted service accounts to service accounts
	r.services = make(map[string]*model.ServiceAccount)
	for id, encryptedService := range storageData.Services {
		// Decode and decrypt the service secret
		encryptedSecret, err := hex.DecodeString(encryptedService.EncryptedSecret)
		if err != nil {
			r.logger.Error("Failed to decode encrypted secret for service", "serviceID", id, "error", err)
			continue
		}

		secretNonce, err := hex.DecodeString(encryptedService.SecretNonce)
		if err != nil {
			r.logger.Error("Failed to decode secret nonce for service", "serviceID", id, "error", err)
			continue
		}

		secretBytes, err := r.crypto.Decrypt(encryptedSecret, secretNonce, r.key)
		if err != nil {
			r.logger.Error("Failed to decrypt secret for service", "serviceID", id, "error", err)
			continue
		}

		service := &model.ServiceAccount{
			ID:          encryptedService.ID,
			Name:        encryptedService.Name,
			Secret:      string(secretBytes),
			Permissions: encryptedService.Permissions,
			IPWhitelist: encryptedService.IPWhitelist,
			CreatedAt:   encryptedService.CreatedAt,
			LastUsed:    encryptedService.LastUsed,
			Enabled:     encryptedService.Enabled,
		}

		r.services[id] = service
	}

	r.logger.Info("Loaded services from file", "count", len(r.services), "path", r.filePath)
	return nil
}

// encrypts and writes services to file
func (r *SecureServiceRepository) save() error {
	return r.saveServices(r.services)
}

// generates a cryptographically secure secret for a service
func GenerateServiceSecret() string {
	// Generate 32 bytes of random data and encode as hex
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		// Fallback to timestamp-based generation if crypto/rand fails
		return fmt.Sprintf("secret-%d-%s", time.Now().UnixNano(), generateRandomString(16))
	}
	return hex.EncodeToString(secretBytes)
}

// generates a random string of specified length
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[mathrand.Intn(len(charset))]
	}
	return string(result)
}
