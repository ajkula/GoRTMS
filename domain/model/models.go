package model

import (
	"context"
	"sync"
	"time"
)

// Message represents a message in the system
type Message struct {
	ID        string            // Unique identifier of the message
	Topic     string            // Topic of the message
	Payload   []byte            // Message content
	Headers   map[string]string // Message headers
	Metadata  map[string]any    // Metadata for routing and processing
	Timestamp time.Time         // Message creation timestamp
}

// MessageHandler is a callback function for processing messages
type MessageHandler func(*Message) error

// Queue represents a message queue
type Queue struct {
	Name         string      // Queue name
	DomainName   string      // Parent domain name
	Config       QueueConfig // Queue configuration
	MessageCount int         // Number of messages in the queue
}

// QueueConfig contains the configuration for a message queue
type QueueConfig struct {
	// IsPersistent indicates whether messages should be persisted
	// when deleted from the repo via a dependency
	IsPersistent bool `yaml:"isPersistent"`

	// MaxSize defines the maximum queue size (0 = unlimited)
	MaxSize int `yaml:"maxSize"`

	// TTL defines the time-to-live for messages (0 = unlimited)
	TTL time.Duration `yaml:"ttl"`

	// New fields
	WorkerCount int `yaml:"workerCount"`

	// RetryEnabled enables the retry mechanism
	RetryEnabled bool `yaml:"retryEnabled"`

	// RetryConfig defines the retry settings
	RetryConfig *RetryConfig `yaml:"retryConfig,omitempty"`

	// CircuitBreakerEnabled enables the circuit breaker
	CircuitBreakerEnabled bool                  `yaml:"circuitBreakerEnabled"`
	CircuitBreakerConfig  *CircuitBreakerConfig `yaml:"circuitBreakerConfig,omitempty"`
}

// CircuitBreakerConfig defines the circuit breaker configuration
type CircuitBreakerConfig struct {
	ErrorThreshold   float64       `yaml:"errorThreshold"`
	MinimumRequests  int           `yaml:"minimumRequests"`
	OpenTimeout      time.Duration `yaml:"openTimeout"`
	SuccessThreshold int           `yaml:"successThreshold"`
}

// QueueHandler defines the operations for a concurrent queue implementation
type QueueHandler interface {
	// Enqueue adds a message to the queue in a thread-safe way
	Enqueue(ctx context.Context, message *Message) error

	// Dequeue retrieves a message from the queue in a thread-safe way
	Dequeue(ctx context.Context) (*Message, error)

	// GetQueue returns the associated Queue structure
	GetQueue() *Queue

	// Start starts workers to process messages
	Start(ctx context.Context)

	// Stop stops workers and releases resources
	Stop()

	// For dual-channel system
	AddConsumerGroup(groupID string, lastIndex int64) error
	RemoveConsumerGroup(groupID string)
	RequestMessages(groupID string, count int) error
	ConsumeMessage(groupID string, timeout time.Duration) (*Message, error)
}

// RetryConfig defines the configuration for retrying failed messages
type RetryConfig struct {
	MaxRetries int

	// Delay before the first attempt
	InitialDelay time.Duration

	// Maximum delay between attempts
	MaxDelay time.Duration

	// Factor defines the multiplier for exponential backoff
	Factor float64
}

// MessageWithRetry represents a message with retry information
type MessageWithRetry struct {
	Message     *Message
	RetryCount  int
	NextRetryAt time.Time
	Handler     MessageHandler
}

type CircuitBreakerState int

const (
	// CircuitClosed = closed circuit, messages pass through normally
	CircuitClosed CircuitBreakerState = iota

	// CircuitOpen = open circuit, messages are rejected
	CircuitOpen

	// CircuitHalfOpen = circuit in test mode
	CircuitHalfOpen
)

