package outbound

import (
	"context"

	"github.com/ajkula/GoRTMS/domain/model"
)

// defines storage operations for account requests
type AccountRequestRepository interface {
	// persists the account request database to storage
	Save(ctx context.Context, db *model.AccountRequestDatabase) error

	// retrieves the account request database from storage
	Load(ctx context.Context) (*model.AccountRequestDatabase, error)

	// checks if the account request database file exists
	Exists() bool

	// saves a single account request
	Store(ctx context.Context, request *model.AccountRequest) error

	// retrieves an account request by ID
	GetByID(ctx context.Context, requestID string) (*model.AccountRequest, error)

	// retrieves an account request by username
	GetByUsername(ctx context.Context, username string) (*model.AccountRequest, error)

	// retrieves all account requests with optional status filter
	List(ctx context.Context, status *model.AccountRequestStatus) ([]*model.AccountRequest, error)

	// removes an account request by ID
	Delete(ctx context.Context, requestID string) error

	// retrieves all pending account requests
	GetPendingRequests(ctx context.Context) ([]*model.AccountRequest, error)
}
