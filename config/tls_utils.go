// config/tls_utils.go - Utilities for TLS certificate management

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

// EnsureTLSCertificates ensures TLS certificates exist, generating them if necessary
func EnsureTLSCertificates(config *Config, cryptoService outbound.CryptoService, logger outbound.Logger) error {
	if !config.HTTP.TLS {
		return nil // TLS not enabled
	}

	certPath := config.HTTP.CertFile
	keyPath := config.HTTP.KeyFile

	// If certificate paths are not specified, use default paths
	if certPath == "" || keyPath == "" {
		tlsDir := filepath.Join(config.General.DataDir, "tls")
		if err := os.MkdirAll(tlsDir, 0755); err != nil {
			return fmt.Errorf("failed to create TLS directory: %w", err)
		}

		certPath = filepath.Join(tlsDir, "server.crt")
		keyPath = filepath.Join(tlsDir, "server.key")

		// Update config with generated paths
		config.HTTP.CertFile = certPath
		config.HTTP.KeyFile = keyPath
	}

	// Check if certificates already exist
	if certificatesExist(certPath, keyPath) {
		logger.Info("Using existing TLS certificates",
			"certFile", certPath,
			"keyFile", keyPath)
		return nil
	}

	// Generate new certificates
	logger.Info("Generating self-signed TLS certificates...")

	hostname := config.HTTP.Address
	if hostname == "0.0.0.0" || hostname == "" {
		hostname = "localhost"
	}

	certPEM, keyPEM, err := cryptoService.GenerateTLSCertificate(hostname)
	if err != nil {
		return fmt.Errorf("failed to generate TLS certificates: %w", err)
	}

	// Save certificate
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		return fmt.Errorf("failed to save certificate: %w", err)
	}

	// Save private key
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return fmt.Errorf("failed to save private key: %w", err)
	}

	logger.Info("🔐 TLS certificates generated successfully",
		"certFile", certPath,
		"keyFile", keyPath,
		"hostname", hostname,
		"validity", "1 year",
		"note", "Self-signed certificate - browsers will show security warning")

	return nil
}

// certificatesExist checks if both certificate and key files exist
func certificatesExist(certPath, keyPath string) bool {
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return false
	}
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return false
	}
	return true
}