// CircuitBreaker implements the pattern of the same name to protect against overload
type CircuitBreaker struct {
	ErrorThreshold   float64             // Error threshold to open the circuit
	SuccessThreshold int                 // Number of successes to close the circuit
	MinimumRequests  int                 // Minimum number of requests before applying logic
	OpenTimeout      time.Duration       // Duration the circuit remains open
	State            CircuitBreakerState // Current state
	FailureCount     int                 // Failure counter
	SuccessCount     int                 // Success counter
	TotalCount       int                 // Total attempts counter
	LastStateChange  time.Time           // Last state change timestamp
	NextAttempt      time.Time           // Next attempt time after opening
	mu               sync.RWMutex        // Mutex for thread-safety
}

// Domain represents a domain that encapsulates queues and rules
type Domain struct {
	Name   string                             // Domain name
	Schema *Schema                            // Validation schema
	Queues map[string]*Queue                  // Map of queues by domainName
	Routes map[string]map[string]*RoutingRule // Map of routing rules (sourceQueue -> destQueue -> rule)
	System bool
}

// DomainConfig contains the configuration of a domain
type DomainConfig struct {
	Name         string                 // Domain name
	Schema       *Schema                // Validation schema
	QueueConfigs map[string]QueueConfig // Queue configurations
	RoutingRules []*RoutingRule         // Routing rules
}

type SchemaInfo struct {
	Fields map[string]string `json:"fields,omitempty"`
	// No Validation field since it's a function
	HasValidation bool `json:"hasValidation,omitempty"` // Optional, for information only
}

// SystemEvent represents a system event
type SystemEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`      // "info", "warning", "error"
	EventType string    `json:"eventType"` // "domain_active", "queue_capacity", "connection_lost"
	Resource  string    `json:"resource"`  // Name of the affected resource
	Data      any       `json:"data"`      // Additional data (e.g., capacity percentage)
	Timestamp time.Time `json:"-"`         // For internal use
	UnixTime  int64     `json:"timestamp"` // Unix timestamp for the client
}

// Schema defines the structure of messages for a domain
type Schema struct {
	// Fields defines the required fields in the payload
	Fields map[string]FieldType

	// Validation contains a custom validation function
	Validation func([]byte) error
}

// FieldType defines the type of a field in the schema
type FieldType string

const (
	StringType  FieldType = "string"
	NumberType  FieldType = "number"
	BooleanType FieldType = "boolean"
	ObjectType  FieldType = "object"
	ArrayType   FieldType = "array"
)

// RoutingRule defines a routing rule for messages
type RoutingRule struct {
	// SourceQueue is the source queue
	SourceQueue string

	// DestinationQueue is the target queue
	DestinationQueue string

	// Predicate is a function or object that determines if a message should be routed
	Predicate any
}

// PredicateFunc is a function that determines whether a message should be routed
type PredicateFunc func(*Message) bool

// JSONPredicate represents a predicate in JSON form for easier configuration
type JSONPredicate struct {
	Type  string `json:"type"`  // Operation type: eq, ne, gt, lt, etc.
	Field string `json:"field"` // Field to evaluate
	Value any    `json:"value"` // Value to compare
}

// Allow checks if an operation is allowed
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	now := time.Now()

	switch cb.State {
	case CircuitOpen:
		// If timeout has passed, switch to half-open
		if now.After(cb.NextAttempt) {
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.State = CircuitHalfOpen
			cb.FailureCount = 0
			cb.SuccessCount = 0
			cb.TotalCount = 0
			cb.LastStateChange = now
			cb.mu.Unlock()
			cb.mu.RLock()
			return true
		}
		return false

	case CircuitHalfOpen:
		// In half-open mode, limit the number of requests
		return cb.TotalCount < 5

	default: // CircuitClosed
		return true
	}
}

// Reset resets the circuit breaker
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.State = CircuitClosed
	cb.FailureCount = 0
	cb.SuccessCount = 0
	cb.TotalCount = 0
	cb.LastStateChange = time.Now()
}
