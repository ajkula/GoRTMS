package model

import (
	"context"
	"sync"
	"time"
)

// Message représente un message dans le système
type Message struct {
	ID        string            // Identifiant unique du message
	Topic     string            // Sujet du message
	Payload   []byte            // Contenu du message
	Headers   map[string]string // En-têtes du message
	Metadata  map[string]any    // Métadonnées pour le routage et le traitement
	Timestamp time.Time         // Horodatage de création du message
}

// MessageHandler est une fonction de rappel pour traiter les messages
type MessageHandler func(*Message) error

// Queue représente une file d'attente de messages
type Queue struct {
	Name         string      // Nom de la file
	DomainName   string      // Nom du domaine parent
	Config       QueueConfig // Configuration de la file
	MessageCount int         // Nombre de messages dans la file
}

// QueueConfig contient la configuration d'une file d'attente
type QueueConfig struct {
	// IsPersistent indique si les messages doivent être persistés
	IsPersistent bool `yaml:"isPersistent"`

	// MaxSize définit la taille maximale de la file d'attente (0 = illimité)
	MaxSize int `yaml:"maxSize"`

	// TTL définit la durée de vie des messages (0 = illimité)
	TTL time.Duration `yaml:"ttl"`

	// DeliveryMode définit le mode de livraison des messages
	DeliveryMode DeliveryMode `yaml:"deliveryMode"`

	// Nouveaux champs
	WorkerCount int `yaml:"workerCount"`

	// RetryEnabled active le mécanisme de retry
	RetryEnabled bool `yaml:"retryEnabled"`

	// RetryConfig définit la config. des retries
	RetryConfig *RetryConfig `yaml:"retryConfig,omitempty"`

	// CircuitBreakerEnabled active le circuit breaker
	CircuitBreakerEnabled bool                  `yaml:"circuitBreakerEnabled"`
	CircuitBreakerConfig  *CircuitBreakerConfig `yaml:"circuitBreakerConfig,omitempty"`
}

// CircuitBreakerConfig définit la configuration du circuit breaker
type CircuitBreakerConfig struct {
	ErrorThreshold   float64       `yaml:"errorThreshold"`
	MinimumRequests  int           `yaml:"minimumRequests"`
	OpenTimeout      time.Duration `yaml:"openTimeout"`
	SuccessThreshold int           `yaml:"successThreshold"`
}

// QueueHandler définit les opérations pour une implémentation de queue concurrente
type QueueHandler interface {
	// Enqueue ajoute un message à la queue de manière thread-safe
	Enqueue(ctx context.Context, message *Message) error

	// Dequeue récupère un message de la queue de manière thread-safe
	Dequeue(ctx context.Context) (*Message, error)

	// GetQueue retourne la structure Queue associée
	GetQueue() *Queue

	// Start démarre les workers pour traiter les messages
	Start(ctx context.Context)

	// Stop arrête les workers et libère les ressources
	Stop()
}

// RetryConfig définit laconfig desretentatives pour les messages échoués
type RetryConfig struct {
	MaxRetries int

	// définit le délai avant la première tentative
	InitialDelay time.Duration

	// Delai max entre tentatives
	MaxDelay time.Duration

	// Factor définit lefacteurde multiplication pour le backoff exponentiel
	Factor float64
}

// MessageWithRetry représente un message avec des informations de retry
type MessageWithRetry struct {
	Message     *Message
	RetryCount  int
	NextRetryAt time.Time
	Handler     MessageHandler
}

type CircuitBreakerState int

const (
	// CircuitClosed= circuitfermé,lesmsg passentnormalement
	CircuitClosed CircuitBreakerState = iota

	// CircuitOpen = circuit ouvert, les msg sont rejetés
	CircuitOpen

	// CircuitHalfOpen = circuit en mode test
	CircuitHalfOpen
)

