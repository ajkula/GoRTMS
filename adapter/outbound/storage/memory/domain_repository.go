package memory

import (
	"context"
	"errors"
	"sync"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

var (
	ErrDomainNotFound = errors.New("domain not found")
	ErrDomainExists   = errors.New("domain already exists")
)

// DomainRepository implémente un repository de domaines en mémoire
type DomainRepository struct {
	domains map[string]*model.Domain
	mu      sync.RWMutex
}

// NewDomainRepository crée un nouveau repository de domaines en mémoire
func NewDomainRepository() outbound.DomainRepository {
	return &DomainRepository{
		domains: make(map[string]*model.Domain),
	}
}

// StoreDomain stocke un domaine
func (r *DomainRepository) StoreDomain(
	ctx context.Context,
	domain *model.Domain,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.domains[domain.Name]; exists {
		// Ne pas remplacer pour protéger contre les écrasements accidentels
		return ErrDomainExists
	}

	r.domains[domain.Name] = domain
	return nil
}

// GetDomain récupère un domaine par son nom
func (r *DomainRepository) GetDomain(
	ctx context.Context,
	name string,
) (*model.Domain, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	domain, exists := r.domains[name]
	if !exists {
		return nil, ErrDomainNotFound
	}

	return domain, nil
}

// DeleteDomain supprime un domaine
func (r *DomainRepository) DeleteDomain(
	ctx context.Context,
	name string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.domains[name]; !exists {
		return ErrDomainNotFound
	}

	delete(r.domains, name)
	return nil
}

// ListDomains liste tous les domaines
func (r *DomainRepository) ListDomains(
	ctx context.Context,
) ([]*model.Domain, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	domains := make([]*model.Domain, 0, len(r.domains))
	for _, domain := range r.domains {
		domains = append(domains, domain)
	}

	return domains, nil
}

// UpdateDomain met à jour un domaine existant
func (r *DomainRepository) UpdateDomain(
	ctx context.Context,
	domain *model.Domain,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.domains[domain.Name]; !exists {
		return ErrDomainNotFound
	}

	r.domains[domain.Name] = domain
	return nil
}
