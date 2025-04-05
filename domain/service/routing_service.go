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

// RoutingServiceImpl implémente le service de routage
type RoutingServiceImpl struct {
	domainRepo outbound.DomainRepository
}

// NewRoutingService crée un nouveau service de routage
func NewRoutingService(
	domainRepo outbound.DomainRepository,
) inbound.RoutingService {
	return &RoutingServiceImpl{
		domainRepo: domainRepo,
	}
}

// AddRoutingRule ajoute une règle de routage
func (s *RoutingServiceImpl) AddRoutingRule(ctx context.Context, domainName string, rule *model.RoutingRule) error {
	log.Printf("Adding routing rule in domain %s: %s -> %s", domainName, rule.SourceQueue, rule.DestinationQueue)

	// Récupérer le domaine
	domain, err := s.domainRepo.GetDomain(ctx, domainName)
	if err != nil {
		return ErrDomainNotFound
	}

	// Vérifier si les files d'attente existent
	if _, exists := domain.Queues[rule.SourceQueue]; !exists {
		return ErrQueueNotFound
	}
	if _, exists := domain.Queues[rule.DestinationQueue]; !exists {
		return ErrQueueNotFound
	}

	// Initialiser la structure des routes si nécessaire
	if domain.Routes == nil {
		domain.Routes = make(map[string]map[string]*model.RoutingRule)
	}

	// Initialiser la map des routes de source si nécessaire
	if _, exists := domain.Routes[rule.SourceQueue]; !exists {
		domain.Routes[rule.SourceQueue] = make(map[string]*model.RoutingRule)
	}

	// Vérifier si la règle existe déjà
	if _, exists := domain.Routes[rule.SourceQueue][rule.DestinationQueue]; exists {
		return ErrRoutingRuleAlreadyExists
	}

	// Ajouter la règle de routage
	domain.Routes[rule.SourceQueue][rule.DestinationQueue] = rule

	// Mettre à jour le domaine
	return s.domainRepo.StoreDomain(ctx, domain)
}

// RemoveRoutingRule supprime une règle de routage
func (s *RoutingServiceImpl) RemoveRoutingRule(ctx context.Context, domainName string, sourceQueue, destQueue string) error {
	log.Printf("Removing routing rule in domain %s: %s -> %s", domainName, sourceQueue, destQueue)

	// Récupérer le domaine
	domain, err := s.domainRepo.GetDomain(ctx, domainName)
	if err != nil {
		return ErrDomainNotFound
	}

	// Vérifier si la règle existe
	if domain.Routes == nil || domain.Routes[sourceQueue] == nil || domain.Routes[sourceQueue][destQueue] == nil {
		return ErrRoutingRuleNotFound
	}

	// Supprimer la règle
	delete(domain.Routes[sourceQueue], destQueue)

	// Si la map source est vide, la supprimer aussi
	if len(domain.Routes[sourceQueue]) == 0 {
		delete(domain.Routes, sourceQueue)
	}

	// Mettre à jour le domaine
	return s.domainRepo.StoreDomain(ctx, domain)
}

// ListRoutingRules liste toutes les règles de routage d'un domaine
func (s *RoutingServiceImpl) ListRoutingRules(ctx context.Context, domainName string) ([]*model.RoutingRule, error) {
	log.Printf("Listing routing rules for domain: %s", domainName)

	// Récupérer le domaine
	domain, err := s.domainRepo.GetDomain(ctx, domainName)
	if err != nil {
		return nil, ErrDomainNotFound
	}

	// Construire la liste des règles
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
