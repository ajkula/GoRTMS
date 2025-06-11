package storage

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

var (
	ErrFileNotFound    = errors.New("user database file not found")
	ErrCorruptedFile   = errors.New("user database file corrupted")
	ErrInvalidChecksum = errors.New("invalid file checksum")
)

type EncryptedUserFile struct {
	Version  uint32   `json:"version"`
	Nonce    []byte   `json:"nonce"`
	Data     []byte   `json:"data"`
	Checksum [32]byte `json:"checksum"`
}

type secureUserRepository struct {
	filePath  string
	crypto    outbound.CryptoService
	machineID outbound.MachineIDService
	logger    outbound.Logger
	key       [32]byte
}

func NewSecureUserRepository(
	filePath string,
	crypto outbound.CryptoService,
	machineID outbound.MachineIDService,
	logger outbound.Logger,
) (outbound.UserRepository, error) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create user database directory: %w", err)
	}

	// machine ID based cypher key
	id, err := machineID.GetMachineID()
	if err != nil {
		return nil, err
	}

	key := crypto.DeriveKey(id)

	return &secureUserRepository{
		filePath:  filePath,
		crypto:    crypto,
		machineID: machineID,
		logger:    logger,
		key:       key,
	}, nil
}

func (r *secureUserRepository) Save(db *model.UserDatabase) error {
	r.logger.Info("Saving user database", "path", r.filePath)

	// serialize to JSON
	jsonData, err := json.Marshal(db)
	if err != nil {
		return err
	}

	// cypher
	encrypted, nonce, err := r.crypto.Encrypt(jsonData, r.key)
	if err != nil {
		return err
	}

	// file struct with checksum
	fileData := EncryptedUserFile{
		Version:  1,
		Nonce:    nonce,
		Data:     encrypted,
		Checksum: sha256.Sum256(encrypted),
	}

	// serialize to fs
	fileJSON, err := json.Marshal(fileData)
	if err != nil {
		return err
	}

	if err := os.WriteFile(r.filePath, fileJSON, 0600); err != nil {
		return err
	}

	r.logger.Info("User database saved successfully")
	return nil
}

func (r *secureUserRepository) Load() (*model.UserDatabase, error) {
	r.logger.Info("Loading user database", "path", r.filePath)

	fileData, err := os.ReadFile(r.filePath)
	if os.IsNotExist(err) {
		return nil, model.ErrUserDatabaseNotFound
	}

	// deserialize file struct
	var encFile EncryptedUserFile
	if err := json.Unmarshal(fileData, &encFile); err != nil {
		return nil, ErrCorruptedFile
	}

	expectedChecksum := sha256.Sum256(encFile.Data)
	if expectedChecksum != encFile.Checksum {
		return nil, model.ErrInvalidChecksum
	}

	// decypher
	decrypted, err := r.crypto.Decrypt(encFile.Data, encFile.Nonce, r.key)
	if err != nil {
		return nil, err
	}

	// deserialize UserDatabase
	var db model.UserDatabase
	if err := json.Unmarshal(decrypted, &db); err != nil {
		return nil, model.ErrUserDatabaseCorrupted
	}

	if db.Users == nil {
		db.Users = make(map[string]*model.User)
	}

	r.logger.Info("User database loaded successfully", "user_count", len(db.Users))
	return &db, nil
}

func (r *secureUserRepository) Exists() bool {
	_, err := os.Stat(r.filePath)
	return !os.IsNotExist(err)
}
