package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
	"gopkg.in/yaml.v3"
)

// Config contient la configuration globale du service
type Config struct {
	// Configuration générale
	General struct {
		// NodeID est l'identifiant unique de ce nœud
		NodeID string `yaml:"nodeId"`

		// DataDir est le répertoire de stockage des données
		DataDir string `yaml:"dataDir"`

		// LogLevel est le niveau de journalisation
		LogLevel string `yaml:"logLevel"`

		// Development indique si le mode développement est activé
		Development bool `yaml:"development"`
	} `yaml:"general"`

	// Configuration du stockage
	Storage struct {
		// Engine est le moteur de stockage à utiliser
		Engine string `yaml:"engine"`

		// Path est le chemin vers le répertoire de stockage
		Path string `yaml:"path"`

		// RetentionDays est le nombre de jours de rétention des messages
		RetentionDays int `yaml:"retentionDays"`

		// Sync indique si les écritures doivent être synchronisées
		Sync bool `yaml:"sync"`

		// MaxSizeMB est la taille maximale du stockage en Mo
		MaxSizeMB int `yaml:"maxSizeMB"`
	} `yaml:"storage"`

	// Configuration du serveur HTTP
	HTTP struct {
		// Enabled indique si le serveur HTTP est activé
		Enabled bool `yaml:"enabled"`

		// Address est l'adresse d'écoute du serveur HTTP
		Address string `yaml:"address"`

		// Port est le port d'écoute du serveur HTTP
		Port int `yaml:"port"`

		// TLS indique si le TLS est activé
		TLS bool `yaml:"tls"`

		// CertFile est le chemin vers le certificat TLS
		CertFile string `yaml:"certFile"`

		// KeyFile est le chemin vers la clé privée TLS
		KeyFile string `yaml:"keyFile"`

		// CORS est la configuration CORS
		CORS struct {
			// Enabled indique si le CORS est activé
			Enabled bool `yaml:"enabled"`

			// AllowedOrigins est la liste des origines autorisées
			AllowedOrigins []string `yaml:"allowedOrigins"`
		} `yaml:"cors"`

		// JWT est la configuration JWT
		JWT struct {
			// Secret est la clé secrète pour signer les tokens
			Secret string `yaml:"secret"`

			// ExpirationMinutes est la durée de validité des tokens en minutes
			ExpirationMinutes int `yaml:"expirationMinutes"`
		} `yaml:"jwt"`
	} `yaml:"http"`

	// Configuration du serveur AMQP
	AMQP struct {
		// Enabled indique si le serveur AMQP est activé
		Enabled bool `yaml:"enabled"`

		// Address est l'adresse d'écoute du serveur AMQP
		Address string `yaml:"address"`

		// Port est le port d'écoute du serveur AMQP
		Port int `yaml:"port"`
	} `yaml:"amqp"`

	// Configuration du serveur MQTT
	MQTT struct {
		// Enabled indique si le serveur MQTT est activé
		Enabled bool `yaml:"enabled"`

		// Address est l'adresse d'écoute du serveur MQTT
		Address string `yaml:"address"`

		// Port est le port d'écoute du serveur MQTT
		Port int `yaml:"port"`
	} `yaml:"mqtt"`

	// Configuration du serveur gRPC
	GRPC struct {
		// Enabled indique si le serveur gRPC est activé
		Enabled bool `yaml:"enabled"`

		// Address est l'adresse d'écoute du serveur gRPC
		Address string `yaml:"address"`

		// Port est le port d'écoute du serveur gRPC
		Port int `yaml:"port"`
	} `yaml:"grpc"`

	// Configuration de la sécurité
	Security struct {
		// EnableAuthentication indique si l'authentification est activée
		EnableAuthentication bool `yaml:"enableAuthentication"`

		// EnableAuthorization indique si l'autorisation est activée
		EnableAuthorization bool `yaml:"enableAuthorization"`

		// AdminUsername est le nom d'utilisateur de l'administrateur
		AdminUsername string `yaml:"adminUsername"`

		// AdminPassword est le mot de passe de l'administrateur
		AdminPassword string `yaml:"adminPassword"`
	} `yaml:"security"`

	// Configuration du monitoring
	Monitoring struct {
		// Enabled indique si le monitoring est activé
		Enabled bool `yaml:"enabled"`

		// Address est l'adresse d'écoute du serveur de monitoring
		Address string `yaml:"address"`

		// Port est le port d'écoute du serveur de monitoring
		Port int `yaml:"port"`

		// Prometheus indique si l'export Prometheus est activé
		Prometheus bool `yaml:"prometheus"`
	} `yaml:"monitoring"`

	// Configuration du cluster
	Cluster struct {
		// Enabled indique si le mode cluster est activé
		Enabled bool `yaml:"enabled"`

		// Peers est la liste des pairs du cluster
		Peers []string `yaml:"peers"`

		// HeartbeatInterval est l'intervalle entre les heartbeats
		HeartbeatInterval time.Duration `yaml:"heartbeatInterval"`

		// ElectionTimeout est le timeout pour les élections
		ElectionTimeout time.Duration `yaml:"electionTimeout"`
	} `yaml:"cluster"`

	// Configuration des domaines prédéfinis
	Domains []DomainConfig `yaml:"domains"`
}

