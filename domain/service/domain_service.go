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
	ErrDomainAlreadyExists = errors.New("domain already exists")
)

type DomainServiceImpl struct {
	domainRepo   outbound.DomainRepository
	queueService inbound.QueueService
	rootCtx      context.Context
}

func NewDomainService(
	domainRepo outbound.DomainRepository,
	queueService inbound.QueueService,
	rootCtx context.Context,
) inbound.DomainService {
	return &DomainServiceImpl{
		domainRepo:   domainRepo,
		queueService: queueService,
		rootCtx:      rootCtx,
	}
}

func (s *DomainServiceImpl) CreateDomain(ctx context.Context, config *model.DomainConfig) error {
	log.Printf("Creating domain: %s", config.Name)

	existingDomain, err := s.domainRepo.GetDomain(ctx, config.Name)
	if err == nil && existingDomain != nil {
		return ErrDomainAlreadyExists
	}

	domain := &model.Domain{
		Name:   config.Name,
		Schema: config.Schema,
		Queues: make(map[string]*model.Queue),
		Routes: make(map[string]map[string]*model.RoutingRule),
	}

	// If set create initial queues
	if config.QueueConfigs != nil {
		for queueName, queueConfig := range config.QueueConfigs {
			domain.Queues[queueName] = &model.Queue{
				Name:         queueName,
				DomainName:   config.Name,
				Config:       queueConfig,
				MessageCount: 0,
			}
		}
	}

	// if set add routing rules
	if config.RoutingRules != nil {
		for _, rule := range config.RoutingRules {
			if domain.Routes[rule.SourceQueue] == nil {
				domain.Routes[rule.SourceQueue] = make(map[string]*model.RoutingRule)
			}
			domain.Routes[rule.SourceQueue][rule.DestinationQueue] = rule
		}
	}

	return s.domainRepo.StoreDomain(ctx, domain)
}

func (s *DomainServiceImpl) GetDomain(ctx context.Context, name string) (*model.Domain, error) {
	log.Printf("Getting domain: %s", name)
	return s.domainRepo.GetDomain(ctx, name)
}

func (s *DomainServiceImpl) DeleteDomain(ctx context.Context, name string) error {
	log.Printf("Deleting domain: %s", name)

	_, err := s.domainRepo.GetDomain(ctx, name)
	if err != nil {
		return ErrDomainNotFound
	}

	s.queueService.StopDomainQueues(ctx, name)

	return s.domainRepo.DeleteDomain(ctx, name)
}

func (s *DomainServiceImpl) ListDomains(ctx context.Context) ([]*model.Domain, error) {
	log.Println("Listing domains")
	return s.domainRepo.ListDomains(ctx)
}

func (s *DomainServiceImpl) Cleanup() {
	log.Println("Cleaning up domain service resources...")
	// noop
}
