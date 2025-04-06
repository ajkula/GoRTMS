package memory

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

// DomainRepository est une implémentation en mémoire du repository de domaines
type DomainRepository struct {
	domains map[string]*model.Domain
	mutex   sync.RWMutex
}

// NewDomainRepository crée un nouveau repository de domaines en mémoire
func NewDomainRepository() outbound.DomainRepository {
	return &DomainRepository{
		domains: make(map[string]*model.Domain),
	}
}

// StoreDomain stocke un domaine (création ou mise à jour)
func (r *DomainRepository) StoreDomain(ctx context.Context, domain *model.Domain) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Vérifier si c'est une mise à jour ou une création
	_, exists := r.domains[domain.Name]
	if exists {
		log.Printf("Updating existing domain: %s", domain.Name)
	} else {
		log.Printf("Creating new domain: %s", domain.Name)
	}

	// Stocker/remplacer le domaine (peu importe s'il existe déjà)
	r.domains[domain.Name] = domain
	return nil
}

// GetDomain récupère un domaine par son nom
func (r *DomainRepository) GetDomain(ctx context.Context, name string) (*model.Domain, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	domain, ok := r.domains[name]
	if !ok {
		return nil, errors.New("domain not found")
	}

	return domain, nil
}

// DeleteDomain supprime un domaine
func (r *DomainRepository) DeleteDomain(ctx context.Context, name string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, ok := r.domains[name]; !ok {
		return errors.New("domain not found")
	}

	delete(r.domains, name)
	return nil
}

// ListDomains liste tous les domaines
func (r *DomainRepository) ListDomains(ctx context.Context) ([]*model.Domain, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	domains := make([]*model.Domain, 0, len(r.domains))
	for _, domain := range r.domains {
		domains = append(domains, domain)
	}

	return domains, nil
}