// DomainConfig contient la configuration d'un domaine
type DomainConfig struct {
	// Name est le nom du domaine
	Name string `yaml:"name"`

	// Schema est le schéma de validation
	Schema map[string]interface{} `yaml:"schema"`

	// Queues est la liste des files d'attente
	Queues []QueueConfig `yaml:"queues"`

	// Routes est la liste des règles de routage
	Routes []RoutingRule `yaml:"routes"`
}

// QueueConfig est la configuration d'une file d'attente
type QueueConfig struct {
	// Name est le nom de la file d'attente
	Name string `yaml:"name"`

	// Config est la configuration de la file d'attente
	Config model.QueueConfig `yaml:"config"`
}

// RoutingRule est une règle de routage
type RoutingRule struct {
	// SourceQueue est la file d'attente source
	SourceQueue string `yaml:"sourceQueue"`

	// DestinationQueue est la file d'attente destination
	DestinationQueue string `yaml:"destinationQueue"`

	// Predicate est le prédicat de routage
	Predicate map[string]interface{} `yaml:"predicate"`
}

// DefaultConfig retourne une configuration par défaut
func DefaultConfig() *Config {
	c := &Config{}

	// Configuration générale
	c.General.NodeID = "node1"
	c.General.DataDir = "./data"
	c.General.LogLevel = "info"
	c.General.Development = false

	// Configuration du stockage
	c.Storage.Engine = "memory"
	c.Storage.Path = "./data/storage"
	c.Storage.RetentionDays = 7
	c.Storage.Sync = true
	c.Storage.MaxSizeMB = 1024

	// Configuration du serveur HTTP
	c.HTTP.Enabled = true
	c.HTTP.Address = "0.0.0.0"
	c.HTTP.Port = 8080
	c.HTTP.TLS = false
	c.HTTP.CertFile = ""
	c.HTTP.KeyFile = ""
	c.HTTP.CORS.Enabled = true
	c.HTTP.CORS.AllowedOrigins = []string{"*"}
	c.HTTP.JWT.Secret = "changeme"
	c.HTTP.JWT.ExpirationMinutes = 60

	// Configuration du serveur AMQP
	c.AMQP.Enabled = false
	c.AMQP.Address = "0.0.0.0"
	c.AMQP.Port = 5672

	// Configuration du serveur MQTT
	c.MQTT.Enabled = false
	c.MQTT.Address = "0.0.0.0"
	c.MQTT.Port = 1883

	// Configuration du serveur gRPC
	c.GRPC.Enabled = false
	c.GRPC.Address = "0.0.0.0"
	c.GRPC.Port = 50051

	// Configuration de la sécurité
	c.Security.EnableAuthentication = false
	c.Security.EnableAuthorization = false
	c.Security.AdminUsername = "admin"
	c.Security.AdminPassword = "admin"

	// Configuration du monitoring
	c.Monitoring.Enabled = true
	c.Monitoring.Address = "0.0.0.0"
	c.Monitoring.Port = 9090
	c.Monitoring.Prometheus = true

	// Configuration du cluster
	c.Cluster.Enabled = false
	c.Cluster.Peers = []string{}
	c.Cluster.HeartbeatInterval = 100 * time.Millisecond
	c.Cluster.ElectionTimeout = 1000 * time.Millisecond

	return c
}