// CircuitBreaker implémente lepatterndu même nom pour protéger contre les surcharges
type CircuitBreaker struct {
	ErrorThreshold   float64             // Seuil d'erreurs pour ouvrir
	SuccessThreshold int                 // nbr de succès pour fermer
	MinimumRequests  int                 // Nombre min de req avant d'appliquer
	OpenTimeout      time.Duration       // Durée d'ouverture
	State            CircuitBreakerState // Etat actuel
	FailureCount     int                 // compteur d'échecs
	SuccessCount     int                 // Compteur de succès
	TotalCount       int                 // Compteur total
	LastStateChange  time.Time           // Derniere modif. d'état
	NextAttempt      time.Time           // Prochaine tentative après ouverture
	mu               sync.RWMutex        // Mutex de thread-safety
}

// DeliveryMode définit comment les messages sont distribués aux consommateurs
type DeliveryMode int

const (
	// BroadcastMode envoie le message à tous les consommateurs
	BroadcastMode DeliveryMode = iota

	// RoundRobinMode distribue les messages de manière équilibrée entre les consommateurs
	RoundRobinMode

	// SingleConsumerMode n'envoie le message qu'à un seul consommateur
	SingleConsumerMode
)

// Domain représente un domaine qui encapsule des files d'attente et des règles
type Domain struct {
	Name   string                             // Nom du domaine
	Schema *Schema                            // Schéma de validation
	Queues map[string]*Queue                  // Map des files d'attente
	Routes map[string]map[string]*RoutingRule // Map des règles de routage (sourceQueue -> destQueue -> rule)
}

// DomainConfig contient la configuration d'un domaine
type DomainConfig struct {
	Name         string                 // Nom du domaine
	Schema       *Schema                // Schéma de validation
	QueueConfigs map[string]QueueConfig // Configurations des files d'attente
	RoutingRules []*RoutingRule         // Règles de routage
}

type SchemaInfo struct {
	Fields map[string]string `json:"fields,omitempty"`
	// Pas de champ Validation car c'est une fonction
	HasValidation bool `json:"hasValidation,omitempty"` // Optionnel, juste pour information
}

// SystemEvent représente un événement système
type SystemEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`      // "info", "warning", "error"
	EventType string    `json:"eventType"` // "domain_active", "queue_capacity", "connection_lost"
	Resource  string    `json:"resource"`  // Le nom de la ressource concernée
	Data      any       `json:"data"`      // Données supplémentaires (comme le pourcentage de capacité)
	Timestamp time.Time `json:"-"`         // Pour usage interne
	UnixTime  int64     `json:"timestamp"` // Timestamp Unix pour le client
}

// Schema définit la structure des messages pour un domaine
type Schema struct {
	// Fields définit les champs obligatoires dans le payload
	Fields map[string]FieldType

	// Validation contient une fonction de validation personnalisée
	Validation func([]byte) error
}

// FieldType définit le type d'un champ dans le schéma
type FieldType string

const (
	StringType  FieldType = "string"
	NumberType  FieldType = "number"
	BooleanType FieldType = "boolean"
	ObjectType  FieldType = "object"
	ArrayType   FieldType = "array"
)

// RoutingRule définit une règle de routage pour les messages
type RoutingRule struct {
	// SourceQueue est la file d'attente source
	SourceQueue string

	// DestinationQueue est la file d'attente destination
	DestinationQueue string

	// Predicate est une fonction ou un objet qui détermine si un message doit être routé
	Predicate any
}

// PredicateFunc est une fonction qui détermine si un message doit être routé
type PredicateFunc func(*Message) bool

// JSONPredicate représente un prédicat sous forme de JSON pour faciliter la configuration
type JSONPredicate struct {
	Type  string `json:"type"`  // Type d'opération: eq, ne, gt, lt, etc.
	Field string `json:"field"` // Champ à évaluer
	Value any    `json:"value"` // Valeur à comparer
}

// Allow vérifie si une opération peut être exécutée
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	now := time.Now()

	switch cb.State {
	case CircuitOpen:
		// Si le timeout est passé, passer en mode semi-ouvert
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
		// En mode semi-ouvert, limiter le nombre de requêtes
		return cb.TotalCount < 5

	default: // CircuitClosed
		return true
	}
}

// Reset réinitialise le circuit breaker
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.State = CircuitClosed
	cb.FailureCount = 0
	cb.SuccessCount = 0
	cb.TotalCount = 0
	cb.LastStateChange = time.Now()
}
