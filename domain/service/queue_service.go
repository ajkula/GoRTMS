package service

import (
	"context"
	"errors"
	"log"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/inbound"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

var (
	ErrQueueAlreadyExists = errors.New("queue already exists")
)

// QueueServiceImpl implémente le service des files d'attente
type QueueServiceImpl struct {
	domainRepo outbound.DomainRepository
}

// NewQueueService crée un nouveau service de files d'attente
func NewQueueService(
	domainRepo outbound.DomainRepository,
) inbound.QueueService {
	return &QueueServiceImpl{
		domainRepo: domainRepo,
	}
}

// CreateQueue crée une nouvelle file d'attente dans un domaine
func (s *QueueServiceImpl) CreateQueue(ctx context.Context, domainName, queueName string, config *model.QueueConfig) error {
	log.Printf("Creating queue: %s.%s", domainName, queueName)

	// Récupérer le domaine
	domain, err := s.domainRepo.GetDomain(ctx, domainName)
	if err != nil {
		return ErrDomainNotFound
	}

	// Vérifier si la file d'attente existe déjà
	if _, exists := domain.Queues[queueName]; exists {
		return ErrQueueAlreadyExists
	}

	// Créer la file d'attente
	queue := &model.Queue{
		Name:         queueName,
		DomainName:   domainName,
		Config:       *config,
		MessageCount: 0,
	}

	// Ajouter la file d'attente au domaine
	if domain.Queues == nil {
		domain.Queues = make(map[string]*model.Queue)
	}
	domain.Queues[queueName] = queue

	// Mettre à jour le domaine
	return s.domainRepo.StoreDomain(ctx, domain)
}

// GetQueue récupère une file d'attente
func (s *QueueServiceImpl) GetQueue(ctx context.Context, domainName, queueName string) (*model.Queue, error) {
	log.Printf("Getting queue: %s.%s", domainName, queueName)

	// Récupérer le domaine
	domain, err := s.domainRepo.GetDomain(ctx, domainName)
	if err != nil {
		return nil, ErrDomainNotFound
	}

	// Récupérer la file d'attente
	queue, exists := domain.Queues[queueName]
	if !exists {
		return nil, ErrQueueNotFound
	}

	return queue, nil
}

// DeleteQueue supprime une file d'attente
func (s *QueueServiceImpl) DeleteQueue(ctx context.Context, domainName, queueName string) error {
	log.Printf("Deleting queue: %s.%s", domainName, queueName)

	// Récupérer le domaine
	domain, err := s.domainRepo.GetDomain(ctx, domainName)
	if err != nil {
		return ErrDomainNotFound
	}

	// Vérifier si la file d'attente existe
	if _, exists := domain.Queues[queueName]; !exists {
		return ErrQueueNotFound
	}

	// Supprimer la file d'attente
	delete(domain.Queues, queueName)

	// Supprimer les règles de routage associées
	if domain.Routes != nil {
		delete(domain.Routes, queueName)
		for srcQueue, destRoutes := range domain.Routes {
			delete(destRoutes, queueName)
			// Si la map est vide, la supprimer aussi
			if len(destRoutes) == 0 {
				delete(domain.Routes, srcQueue)
			}
		}
	}

	// Mettre à jour le domaine
	return s.domainRepo.StoreDomain(ctx, domain)
}

// ListQueues liste toutes les files d'attente d'un domaine
func (s *QueueServiceImpl) ListQueues(ctx context.Context, domainName string) ([]*model.Queue, error) {
	log.Printf("Listing queues for domain: %s", domainName)

	// Récupérer le domaine
	domain, err := s.domainRepo.GetDomain(ctx, domainName)
	if err != nil {
		return nil, ErrDomainNotFound
	}

	// Construire la liste des files d'attente
	queues := make([]*model.Queue, 0, len(domain.Queues))
	for _, queue := range domain.Queues {
		queues = append(queues, queue)
	}

	return queues, nil
}
