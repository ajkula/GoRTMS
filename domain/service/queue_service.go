package service

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/inbound"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

var (
	ErrQueueAlreadyExists = errors.New("queue already exists")
)

// QueueServiceImpl implémente le service des files d'attente
type QueueServiceImpl struct {
	domainRepo    outbound.DomainRepository
	channelQueues map[string]map[string]*model.ChannelQueue // domainName -> queueName -> ChannelQueue
	rootCtx       context.Context
	mu            sync.RWMutex
}

// NewQueueService crée un nouveau service de files d'attente
func NewQueueService(
	domainRepo outbound.DomainRepository,
	rootCtx context.Context,
) inbound.QueueService {
	svc := &QueueServiceImpl{
		domainRepo:    domainRepo,
		channelQueues: make(map[string]map[string]*model.ChannelQueue),
		rootCtx:       rootCtx,
	}

	// init les queues existantes
	go svc.initializeExistingQueues()

	return svc
}

// initializeExistingQueues initialise les channel queues pour les queues existantes
func (s *QueueServiceImpl) initializeExistingQueues() {
	ctx := context.Background()
	domains, err := s.domainRepo.ListDomains(ctx)
	if err != nil {
		log.Printf("Failed to list domains for queue initialization: %v", err)
		return
	}

	for _, domain := range domains {
		if domain.Queues == nil {
			continue
		}

		for _, queue := range domain.Queues {
			s.GetChannelQueue(ctx, domain.Name, queue.Name)
		}
	}
}

// GetChannelQueue récupère ou crée une ChannelQueue pour une file d'attente
func (s *QueueServiceImpl) GetChannelQueue(ctx context.Context, domainName, queueName string) (model.QueueHandler, error) {
	// Récupérer le domaine
	domain, err := s.domainRepo.GetDomain(ctx, domainName)
	if err != nil {
		return nil, ErrDomainNotFound
	}

	// Vérifier si la file d'attente existe
	queue, exists := domain.Queues[queueName]
	if !exists {
		return nil, ErrQueueNotFound
	}

	// Obtenir ou créer la channel queue
	return s.getOrCreateChannelQueue(domainName, queue)
}

// GetChannelQueue récupère ou créé une channel queue
func (s *QueueServiceImpl) getOrCreateChannelQueue(domainName string, queue *model.Queue) (*model.ChannelQueue, error) {
	s.mu.RLock()
	if domainQueues, exists := s.channelQueues[domainName]; exists {
		if cq, exists := domainQueues[queue.Name]; exists {
			s.mu.RUnlock()
			return cq, nil
		}
	}
	s.mu.RUnlock()

	// Créer une nouvelle channel queue
	s.mu.Lock()
	defer s.mu.Unlock()

	// Vérifier à nouveau au cas où une autre goroutine l'aurait créée entre-temps
	if domainQueues, exists := s.channelQueues[domainName]; exists {
		if cq, exists := domainQueues[queue.Name]; exists {
			return cq, nil
		}
	} else {
		// Initialiser la map si elle n'existe pas
		s.channelQueues[domainName] = make(map[string]*model.ChannelQueue)
	}

	bufferSize := 100 // default
	if queue.Config.MaxSize > 0 {
		bufferSize = queue.Config.MaxSize
	}

	// Créer et démarrer la nouvelle queue
	cq := model.NewChannelQueue(queue, s.rootCtx, bufferSize)
	s.channelQueues[domainName][queue.Name] = cq

	// Démarrer les workers
	cq.Start(s.rootCtx)

	return cq, nil
}

// CreateQueue crée une nouvelle file d'attente dans un domaine
func (s *QueueServiceImpl) CreateQueue(ctx context.Context, domainName, queueName string, config *model.QueueConfig) error {
	log.Printf("Creating queue: %s.%s", domainName, queueName)

	// Récupérer le domaine
	domain, err := s.domainRepo.GetDomain(ctx, domainName)
	if err != nil {
		log.Printf("Error getting domain %s: %v", domainName, err)
		return ErrDomainNotFound
	}

	// Vérifier si la file d'attente existe déjà
	if domain.Queues != nil {
		if _, exists := domain.Queues[queueName]; exists {
			return ErrQueueAlreadyExists
		}
	} else {
		domain.Queues = make(map[string]*model.Queue)
	}

	// Créer la file d'attente
	queue := &model.Queue{
		Name:         queueName,
		DomainName:   domainName,
		Config:       *config,
		MessageCount: 0,
	}

	// Ajouter la file d'attente au domaine
	domain.Queues[queueName] = queue

	// Vérifier que Routes est initialisé
	if domain.Routes == nil {
		domain.Routes = make(map[string]map[string]*model.RoutingRule)
	}

	// Mettre à jour le domaine
	log.Printf("Storing domain with new queue %s", queueName)
	if err := s.domainRepo.StoreDomain(ctx, domain); err != nil {
		log.Printf("Error storing domain %s: %v", domainName, err)
		return err
	}

	s.getOrCreateChannelQueue(domainName, queue)

	return nil
}

// graceful close les channel queues
func (s *QueueServiceImpl) Cleanup() {
	log.Println("Cleaning up queue service resources...")

	s.mu.Lock()
	defer s.mu.Unlock()

	var wg sync.WaitGroup

	for domainName, domainQueues := range s.channelQueues {
		for queueName, cq := range domainQueues {
			wg.Add(1)
			go func(d, q string, queue *model.ChannelQueue) {
				defer wg.Done()
				log.Printf("Stopping queue: %s.%s", d, q)
				queue.Stop()
				log.Printf("Queue stopped: %s.%s", d, q)
			}(domainName, queueName, cq)
		}
	}

	// Attendre avectimeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("All queues cleanly stopped")
	case <-time.After(10 * time.Second):
		log.Println("Timeout waiting forqueues to stop, forcing shutdown")
	}
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
	if domain.Queues == nil {
		return nil, ErrQueueNotFound
	}

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
	if domain.Queues == nil || domain.Queues[queueName] == nil {
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
	queues := make([]*model.Queue, 0)
	if domain.Queues != nil {
		for _, queue := range domain.Queues {
			queues = append(queues, queue)
		}
	}

	return queues, nil
}
