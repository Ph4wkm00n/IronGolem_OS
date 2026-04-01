// Package store provides a database abstraction layer for IronGolem OS.
//
// The Store interface defines the persistence contract used by all services.
// Two implementations are provided:
//   - SqliteStore for solo/household deployment (local-first)
//   - PgStore for team deployment (multi-tenant PostgreSQL)
//
// Use NewStore with the appropriate Config to instantiate the correct backend.
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/models"
)

// Driver identifies the database backend to use.
type Driver string

const (
	// DriverSQLite selects the SQLite backend for solo/household mode.
	DriverSQLite Driver = "sqlite"

	// DriverPostgres selects the PostgreSQL backend for team mode.
	DriverPostgres Driver = "postgres"
)

// Config holds the connection parameters for a Store backend.
type Config struct {
	// Driver selects the database backend ("sqlite" or "postgres").
	Driver Driver

	// DSN is the data source name / connection string.
	// For SQLite: a file path, e.g. "/data/irongolem.db" or ":memory:"
	// For PostgreSQL: a connection string, e.g.
	//   "host=localhost port=5432 user=irongolem dbname=irongolem sslmode=disable"
	DSN string

	// MaxOpenConns controls the maximum number of open connections (PostgreSQL only).
	MaxOpenConns int

	// MaxIdleConns controls the maximum number of idle connections (PostgreSQL only).
	MaxIdleConns int

	// ConnMaxLifetime controls how long a connection can be reused (PostgreSQL only).
	ConnMaxLifetime time.Duration
}

// Store is the persistence interface for IronGolem OS services.
// All methods accept a context.Context for cancellation and tenant propagation.
type Store interface {
	// --- Events ---

	// SaveEvent persists a new event to the event log.
	SaveEvent(ctx context.Context, event *events.Event) error

	// ListEvents returns events for a tenant, ordered by timestamp descending,
	// up to the specified limit. If workspaceID is non-empty, events are
	// further filtered by workspace.
	ListEvents(ctx context.Context, tenantID, workspaceID string, limit int) ([]events.Event, error)

	// --- Recipes ---

	// SaveRecipe creates or updates a recipe.
	SaveRecipe(ctx context.Context, recipe *models.Recipe) error

	// ListRecipes returns all recipes for a tenant and workspace.
	ListRecipes(ctx context.Context, tenantID, workspaceID string) ([]models.Recipe, error)

	// --- Approvals ---

	// SaveApproval creates or updates an approval request.
	SaveApproval(ctx context.Context, approval *models.ApprovalRequest) error

	// GetApproval retrieves an approval request by ID.
	GetApproval(ctx context.Context, id string) (*models.ApprovalRequest, error)

	// ListPendingApprovals returns all pending approval requests for a tenant.
	ListPendingApprovals(ctx context.Context, tenantID string) ([]models.ApprovalRequest, error)

	// --- Workspaces ---

	// SaveWorkspace creates or updates a workspace.
	SaveWorkspace(ctx context.Context, ws *models.Workspace) error

	// ListWorkspaces returns all workspaces for a tenant.
	ListWorkspaces(ctx context.Context, tenantID string) ([]models.Workspace, error)

	// --- Tenants ---

	// SaveTenant creates or updates a tenant.
	SaveTenant(ctx context.Context, tenant *models.Tenant) error

	// GetTenant retrieves a tenant by ID.
	GetTenant(ctx context.Context, id string) (*models.Tenant, error)

	// --- Members ---

	// SaveMember creates or updates a user (workspace member).
	SaveMember(ctx context.Context, user *models.User) error

	// ListMembers returns all members for a workspace.
	ListMembers(ctx context.Context, tenantID, workspaceID string) ([]models.User, error)

	// --- Lifecycle ---

	// Close releases database resources.
	Close() error
}

// NewStore creates a Store backed by the driver specified in cfg.
// It initializes the schema and returns a ready-to-use store.
func NewStore(cfg Config) (Store, error) {
	switch cfg.Driver {
	case DriverSQLite:
		return NewSqliteStore(cfg)
	case DriverPostgres:
		return NewPgStore(cfg)
	default:
		return nil, fmt.Errorf("unsupported store driver: %q", cfg.Driver)
	}
}