// LoadConfig charge la configuration à partir d'un fichier
func LoadConfig(path string) (*Config, error) {
	// Vérifier si le fichier existe
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", path)
	}

	// Lire le fichier
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Charger la configuration par défaut
	config := DefaultConfig()

	// Décoder le fichier YAML
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Compléter les chemins relatifs
	if !filepath.IsAbs(config.General.DataDir) {
		dir, err := filepath.Abs(filepath.Dir(path))
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path: %w", err)
		}
		config.General.DataDir = filepath.Join(dir, config.General.DataDir)
	}

	if !filepath.IsAbs(config.Storage.Path) {
		config.Storage.Path = filepath.Join(config.General.DataDir, config.Storage.Path)
	}

	// Valider la configuration
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}

// SaveConfig sauvegarde la configuration dans un fichier
func SaveConfig(config *Config, path string) error {
	// Encoder la configuration en YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	// Créer le répertoire parent si nécessaire
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Écrire le fichier
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// validateConfig valide la configuration
func validateConfig(config *Config) error {
	// Vérifier le niveau de journalisation
	logLevel := strings.ToLower(config.General.LogLevel)
	if logLevel != "debug" && logLevel != "info" && logLevel != "warn" && logLevel != "error" {
		return fmt.Errorf("invalid log level: %s", config.General.LogLevel)
	}

	// Vérifier le moteur de stockage
	engine := strings.ToLower(config.Storage.Engine)
	if engine != "memory" && engine != "file" && engine != "sqlite" && engine != "badger" {
		return fmt.Errorf("invalid storage engine: %s", config.Storage.Engine)
	}

	// Vérifier les ports
	if config.HTTP.Enabled && (config.HTTP.Port < 1 || config.HTTP.Port > 65535) {
		return fmt.Errorf("invalid HTTP port: %d", config.HTTP.Port)
	}

	if config.AMQP.Enabled && (config.AMQP.Port < 1 || config.AMQP.Port > 65535) {
		return fmt.Errorf("invalid AMQP port: %d", config.AMQP.Port)
	}

	if config.MQTT.Enabled && (config.MQTT.Port < 1 || config.MQTT.Port > 65535) {
		return fmt.Errorf("invalid MQTT port: %d", config.MQTT.Port)
	}

	if config.GRPC.Enabled && (config.GRPC.Port < 1 || config.GRPC.Port > 65535) {
		return fmt.Errorf("invalid gRPC port: %d", config.GRPC.Port)
	}

	// Vérifier les configurations TLS
	if config.HTTP.TLS {
		if config.HTTP.CertFile == "" || config.HTTP.KeyFile == "" {
			return fmt.Errorf("TLS enabled but certificate or key file not specified")
		}
		if _, err := os.Stat(config.HTTP.CertFile); os.IsNotExist(err) {
			return fmt.Errorf("certificate file not found: %s", config.HTTP.CertFile)
		}
		if _, err := os.Stat(config.HTTP.KeyFile); os.IsNotExist(err) {
			return fmt.Errorf("key file not found: %s", config.HTTP.KeyFile)
		}
	}

	return nil
}
