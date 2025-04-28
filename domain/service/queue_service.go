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
	statsService  inbound.StatsService
	channelQueues map[string]map[string]*model.ChannelQueue // domainName -> queueName -> ChannelQueue
	rootCtx       context.Context
	mu            sync.RWMutex
}

// NewQueueService crée un nouveau service de files d'attente
func NewQueueService(
	domainRepo outbound.DomainRepository,
	statsService inbound.StatsService,
	rootCtx context.Context,
) inbound.QueueService {
	svc := &QueueServiceImpl{
		domainRepo:    domainRepo,
		statsService:  statsService,
		channelQueues: make(map[string]map[string]*model.ChannelQueue),
		rootCtx:       rootCtx,
	}

	// init les queues existantes
	go svc.initializeExistingQueues()

	return svc
}

// initializeExistingQueues initialise les channel queues pour les queues existantes
func (s *QueueServiceImpl) initializeExistingQueues() {
	domains, err := s.domainRepo.ListDomains(s.rootCtx)
	if err != nil {
		log.Printf("Failed to list domains for queue initialization: %v", err)
		return
	}

	for _, domain := range domains {
		if domain.Queues == nil {
			continue
		}

		for _, queue := range domain.Queues {
			s.GetChannelQueue(s.rootCtx, domain.Name, queue.Name)
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
	s.mu.Unlock()

	// Utiliser une goroutine pour enregistrer l'événement sans bloquer
	if s.statsService != nil {
		go func(ctx context.Context, domainName string) {
			// Utiliser un contexte avec timeout pour éviter les blocages
			timeoutCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()

			domain, err := s.domainRepo.GetDomain(timeoutCtx, domainName)
			if err == nil {
				queueCount := len(domain.Queues)
				s.statsService.RecordDomainActive(domainName, queueCount)
			}
		}(s.rootCtx, domainName) // Passer le contexte racine, pas Background()
	}

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

	// Arrêter la ChannelQueue si elle existe
	s.mu.Lock()
	if domainQueues, exists := s.channelQueues[domainName]; exists {
		if channelQueue, exists := domainQueues[queueName]; exists {
			// On libère le mutex pendant l'opération potentiellement longue
			s.mu.Unlock()
			log.Printf("Stopping queue: %s.%s", domainName, queueName)
			channelQueue.Stop()
			log.Printf("Queue stopped: %s.%s", domainName, queueName)

			// On le reprend pour mettre à jour la map
			s.mu.Lock()
			delete(domainQueues, queueName)
			if len(domainQueues) == 0 {
				delete(s.channelQueues, domainName)
			}
		}
	}
	s.mu.Unlock()

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

	queueCount := len(domain.Queues)
	if queueCount >= 0 && s.statsService != nil {
		s.statsService.RecordDomainActive(domainName, queueCount)
	}

	// Mettre à jour le domaine
	return s.domainRepo.StoreDomain(ctx, domain)
}

// StopDomainQueues arrête toutes les queues d'un domaine
func (s *QueueServiceImpl) StopDomainQueues(ctx context.Context, domainName string) error {
	s.mu.Lock()
	queueMap, exists := s.channelQueues[domainName]
	if !exists {
		s.mu.Unlock()
		return nil // Pas de queues pour ce domaine
	}

	// Copier les clés pour éviter de modifier la map pendant l'itération
	queueNames := make([]string, 0, len(queueMap))
	for qName := range queueMap {
		queueNames = append(queueNames, qName)
	}
	s.mu.Unlock()

	// Arrêter chaque queue
	for _, qName := range queueNames {
		s.mu.Lock()
		cq, stillExists := queueMap[qName]
		s.mu.Unlock()

		if stillExists {
			log.Printf("Stopping queue for domain deletion: %s.%s", domainName, qName)
			cq.Stop()
		}
	}

	// Supprimer toutes les références
	s.mu.Lock()
	delete(s.channelQueues, domainName)
	s.mu.Unlock()

	return nil
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

// graceful close les channel queues
func (s *QueueServiceImpl) Cleanup() {
	log.Println("Cleaning up queue service resources...")

	s.mu.Lock()
	// Copier les clés pour éviter de modifier la map pendant l'itération
	domainKeys := make([]string, 0, len(s.channelQueues))
	for domainName := range s.channelQueues {
		domainKeys = append(domainKeys, domainName)
	}
	s.mu.Unlock()

	var wg sync.WaitGroup

	// Arrêter chaque queue en parallèle
	for _, domainName := range domainKeys {
		s.mu.RLock()
		queueMap := s.channelQueues[domainName]
		queueKeys := make([]string, 0, len(queueMap))
		for queueName := range queueMap {
			queueKeys = append(queueKeys, queueName)
		}
		s.mu.RUnlock()

		for _, queueName := range queueKeys {
			s.mu.RLock()
			queue, exists := s.channelQueues[domainName][queueName]
			s.mu.RUnlock()

			if exists {
				wg.Add(1)
				go func(d, q string, cq *model.ChannelQueue) {
					defer wg.Done()
					log.Printf("Stopping queue: %s.%s", d, q)
					cq.Stop()
					log.Printf("Queue stopped: %s.%s", d, q)
				}(domainName, queueName, queue)
			}
		}
	}

	// Attendre avec timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("All queues cleanly stopped")
	case <-time.After(10 * time.Second):
		log.Println("Timeout waiting for queues to stop, forcing shutdown")
	}
}
