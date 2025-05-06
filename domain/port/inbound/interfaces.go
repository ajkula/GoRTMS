package inbound

import (
	"context"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
)

// ConsumeOptions définit les options pour la consommation
type ConsumeOptions struct {
	ResetOffset bool
	StartFromID string
	ConsumerID  string
	Timeout     time.Duration
}

// MessageService définit les opérations sur les messages
type MessageService interface {
	// PublishMessage publie un message dans une file d'attente
	PublishMessage(domainName, queueName string, message *model.Message) error

	// ConsumeMessage consomme un message d'une file d'attente
	ConsumeMessage(domainName, queueName string) (*model.Message, error)

	// SubscribeToQueue s'abonne à une file d'attente
	SubscribeToQueue(domainName, queueName string, handler model.MessageHandler) (string, error)

	// UnsubscribeFromQueue se désinscrit d'une file d'attente
	UnsubscribeFromQueue(domainName, queueName string, subscriptionID string) error

	// ConsumeMessageWithGroup consomme un message avec gestion des offsets
	ConsumeMessageWithGroup(ctx context.Context,
		domainName, queueName, groupID string, options *ConsumeOptions,
	) (*model.Message, error)
}

// DomainService définit les opérations sur les domaines
type DomainService interface {
	// CreateDomain crée un nouveau domaine
	CreateDomain(ctx context.Context, config *model.DomainConfig) error

	// GetDomain récupère un domaine existant
	GetDomain(ctx context.Context, name string) (*model.Domain, error)

	// DeleteDomain supprime un domaine
	DeleteDomain(ctx context.Context, name string) error

	// ListDomains liste tous les domaines
	ListDomains(ctx context.Context) ([]*model.Domain, error)
}

// QueueService définit les opérations sur les files d'attente
type QueueService interface {
	// CreateQueue crée une nouvelle file d'attente
	CreateQueue(ctx context.Context, domainName, queueName string, config *model.QueueConfig) error

	// GetQueue récupère une file d'attente existante
	GetQueue(ctx context.Context, domainName, queueName string) (*model.Queue, error)

	// DeleteQueue supprime une file d'attente
	DeleteQueue(ctx context.Context, domainName, queueName string) error

	// ListQueues liste toutes les files d'attente d'un domaine
	ListQueues(ctx context.Context, domainName string) ([]*model.Queue, error)

	// GetChannelQueue récupère ou crée une ChannelQueue pour une file d'attente existante
	GetChannelQueue(ctx context.Context, domainName, queueName string) (model.QueueHandler, error)

	//StopDomainQueues arrête toutes les queues d'un domaine
	StopDomainQueues(ctx context.Context, domainName string) error

	// Cleanup nettoie les ressources utilisées par le service
	Cleanup()
}

// RoutingService définit les opérations sur les règles de routage
type RoutingService interface {
	// AddRoutingRule ajoute une règle de routage
	AddRoutingRule(ctx context.Context, domainName string, rule *model.RoutingRule) error

	// RemoveRoutingRule supprime une règle de routage
	RemoveRoutingRule(ctx context.Context, domainName string, sourceQueue, destQueue string) error

	// ListRoutingRules liste toutes les règles de routage d'un domaine
	ListRoutingRules(ctx context.Context, domainName string) ([]*model.RoutingRule, error)
}
