package model

import (
	"time"
)

// Message représente un message dans le système
type Message struct {
	ID        string                 // Identifiant unique du message
	Topic     string                 // Sujet du message
	Payload   []byte                 // Contenu du message
	Headers   map[string]string      // En-têtes du message
	Metadata  map[string]interface{} // Métadonnées pour le routage et le traitement
	Timestamp time.Time              // Horodatage de création du message
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
	IsPersistent bool

	// MaxSize définit la taille maximale de la file d'attente (0 = illimité)
	MaxSize int

	// TTL définit la durée de vie des messages (0 = illimité)
	TTL time.Duration

	// DeliveryMode définit le mode de livraison des messages
	DeliveryMode DeliveryMode
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
	Predicate interface{}
}

// PredicateFunc est une fonction qui détermine si un message doit être routé
type PredicateFunc func(*Message) bool

// JSONPredicate représente un prédicat sous forme de JSON pour faciliter la configuration
type JSONPredicate struct {
	Type  string      `json:"type"`  // Type d'opération: eq, ne, gt, lt, etc.
	Field string      `json:"field"` // Champ à évaluer
	Value interface{} `json:"value"` // Valeur à comparer
}
