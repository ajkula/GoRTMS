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

type QueueServiceImpl struct {
	rootCtx        context.Context
	logger         outbound.Logger
	domainRepo     outbound.DomainRepository
	statsService   inbound.StatsService
	channelQueues  map[string]map[string]*model.ChannelQueue // domainName -> queueName -> ChannelQueue
	messageService model.MessageProvider
	mu             sync.RWMutex
}

func NewQueueService(
	rootCtx context.Context,
	logger outbound.Logger,
	domainRepo outbound.DomainRepository,
	statsService inbound.StatsService,
) inbound.QueueService {
	svc := &QueueServiceImpl{
		rootCtx:       rootCtx,
		logger:        logger,
		domainRepo:    domainRepo,
		statsService:  statsService,
		channelQueues: make(map[string]map[string]*model.ChannelQueue),
	}

	// init existing queues
	go svc.initializeExistingQueues()

	return svc
}

func (s *QueueServiceImpl) SetMessageService(messageService model.MessageProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messageService = messageService
}

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

func (s *QueueServiceImpl) GetChannelQueue(ctx context.Context, domainName, queueName string) (model.QueueHandler, error) {
	domain, err := s.domainRepo.GetDomain(ctx, domainName)
	if err != nil {
		return nil, ErrDomainNotFound
	}

	queue, exists := domain.Queues[queueName]
	if !exists {
		return nil, ErrQueueNotFound
	}

	return s.getOrCreateChannelQueue(domainName, queue)
}

func (s *QueueServiceImpl) getOrCreateChannelQueue(domainName string, queue *model.Queue) (*model.ChannelQueue, error) {
	s.mu.RLock()
	if domainQueues, exists := s.channelQueues[domainName]; exists {
		if cq, exists := domainQueues[queue.Name]; exists {
			s.mu.RUnlock()
			return cq, nil
		}
	}
	s.mu.RUnlock()

	// create new channel queue
	s.mu.Lock()

	// Double-check in case another goroutine created it in the meantime
	if domainQueues, exists := s.channelQueues[domainName]; exists {
		if cq, exists := domainQueues[queue.Name]; exists {
			return cq, nil
		}
	} else {
		s.channelQueues[domainName] = make(map[string]*model.ChannelQueue)
	}

	bufferSize := 100 // [CHECK] todo: use config
	if queue.Config.MaxSize > 0 {
		bufferSize = queue.Config.MaxSize
	}

	cq := model.NewChannelQueue(s.rootCtx, s.logger, queue, bufferSize, s.messageService)
	s.channelQueues[domainName][queue.Name] = cq

	// start workers
	cq.Start(s.rootCtx)
	s.mu.Unlock()

	// Use a goroutine to log the event without blocking
	if s.statsService != nil {
		go func(ctx context.Context, domainName string) {
			// Use a context with timeout to avoid blocking
			timeoutCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()

			domain, err := s.domainRepo.GetDomain(timeoutCtx, domainName)
			if err == nil {
				queueCount := len(domain.Queues)
				s.statsService.RecordDomainActive(domainName, queueCount)
			}
		}(s.rootCtx, domainName) // Pass the root context, not Background()
	}

	return cq, nil
}

func (s *QueueServiceImpl) CreateQueue(ctx context.Context, domainName, queueName string, config *model.QueueConfig) error {
	log.Printf("Creating queue: %s.%s", domainName, queueName)

	domain, err := s.domainRepo.GetDomain(ctx, domainName)
	if err != nil {
		log.Printf("Error getting domain %s: %v", domainName, err)
		return ErrDomainNotFound
	}

	if domain.Queues != nil {
		if _, exists := domain.Queues[queueName]; exists {
			return ErrQueueAlreadyExists
		}
	} else {
		domain.Queues = make(map[string]*model.Queue)
	}

	queue := &model.Queue{
		Name:         queueName,
		DomainName:   domainName,
		Config:       *config,
		MessageCount: 0,
	}

	domain.Queues[queueName] = queue

	if domain.Routes == nil {
		domain.Routes = make(map[string]map[string]*model.RoutingRule)
	}

	// update domain
	log.Printf("Storing domain with new queue %s", queueName)
	if err := s.domainRepo.StoreDomain(ctx, domain); err != nil {
		log.Printf("Error storing domain %s: %v", domainName, err)
		return err
	}

	s.getOrCreateChannelQueue(domainName, queue)

	return nil
}

