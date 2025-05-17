package outbound

import (
	"context"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
)

// MessageRepository définit les opérations de stockage pour les messages
type MessageRepository interface {
	// StoreMessage stocke un message
	StoreMessage(ctx context.Context, domainName, queueName string, message *model.Message) error

	// GetMessage récupère un message par son ID
	GetMessage(ctx context.Context, domainName, queueName, messageID string) (*model.Message, error)

	// DeleteMessage supprime un message
	DeleteMessage(ctx context.Context, domainName, queueName, messageID string) error

	// GetMessagesAfterIndex récupère les messages à partir d'un index donné
	// Si startIndex=0, comportement équivalent à l'ancienne méthode GetMessages
	GetMessagesAfterIndex(
		ctx context.Context,
		domainName, queueName string, startIndex int64,
		limit int,
	) ([]*model.Message, error)
	GetIndexByMessageID(ctx context.Context, domainName, queueName, messageID string) (int64, error)

	// GetOrCreateAckMatrix récupère ou crée une matrice d'acquittement pour une queue
	GetOrCreateAckMatrix(domainName, queueName string) *model.AckMatrix

	// AcknowledgeMessage marque un message comme acquitté par un groupe
	// Retourne true si le message est entièrement acquitté par tous les groupes
	AcknowledgeMessage(
		ctx context.Context,
		domainName, queueName, groupID, messageID string,
	) (bool, error)

	// ClearQueueIndices nettoie toutes les références d'index pour une queue spécifique
	ClearQueueIndices(
		ctx context.Context,
		domainName, queueName string,
	)
	CleanupMessageIndices(
		ctx context.Context,
		domainName, queueName string,
		minPosition int64,
	)
}

// DomainRepository définit les opérations de stockage pour les domaines
type DomainRepository interface {
	// StoreDomain stocke un domaine
	StoreDomain(ctx context.Context, domain *model.Domain) error

	// GetDomain récupère un domaine par son nom
	GetDomain(ctx context.Context, name string) (*model.Domain, error)

	// DeleteDomain supprime un domaine
	DeleteDomain(ctx context.Context, name string) error

	// ListDomains liste tous les domaines
	ListDomains(ctx context.Context) ([]*model.Domain, error)
}

// QueueRepository définit les opérations de stockage pour les files d'attente
type QueueRepository interface {
	// StoreQueue stocke une file d'attente
	StoreQueue(ctx context.Context, domainName string, queue *model.Queue) error

	// GetQueue récupère une file d'attente par son nom
	GetQueue(ctx context.Context, domainName, queueName string) (*model.Queue, error)

	// DeleteQueue supprime une file d'attente
	DeleteQueue(ctx context.Context, domainName, queueName string) error

	// ListQueues liste toutes les files d'attente d'un domaine
	ListQueues(ctx context.Context, domainName string) ([]*model.Queue, error)
}

// SubscriptionRegistry définit les opérations pour gérer les abonnements
type SubscriptionRegistry interface {
	// RegisterSubscription enregistre un nouvel abonnement
	RegisterSubscription(domainName, queueName string, handler model.MessageHandler) (string, error)

	// UnregisterSubscription supprime un abonnement
	UnregisterSubscription(subscriptionID string) error

	// NotifySubscribers notifie tous les abonnés d'un message
	NotifySubscribers(domainName, queueName string, message *model.Message) error
}

// ConsumerGroupRepository définit les opérations pour les groupes
type ConsumerGroupRepository interface {
	//StoreOffset enregistre unoffset pour un groupe
	StorePosition(ctx context.Context, domainName, queueNamme, groupID string, index int64) error

	// GetOffset récup. le dernier offset d'un group
	GetPosition(ctx context.Context, domainName, queueName, groupID string) (int64, error)

	// RegisterConsumer enregistre unconsumer dans un groupe
	RegisterConsumer(ctx context.Context, domainName, queueName, groupID, consumerID string) error

	// RemoveConsumer supprime un consumerd'un groupe
	RemoveConsumer(ctx context.Context, domainName, queueName, groupID, consumerID string) error

	// ListGroups liste tous les groupes pour une queue
	ListGroups(ctx context.Context, domainName, queueName string) ([]string, error)

	CleanupStaleGroups(ctx context.Context, olderThan time.Duration) error
}
