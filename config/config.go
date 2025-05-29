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

// Config holds the global service configuration
type Config struct {
	// General configuration
	General struct {
		// NodeID is this node's unique identifier
		NodeID string `yaml:"nodeId"`

		// DataDir is the data storage directory
		DataDir string `yaml:"dataDir"`

		// LogLevel is the logging level
		LogLevel string `yaml:"logLevel"`

		// Development enables development mode
		Development bool `yaml:"development"`
	} `yaml:"general"`

	// Storage configuration
	Storage struct {
		// Engine specifies the storage engine
		Engine string `yaml:"engine"`

		// Path to the storage directory
		Path string `yaml:"path"`

		// RetentionDays is the number of days to retain messages
		RetentionDays int `yaml:"retentionDays"`

		// Sync determines if writes are synchronized
		Sync bool `yaml:"sync"`

		// MaxSizeMB is the max storage size in MB
		MaxSizeMB int `yaml:"maxSizeMB"`
	} `yaml:"storage"`

	// HTTP server configuration
	HTTP struct {
		// Enabled enables the HTTP server
		Enabled bool `yaml:"enabled"`

		// Address to bind the HTTP server
		Address string `yaml:"address"`

		// Port to bind the HTTP server
		Port int `yaml:"port"`

		// TLS enables TLS
		TLS bool `yaml:"tls"`

		// CertFile is the TLS certificate path
		CertFile string `yaml:"certFile"`

		// KeyFile is the TLS private key path
		KeyFile string `yaml:"keyFile"`

		// CORS configuration
		CORS struct {
			// Enabled enables CORS
			Enabled bool `yaml:"enabled"`

			// AllowedOrigins is the list of allowed origins
			AllowedOrigins []string `yaml:"allowedOrigins"`
		} `yaml:"cors"`

		// JWT configuration
		JWT struct {
			// Secret is the signing key for tokens
			Secret string `yaml:"secret"`

			// ExpirationMinutes is the token validity duration
			ExpirationMinutes int `yaml:"expirationMinutes"`
		} `yaml:"jwt"`
	} `yaml:"http"`

	// AMQP server configuration
	AMQP struct {
		// Enabled enables the AMQP server
		Enabled bool `yaml:"enabled"`

		// Address to bind the AMQP server
		Address string `yaml:"address"`

		// Port to bind the AMQP server
		Port int `yaml:"port"`
	} `yaml:"amqp"`

	// MQTT server configuration
	MQTT struct {
		// Enabled enables the MQTT server
		Enabled bool `yaml:"enabled"`

		// Address to bind the MQTT server
		Address string `yaml:"address"`

		// Port to bind the MQTT server
		Port int `yaml:"port"`
	} `yaml:"mqtt"`

	// gRPC server configuration
	GRPC struct {
		// Enabled enables the gRPC server
		Enabled bool `yaml:"enabled"`

		// Address to bind the gRPC server
		Address string `yaml:"address"`

		// Port to bind the gRPC server
		Port int `yaml:"port"`
	} `yaml:"grpc"`

	// Security configuration
	Security struct {
		// EnableAuthentication enables authentication
		EnableAuthentication bool `yaml:"enableAuthentication"`

		// EnableAuthorization enables authorization
		EnableAuthorization bool `yaml:"enableAuthorization"`

		// AdminUsername is the admin username
		AdminUsername string `yaml:"adminUsername"`

		// AdminPassword is the admin password
		AdminPassword string `yaml:"adminPassword"`
	} `yaml:"security"`

	// Monitoring configuration
	Monitoring struct {
		// Enabled enables monitoring
		Enabled bool `yaml:"enabled"`

		// Address to bind the monitoring server
		Address string `yaml:"address"`

		// Port to bind the monitoring server
		Port int `yaml:"port"`

		// Prometheus enables Prometheus export
		Prometheus bool `yaml:"prometheus"`
	} `yaml:"monitoring"`

	// Cluster configuration
	Cluster struct {
		// Enabled enables cluster mode
		Enabled bool `yaml:"enabled"`

		// Peers is the list of cluster peers
		Peers []string `yaml:"peers"`

		// HeartbeatInterval is the interval between heartbeats
		HeartbeatInterval time.Duration `yaml:"heartbeatInterval"`

		// ElectionTimeout is the timeout for elections
		ElectionTimeout time.Duration `yaml:"electionTimeout"`
	} `yaml:"cluster"`

	// Predefined domain configurations
	Domains []DomainConfig `yaml:"domains"`

	Logging struct {
		Level       string `yaml:"level"` // "ERROR", "WARN", "INFO", "DEBUG"
		ChannelSize int    `yaml:"channelSize"`
		Format      string `yaml:"format"`
		Output      string `yaml:"output"`
		FilePath    string `yaml:"filePath"`
	} `yaml:"logging"`
}

