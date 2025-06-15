package config

import "time"

// safe for API structure
type PublicConfig struct {
	General struct {
		NodeID      string `yaml:"nodeId"`
		DataDir     string `yaml:"dataDir"`
		LogLevel    string `yaml:"logLevel"`
		Development bool   `yaml:"development"`
	} `yaml:"general"`

	Storage struct {
		Engine        string `yaml:"engine"`
		Path          string `yaml:"path"`
		RetentionDays int    `yaml:"retentionDays"`
		Sync          bool   `yaml:"sync"`
		MaxSizeMB     int    `yaml:"maxSizeMB"`
	} `yaml:"storage"`

	HTTP struct {
		Enabled  bool   `yaml:"enabled"`
		Address  string `yaml:"address"`
		Port     int    `yaml:"port"`
		TLS      bool   `yaml:"tls"`
		CertFile string `yaml:"certFile"`
		KeyFile  string `yaml:"keyFile"`

		CORS struct {
			Enabled        bool     `yaml:"enabled"`
			AllowedOrigins []string `yaml:"allowedOrigins"`
		} `yaml:"cors"`

		JWT struct {
			ExpirationMinutes int `yaml:"expirationMinutes"`
		} `yaml:"jwt"`
	} `yaml:"http"`

	// AMQP, MQTT, GRPC
	AMQP struct {
		Enabled bool   `yaml:"enabled"`
		Address string `yaml:"address"`
		Port    int    `yaml:"port"`
	} `yaml:"amqp"`

	MQTT struct {
		Enabled bool   `yaml:"enabled"`
		Address string `yaml:"address"`
		Port    int    `yaml:"port"`
	} `yaml:"mqtt"`

	GRPC struct {
		Enabled bool   `yaml:"enabled"`
		Address string `yaml:"address"`
		Port    int    `yaml:"port"`
	} `yaml:"grpc" json:"grpc"`

	Security struct {
		EnableAuthentication bool   `yaml:"enableAuthentication"`
		EnableAuthorization  bool   `yaml:"enableAuthorization"`
		AdminUsername        string `yaml:"adminUsername"`

		// HMAC configuration for service authentication
		HMAC struct {
			Enabled         bool   `yaml:"enabled"`
			TimestampWindow string `yaml:"timestampWindow"`
			RequireTLS      bool   `yaml:"requireTLS"`
		} `yaml:"hmac"`
	} `yaml:"security" json:"security"`

	// Monitoring, Cluster, Domains, Logging
	Monitoring struct {
		Enabled    bool   `yaml:"enabled"`
		Address    string `yaml:"address"`
		Port       int    `yaml:"port"`
		Prometheus bool   `yaml:"prometheus"`
	} `yaml:"monitoring"`

	Cluster struct {
		Enabled           bool          `yaml:"enabled"`
		Peers             []string      `yaml:"peers"`
		HeartbeatInterval time.Duration `yaml:"heartbeatInterval"`
		ElectionTimeout   time.Duration `yaml:"electionTimeout"`
	} `yaml:"cluster"`

	Domains []DomainConfig `yaml:"domains"`

	Logging struct {
		Level       string `yaml:"level"`
		ChannelSize int    `yaml:"channelSize"`
		Format      string `yaml:"format"`
		Output      string `yaml:"output"`
		FilePath    string `yaml:"filePath"`
	} `yaml:"logging"`
}
