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

// DomainServiceImpl implémente le service des domaines
type DomainServiceImpl struct {
	domainRepo outbound.DomainRepository
}

// NewDomainService crée un nouveau service de domaines
func NewDomainService(
	domainRepo outbound.DomainRepository,
) inbound.DomainService {
	return &DomainServiceImpl{
		domainRepo: domainRepo,
	}
}

// CreateDomain crée un nouveau domaine
func (s *DomainServiceImpl) CreateDomain(ctx context.Context, config *model.DomainConfig) error {
	log.Printf("Creating domain: %s", config.Name)

	// Vérifier si le domaine existe déjà
	existingDomain, err := s.domainRepo.GetDomain(ctx, config.Name)
	if err == nil && existingDomain != nil {
		return ErrDomainAlreadyExists
	}

	// Créer le domaine
	domain := &model.Domain{
		Name:   config.Name,
		Schema: config.Schema,
		Queues: make(map[string]*model.Queue),
		Routes: make(map[string]map[string]*model.RoutingRule),
	}

	// Créer les files d'attente initiales si elles sont définies
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

	// Ajouter les règles de routage si elles sont définies
	if config.RoutingRules != nil {
		for _, rule := range config.RoutingRules {
			// Initialiser la map si nécessaire
			if domain.Routes[rule.SourceQueue] == nil {
				domain.Routes[rule.SourceQueue] = make(map[string]*model.RoutingRule)
			}
			domain.Routes[rule.SourceQueue][rule.DestinationQueue] = rule
		}
	}

	// Stocker le domaine
	return s.domainRepo.StoreDomain(ctx, domain)
}

// GetDomain récupère un domaine par son nom
func (s *DomainServiceImpl) GetDomain(ctx context.Context, name string) (*model.Domain, error) {
	log.Printf("Getting domain: %s", name)
	return s.domainRepo.GetDomain(ctx, name)
}

// DeleteDomain supprime un domaine
func (s *DomainServiceImpl) DeleteDomain(ctx context.Context, name string) error {
	log.Printf("Deleting domain: %s", name)

	// Vérifier si le domaine existe
	_, err := s.domainRepo.GetDomain(ctx, name)
	if err != nil {
		return ErrDomainNotFound
	}

	// Supprimer le domaine
	return s.domainRepo.DeleteDomain(ctx, name)
}

// ListDomains liste tous les domaines
func (s *DomainServiceImpl) ListDomains(ctx context.Context) ([]*model.Domain, error) {
	log.Println("Listing domains")
	return s.domainRepo.ListDomains(ctx)
}
