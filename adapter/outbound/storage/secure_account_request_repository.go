package storage

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

// represents the structure of the encrypted file
type EncryptedAccountRequestFile struct {
	Version  uint32   `json:"version"`
	Nonce    []byte   `json:"nonce"`
	Data     []byte   `json:"data"`
	Checksum [32]byte `json:"checksum"`
}

type secureAccountRequestRepository struct {
	filePath  string
	crypto    outbound.CryptoService
	machineID outbound.MachineIDService
	logger    outbound.Logger
	key       [32]byte
	database  *model.AccountRequestDatabase
}

func NewSecureAccountRequestRepository(
	filePath string,
	crypto outbound.CryptoService,
	machineID outbound.MachineIDService,
	logger outbound.Logger,
) (outbound.AccountRequestRepository, error) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create account request database directory: %w", err)
	}

	id, err := machineID.GetMachineID()
	if err != nil {
		return nil, err
	}

	key := crypto.DeriveKey(id)

	return &secureAccountRequestRepository{
		filePath:  filePath,
		crypto:    crypto,
		machineID: machineID,
		logger:    logger,
		key:       key,
	}, nil
}

func (r *secureAccountRequestRepository) Save(ctx context.Context, db *model.AccountRequestDatabase) error {
	r.logger.Info("Saving account request database", "path", r.filePath)

	jsonData, err := json.Marshal(db)
	if err != nil {
		return err
	}

	encrypted, nonce, err := r.crypto.Encrypt(jsonData, r.key)
	if err != nil {
		return err
	}

	// create file struct with checksum
	fileData := EncryptedAccountRequestFile{
		Version:  1,
		Nonce:    nonce,
		Data:     encrypted,
		Checksum: sha256.Sum256(encrypted),
	}

	fileJSON, err := json.Marshal(fileData)
	if err != nil {
		return err
	}

	if err := os.WriteFile(r.filePath, fileJSON, 0600); err != nil {
		return err
	}

	// cache db
	r.database = db

	r.logger.Info("Account request database saved successfully")
	return nil
}

func (r *secureAccountRequestRepository) Load(ctx context.Context) (*model.AccountRequestDatabase, error) {
	r.logger.Info("Loading account request database", "path", r.filePath)

	fileData, err := os.ReadFile(r.filePath)
	if os.IsNotExist(err) {
		return nil, model.ErrAccountRequestDatabaseNotFound
	}
	if err != nil {
		return nil, err
	}

	// deserialize file structure
	var encFile EncryptedAccountRequestFile
	if err := json.Unmarshal(fileData, &encFile); err != nil {
		return nil, model.ErrAccountRequestDatabaseCorrupted
	}

	expectedChecksum := sha256.Sum256(encFile.Data)
	if expectedChecksum != encFile.Checksum {
		return nil, model.ErrInvalidChecksum
	}

	decrypted, err := r.crypto.Decrypt(encFile.Data, encFile.Nonce, r.key)
	if err != nil {
		return nil, err
	}

	// deserialize AccountRequestDatabase
	var db model.AccountRequestDatabase
	if err := json.Unmarshal(decrypted, &db); err != nil {
		return nil, model.ErrAccountRequestDatabaseCorrupted
	}

	// initialize maps if nil
	if db.Requests == nil {
		db.Requests = make(map[string]*model.AccountRequest)
	}

	// cache the db
	r.database = &db

	r.logger.Info("Account request database loaded successfully", "request_count", len(db.Requests))
	return &db, nil
}

func (r *secureAccountRequestRepository) Exists() bool {
	_, err := os.Stat(r.filePath)
	return !os.IsNotExist(err)
}

func (r *secureAccountRequestRepository) Store(ctx context.Context, request *model.AccountRequest) error {
	// load database if not cached
	if r.database == nil {
		db, err := r.Load(ctx)
		if err != nil && err != model.ErrAccountRequestDatabaseNotFound {
			return err
		}
		if db == nil {
			// create database
			r.database = &model.AccountRequestDatabase{
				Requests: make(map[string]*model.AccountRequest),
				Salt:     r.crypto.GenerateSalt(),
			}
		}
	}

	// add or update request
	r.database.Requests[request.ID] = request

	return r.Save(ctx, r.database)
}

func (r *secureAccountRequestRepository) GetByID(ctx context.Context, requestID string) (*model.AccountRequest, error) {
	if r.database == nil {
		_, err := r.Load(ctx)
		if err != nil {
			return nil, err
		}
	}

	if request, exists := r.database.Requests[requestID]; exists {
		return request, nil
	}

	return nil, model.ErrAccountRequestNotFound
}

func (r *secureAccountRequestRepository) GetByUsername(ctx context.Context, username string) (*model.AccountRequest, error) {
	if r.database == nil {
		_, err := r.Load(ctx)
		if err != nil {
			return nil, err
		}
	}

	// search through all requests
	for _, request := range r.database.Requests {
		if request.Username == username {
			return request, nil
		}
	}

	return nil, model.ErrAccountRequestNotFound
}

func (r *secureAccountRequestRepository) List(ctx context.Context, status *model.AccountRequestStatus) ([]*model.AccountRequest, error) {
	if r.database == nil {
		_, err := r.Load(ctx)
		if err != nil {
			return nil, err
		}
	}

	var result []*model.AccountRequest

	for _, request := range r.database.Requests {
		// filter by status if specified
		if status == nil || request.Status == *status {
			result = append(result, request)
		}
	}

	return result, nil
}

func (r *secureAccountRequestRepository) Delete(ctx context.Context, requestID string) error {
	if r.database == nil {
		_, err := r.Load(ctx)
		if err != nil {
			return err
		}
	}

	if _, exists := r.database.Requests[requestID]; !exists {
		return model.ErrAccountRequestNotFound
	}

	// remove request
	delete(r.database.Requests, requestID)

	return r.Save(ctx, r.database)
}

func (r *secureAccountRequestRepository) GetPendingRequests(ctx context.Context) ([]*model.AccountRequest, error) {
	status := model.AccountRequestPending
	return r.List(ctx, &status)
}
