package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"net"
	"time"

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

// GenerateTLSCertificate generates a self-signed TLS certificate
func (c *AesCryptoService) GenerateTLSCertificate(hostname string) (certPEM, keyPEM []byte, err error) {
	// Generate RSA private key (2048 bits for good security)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"GoRTMS Auto-Generated"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // Valid for 1 year
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Add hostname to certificate
	if hostname != "" {
		template.DNSNames = []string{hostname}

		// If hostname is an IP address, add it to IPAddresses
		if ip := net.ParseIP(hostname); ip != nil {
			template.IPAddresses = []net.IP{ip}
		}
	}

	// Add common hostnames for local development
	template.DNSNames = append(template.DNSNames, "localhost", "127.0.0.1", "::1")
	template.IPAddresses = append(template.IPAddresses,
		net.IPv4(127, 0, 0, 1),
		net.IPv6loopback,
	)

	// Create the certificate
	certDER, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		&template, // self-signed
		&privateKey.PublicKey,
		privateKey,
	)
	if err != nil {
		return nil, nil, err
	}

	// Encode certificate to PEM
	certPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	// Encode private key to PEM
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, nil, err
	}

	keyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyDER,
	})

	return certPEM, keyPEM, nil
}
