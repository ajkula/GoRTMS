package memory

import (
	"context"
	"errors"
	"sync"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

type DomainRepository struct {
	domains map[string]*model.Domain
	logger  outbound.Logger
	mutex   sync.RWMutex
}

func NewDomainRepository(logger outbound.Logger) outbound.DomainRepository {
	return &DomainRepository{
		domains: make(map[string]*model.Domain),
		logger:  logger,
	}
}

func (r *DomainRepository) StoreDomain(ctx context.Context, domain *model.Domain) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Check if it's an update or a creation
	_, exists := r.domains[domain.Name]
	if exists {
		r.logger.Debug("Updating", "domain", domain.Name)
	} else {
		r.logger.Debug("Creating", "domain", domain.Name)
		r.domains[domain.Name] = domain
	}

	return nil
}

func (r *DomainRepository) GetDomain(ctx context.Context, name string) (*model.Domain, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	domain, ok := r.domains[name]
	if !ok {
		return nil, errors.New("domain not found")
	}

	return domain, nil
}

func (r *DomainRepository) DeleteDomain(ctx context.Context, name string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, ok := r.domains[name]; !ok {
		return errors.New("domain not found")
	}

	delete(r.domains, name)
	return nil
}

func (r *DomainRepository) ListDomains(ctx context.Context) ([]*model.Domain, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	domains := make([]*model.Domain, 0, len(r.domains))
	for _, domain := range r.domains {
		domains = append(domains, domain)
	}

	return domains, nil
}