func (s *QueueServiceImpl) GetQueue(ctx context.Context, domainName, queueName string) (*model.Queue, error) {
	log.Printf("Getting queue: %s.%s", domainName, queueName)

	domain, err := s.domainRepo.GetDomain(ctx, domainName)
	if err != nil {
		return nil, ErrDomainNotFound
	}

	if domain.Queues == nil {
		return nil, ErrQueueNotFound
	}

	queue, exists := domain.Queues[queueName]
	if !exists {
		return nil, ErrQueueNotFound
	}

	return queue, nil
}

func (s *QueueServiceImpl) DeleteQueue(ctx context.Context, domainName, queueName string) error {
	log.Printf("Deleting queue: %s.%s", domainName, queueName)

	domain, err := s.domainRepo.GetDomain(ctx, domainName)
	if err != nil {
		return ErrDomainNotFound
	}

	if domain.Queues == nil || domain.Queues[queueName] == nil {
		return ErrQueueNotFound
	}

	// Stop ChannelQueue if it exists
	s.mu.Lock()
	if domainQueues, exists := s.channelQueues[domainName]; exists {
		if channelQueue, exists := domainQueues[queueName]; exists {
			// Release the mutex during the potentially long operation
			s.mu.Unlock()
			log.Printf("Stopping queue: %s.%s", domainName, queueName)
			channelQueue.Stop()
			log.Printf("Queue stopped: %s.%s", domainName, queueName)

			// Reacquire it to update the map
			s.mu.Lock()
			delete(domainQueues, queueName)
			if len(domainQueues) == 0 {
				delete(s.channelQueues, domainName)
			}
		}
	}
	s.mu.Unlock()

	// Delete queue
	delete(domain.Queues, queueName)

	// Remove associated routing rules
	if domain.Routes != nil {
		delete(domain.Routes, queueName)
		for srcQueue, destRoutes := range domain.Routes {
			delete(destRoutes, queueName)
			if len(destRoutes) == 0 {
				delete(domain.Routes, srcQueue)
			}
		}
	}

	queueCount := len(domain.Queues)
	if queueCount >= 0 && s.statsService != nil {
		s.statsService.RecordDomainActive(domainName, queueCount)
	}

	// update domain
	return s.domainRepo.StoreDomain(ctx, domain)
}

func (s *QueueServiceImpl) StopDomainQueues(ctx context.Context, domainName string) error {
	s.mu.Lock()
	queueMap, exists := s.channelQueues[domainName]
	if !exists {
		s.mu.Unlock()
		return nil
	}

	// Copy the keys to avoid modifying the map during iteration
	queueNames := make([]string, 0, len(queueMap))
	for qName := range queueMap {
		queueNames = append(queueNames, qName)
	}
	s.mu.Unlock()

	// stop each queue
	for _, qName := range queueNames {
		s.mu.Lock()
		cq, stillExists := queueMap[qName]
		s.mu.Unlock()

		if stillExists {
			log.Printf("Stopping queue for domain deletion: %s.%s", domainName, qName)
			cq.Stop()
		}
	}

	// Del all refs
	s.mu.Lock()
	delete(s.channelQueues, domainName)
	s.mu.Unlock()

	return nil
}

func (s *QueueServiceImpl) ListQueues(ctx context.Context, domainName string) ([]*model.Queue, error) {
	log.Printf("Listing queues for domain: %s", domainName)

	domain, err := s.domainRepo.GetDomain(ctx, domainName)
	if err != nil {
		return nil, ErrDomainNotFound
	}

	// Build the list of queues
	queues := make([]*model.Queue, 0)
	if domain.Queues != nil {
		for _, queue := range domain.Queues {
			queues = append(queues, queue)
		}
	}

	return queues, nil
}

func (s *QueueServiceImpl) Cleanup() {
	log.Println("Cleaning up queue service resources...")

	s.mu.Lock()
	// Copy the keys to avoid modifying the map during iteration
	domainKeys := make([]string, 0, len(s.channelQueues))
	for domainName := range s.channelQueues {
		domainKeys = append(domainKeys, domainName)
	}
	s.mu.Unlock()

	var wg sync.WaitGroup

	// concurrent kills
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
