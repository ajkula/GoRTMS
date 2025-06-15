package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"

	"github.com/ajkula/GoRTMS/domain/port/outbound"
	"golang.org/x/crypto/argon2"
)

type AesCryptoService struct{}

func NewAESCryptoService() outbound.CryptoService {
	return &AesCryptoService{}
}

func (c *AesCryptoService) Encrypt(data []byte, key [32]byte) (encrypted []byte, nonce []byte, err error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	nonceBytes := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonceBytes); err != nil {
		return nil, nil, err
	}

	ciphertext := gcm.Seal(nil, nonceBytes, data, nil)
	return ciphertext, nonceBytes, nil
}

func (c *AesCryptoService) Decrypt(encrypted []byte, nonce []byte, key [32]byte) ([]byte, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

func (c *AesCryptoService) DeriveKey(machineID string) [32]byte {
	// derivate 32 bytes key from machine ID
	hash := sha256.Sum256([]byte(machineID + "gortms-encryption-key"))
	return hash
}

func (c *AesCryptoService) GenerateSalt() [32]byte {
	var salt [32]byte
	rand.Read(salt[:])
	return salt
}

func (c *AesCryptoService) HashPassword(password string, salt [16]byte) string {
	// Argon2id - OWASP 2024
	hash := argon2.IDKey([]byte(password), salt[:], 1, 64*1024, 4, 32)
	return hex.EncodeToString(hash)
}

func (c *AesCryptoService) VerifyPassword(password, hash string, salt [16]byte) bool {
	expectedHash := c.HashPassword(password, salt)
	return expectedHash == hash
}
