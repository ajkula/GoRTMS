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
	ErrRoutingRuleAlreadyExists = errors.New("routing rule already exists")
	ErrRoutingRuleNotFound      = errors.New("routing rule not found")
)

type RoutingServiceImpl struct {
	domainRepo outbound.DomainRepository
	rootCtx    context.Context
}

func NewRoutingService(
	domainRepo outbound.DomainRepository,
	rootCtx context.Context,
) inbound.RoutingService {
	return &RoutingServiceImpl{
		domainRepo: domainRepo,
		rootCtx:    rootCtx,
	}
}

func (s *RoutingServiceImpl) AddRoutingRule(ctx context.Context, domainName string, rule *model.RoutingRule) error {
	log.Printf("Adding routing rule in domain %s: %s -> %s", domainName, rule.SourceQueue, rule.DestinationQueue)

	domain, err := s.domainRepo.GetDomain(ctx, domainName)
	if err != nil {
		return ErrDomainNotFound
	}

	if _, exists := domain.Queues[rule.SourceQueue]; !exists {
		return ErrQueueNotFound
	}
	if _, exists := domain.Queues[rule.DestinationQueue]; !exists {
		return ErrQueueNotFound
	}

	if domain.Routes == nil {
		domain.Routes = make(map[string]map[string]*model.RoutingRule)
	}

	// Initialize the source routes map if necessary
	if _, exists := domain.Routes[rule.SourceQueue]; !exists {
		domain.Routes[rule.SourceQueue] = make(map[string]*model.RoutingRule)
	}

	if _, exists := domain.Routes[rule.SourceQueue][rule.DestinationQueue]; exists {
		return ErrRoutingRuleAlreadyExists
	}

	domain.Routes[rule.SourceQueue][rule.DestinationQueue] = rule

	return s.domainRepo.StoreDomain(ctx, domain)
}

func (s *RoutingServiceImpl) RemoveRoutingRule(ctx context.Context, domainName string, sourceQueue, destQueue string) error {
	log.Printf("Removing routing rule in domain %s: %s -> %s", domainName, sourceQueue, destQueue)

	domain, err := s.domainRepo.GetDomain(ctx, domainName)
	if err != nil {
		return ErrDomainNotFound
	}

	if domain.Routes == nil || domain.Routes[sourceQueue] == nil || domain.Routes[sourceQueue][destQueue] == nil {
		return ErrRoutingRuleNotFound
	}

	delete(domain.Routes[sourceQueue], destQueue)

	// If the source map is empty, remove it as well
	if len(domain.Routes[sourceQueue]) == 0 {
		delete(domain.Routes, sourceQueue)
	}

	return s.domainRepo.StoreDomain(ctx, domain)
}

func (s *RoutingServiceImpl) ListRoutingRules(ctx context.Context, domainName string) ([]*model.RoutingRule, error) {
	log.Printf("Listing routing rules for domain: %s", domainName)

	domain, err := s.domainRepo.GetDomain(ctx, domainName)
	if err != nil {
		return nil, ErrDomainNotFound
	}

	// Build the list of rules
	rules := make([]*model.RoutingRule, 0)
	if domain.Routes != nil {
		for _, sourceRules := range domain.Routes {
			for _, rule := range sourceRules {
				rules = append(rules, rule)
			}
		}
	}

	return rules, nil
}

func (s *RoutingServiceImpl) Cleanup() {
	log.Println("Cleaning up routing service resources...")
	// noop
}