// DomainConfig holds the configuration for a domain
type DomainConfig struct {
	// Name is the domain name
	Name string `yaml:"name"`

	// Schema is the validation schema
	Schema map[string]interface{} `yaml:"schema"`

	// Queues is the list of queues
	Queues []QueueConfig `yaml:"queues"`

	// Routes is the list of routing rules
	Routes []RoutingRule `yaml:"routes"`
}

// QueueConfig holds the configuration for a queue
type QueueConfig struct {
	// Name is the queue name
	Name string `yaml:"name"`

	// Config is the queue configuration
	Config model.QueueConfig `yaml:"config"`
}

// RoutingRule defines a routing rule
type RoutingRule struct {
	// SourceQueue is the source queue
	SourceQueue string `yaml:"sourceQueue"`

	// DestinationQueue is the destination queue
	DestinationQueue string `yaml:"destinationQueue"`

	// Predicate defines the routing condition
	Predicate map[string]interface{} `yaml:"predicate"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	c := &Config{}

	// General configuration
	c.General.NodeID = "node1"
	c.General.DataDir = "./data"
	c.General.LogLevel = "info"
	c.General.Development = false

	// Storage configuration
	c.Storage.Engine = "memory"
	c.Storage.Path = "./data/storage"
	c.Storage.RetentionDays = 7
	c.Storage.Sync = true
	c.Storage.MaxSizeMB = 1024

	// HTTP server configuration
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

	// AMQP server configuration
	c.AMQP.Enabled = false
	c.AMQP.Address = "0.0.0.0"
	c.AMQP.Port = 5672

	// MQTT server configuration
	c.MQTT.Enabled = false
	c.MQTT.Address = "0.0.0.0"
	c.MQTT.Port = 1883

	// gRPC server configuration
	c.GRPC.Enabled = false
	c.GRPC.Address = "0.0.0.0"
	c.GRPC.Port = 50051

	// Security configuration
	c.Security.EnableAuthentication = false
	c.Security.EnableAuthorization = false
	c.Security.AdminUsername = "admin"
	c.Security.AdminPassword = "admin"

	// monitoring configuration
	c.Monitoring.Enabled = true
	c.Monitoring.Address = "0.0.0.0"
	c.Monitoring.Port = 9090
	c.Monitoring.Prometheus = true

	// cluster configuration
	c.Cluster.Enabled = false
	c.Cluster.Peers = []string{}
	c.Cluster.HeartbeatInterval = 100 * time.Millisecond
	c.Cluster.ElectionTimeout = 1000 * time.Millisecond

	// Logging configuration defaults
	c.Logging.Level = "INFO"
	c.Logging.ChannelSize = 1000
	c.Logging.Format = "json"
	c.Logging.Output = "stdout"
	c.Logging.FilePath = ""

	return c
}

// LoadConfig loads the configuration from a file
func LoadConfig(path string) (*Config, error) {
	// Check if the file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", path)
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Load the default configuration
	config := DefaultConfig()

	// Decode the YAML file
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Complete relative paths
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

	// Validate the configuration
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}

// SaveConfig saves the configuration to a file
func SaveConfig(config *Config, path string) error {
	// Encode the configuration to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	// Create parent directory if necessary
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	// Check the log level
	logLevel := strings.ToLower(config.General.LogLevel)
	if logLevel != "debug" && logLevel != "info" && logLevel != "warn" && logLevel != "error" {
		return fmt.Errorf("invalid log level: %s", config.General.LogLevel)
	}

	// Check the storage engine
	engine := strings.ToLower(config.Storage.Engine)
	if engine != "memory" && engine != "file" && engine != "sqlite" && engine != "badger" {
		return fmt.Errorf("invalid storage engine: %s", config.Storage.Engine)
	}

	// check ports
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

	// Check the TLS configurations
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
