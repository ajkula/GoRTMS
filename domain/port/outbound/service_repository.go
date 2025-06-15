package outbound

import (
	"context"

	"github.com/ajkula/GoRTMS/domain/model"
)

// interface for service account storage
type ServiceRepository interface {
	GetByID(ctx context.Context, serviceID string) (*model.ServiceAccount, error)
	Create(ctx context.Context, service *model.ServiceAccount) error
	Update(ctx context.Context, service *model.ServiceAccount) error
	Delete(ctx context.Context, serviceID string) error
	List(ctx context.Context) ([]*model.ServiceAccount, error)
	UpdateLastUsed(ctx context.Context, serviceID string) error
}
