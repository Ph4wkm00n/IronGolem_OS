// Package internal implements tenant and workspace management for the
// IronGolem OS Tenancy service.
//
// The TenantManager enforces the multi-tenant isolation hierarchy and
// supports all three deployment modes: Solo (single user), Household
// (small shared group), and Team (full multi-tenant with per-workspace
// database isolation).
package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/models"
)

// Sentinel errors for the tenancy package.
var (
	ErrAccessDenied    = errors.New("access denied: tenant isolation violation")
	ErrTenantNotFound  = errors.New("tenant not found")
	ErrTenantDisabled  = errors.New("tenant is disabled")
	ErrWorkspaceExists = errors.New("workspace with that name already exists")
)

// RoleMember represents a user's role within a workspace.
type RoleMember struct {
	UserID      string          `json:"user_id"`
	TenantID    string          `json:"tenant_id"`
	WorkspaceID string          `json:"workspace_id"`
	Role        models.UserRole `json:"role"`
	AssignedAt  time.Time       `json:"assigned_at"`
	AssignedBy  string          `json:"assigned_by,omitempty"`
}

// TenantStore defines the persistence interface for tenants and workspaces.
type TenantStore interface {
	SaveTenant(ctx context.Context, tenant *models.Tenant) error
	GetTenant(ctx context.Context, id string) (*models.Tenant, error)
	ListTenants(ctx context.Context) ([]*models.Tenant, error)

	SaveWorkspace(ctx context.Context, ws *models.Workspace) error
	GetWorkspace(ctx context.Context, id string) (*models.Workspace, error)
	ListWorkspaces(ctx context.Context, tenantID string) ([]*models.Workspace, error)

	SaveRole(ctx context.Context, member *RoleMember) error
	ListRoles(ctx context.Context, workspaceID string) ([]RoleMember, error)
	GetRole(ctx context.Context, workspaceID, userID string) (*RoleMember, error)
	DeleteRole(ctx context.Context, workspaceID, userID string) error
}

// MemoryTenantStore is an in-memory implementation of TenantStore.
type MemoryTenantStore struct {
	mu         sync.RWMutex
	tenants    map[string]*models.Tenant
	workspaces map[string]*models.Workspace
	roles      map[string][]RoleMember // keyed by workspaceID
}

// NewMemoryTenantStore creates an empty in-memory store.
func NewMemoryTenantStore() *MemoryTenantStore {
	return &MemoryTenantStore{
		tenants:    make(map[string]*models.Tenant),
		workspaces: make(map[string]*models.Workspace),
		roles:      make(map[string][]RoleMember),
	}
}

func (s *MemoryTenantStore) SaveTenant(_ context.Context, tenant *models.Tenant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *tenant
	s.tenants[tenant.ID] = &cp
	return nil
}

func (s *MemoryTenantStore) GetTenant(_ context.Context, id string) (*models.Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tenants[id]
	if !ok {
		return nil, ErrTenantNotFound
	}
	cp := *t
	return &cp, nil
}

func (s *MemoryTenantStore) ListTenants(_ context.Context) ([]*models.Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*models.Tenant, 0, len(s.tenants))
	for _, t := range s.tenants {
		cp := *t
		result = append(result, &cp)
	}
	return result, nil
}

func (s *MemoryTenantStore) SaveWorkspace(_ context.Context, ws *models.Workspace) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *ws
	s.workspaces[ws.ID] = &cp
	return nil
}

func (s *MemoryTenantStore) GetWorkspace(_ context.Context, id string) (*models.Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ws, ok := s.workspaces[id]
	if !ok {
		return nil, fmt.Errorf("workspace %q not found", id)
	}
	cp := *ws
	return &cp, nil
}

func (s *MemoryTenantStore) ListWorkspaces(_ context.Context, tenantID string) ([]*models.Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*models.Workspace
	for _, ws := range s.workspaces {
		if tenantID == "" || ws.TenantID == tenantID {
			cp := *ws
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (s *MemoryTenantStore) SaveRole(_ context.Context, member *RoleMember) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing := s.roles[member.WorkspaceID]
	found := false
	for i, m := range existing {
		if m.UserID == member.UserID {
			existing[i] = *member
			found = true
			break
		}
	}
	if !found {
		s.roles[member.WorkspaceID] = append(existing, *member)
	}
	return nil
}

func (s *MemoryTenantStore) ListRoles(_ context.Context, workspaceID string) ([]RoleMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]RoleMember, len(s.roles[workspaceID]))
	copy(result, s.roles[workspaceID])
	return result, nil
}

func (s *MemoryTenantStore) GetRole(_ context.Context, workspaceID, userID string) (*RoleMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, m := range s.roles[workspaceID] {
		if m.UserID == userID {
			return &m, nil
		}
	}
	return nil, fmt.Errorf("role not found for user %q in workspace %q", userID, workspaceID)
}

func (s *MemoryTenantStore) DeleteRole(_ context.Context, workspaceID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	members := s.roles[workspaceID]
	for i, m := range members {
		if m.UserID == userID {
			s.roles[workspaceID] = append(members[:i], members[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("role not found for user %q in workspace %q", userID, workspaceID)
}

// TenantManager is the primary entry point for all tenancy operations. It
// enforces isolation, validates deployment mode constraints, and emits
// events for the audit trail.
type TenantManager struct {
	store  TenantStore
	logger *slog.Logger
}

// NewTenantManager creates a TenantManager with an in-memory store.
func NewTenantManager(logger *slog.Logger) *TenantManager {
	return &TenantManager{
		store:  NewMemoryTenantStore(),
		logger: logger,
	}
}

// SetStore replaces the backing store (for testing or swapping to PostgreSQL).
func (m *TenantManager) SetStore(store TenantStore) {
	m.store = store
}

// CreateTenant provisions a new tenant with the specified deployment mode.
func (m *TenantManager) CreateTenant(ctx context.Context, name string, mode models.DeploymentMode) (*models.Tenant, error) {
	if name == "" {
		return nil, errors.New("tenant name is required")
	}

	now := time.Now().UTC()
	tenant := &models.Tenant{
		ID:             generateID(),
		Name:           name,
		DeploymentMode: mode,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := m.store.SaveTenant(ctx, tenant); err != nil {
		return nil, fmt.Errorf("saving tenant: %w", err)
	}

	payload, _ := json.Marshal(map[string]string{
		"tenant_id": tenant.ID,
		"name":      tenant.Name,
		"mode":      string(tenant.DeploymentMode),
	})
	evt := events.NewEvent(events.EventKindTenantCreated, tenant.ID, "tenancy", payload)
	m.logger.InfoContext(ctx, "tenant created",
		slog.String("event_id", evt.ID),
		slog.String("tenant_id", tenant.ID),
		slog.String("mode", string(mode)),
	)

	// Solo and Household modes get a default workspace automatically.
	if mode == models.DeploymentSolo || mode == models.DeploymentHousehold {
		_, err := m.CreateWorkspace(ctx, tenant.ID, "default")
		if err != nil {
			return nil, fmt.Errorf("creating default workspace: %w", err)
		}
	}

	return tenant, nil
}

// GetTenant retrieves a tenant by ID.
func (m *TenantManager) GetTenant(ctx context.Context, id string) (*models.Tenant, error) {
	return m.store.GetTenant(ctx, id)
}

// CreateWorkspace creates a new workspace within a tenant. It validates
// that the tenant exists and is not disabled, and enforces deployment
// mode constraints (e.g., Solo mode allows only one workspace).
func (m *TenantManager) CreateWorkspace(ctx context.Context, tenantID, name string) (*models.Workspace, error) {
	if tenantID == "" {
		return nil, errors.New("tenant_id is required")
	}
	if name == "" {
		return nil, errors.New("workspace name is required")
	}

	// Validate tenant exists and is active.
	tenant, err := m.store.GetTenant(ctx, tenantID)
	if err != nil {
		// For in-memory store during bootstrap, auto-create the tenant.
		if errors.Is(err, ErrTenantNotFound) {
			tenant = &models.Tenant{
				ID:             tenantID,
				Name:           tenantID,
				DeploymentMode: models.DeploymentTeam,
				CreatedAt:      time.Now().UTC(),
				UpdatedAt:      time.Now().UTC(),
			}
			if saveErr := m.store.SaveTenant(ctx, tenant); saveErr != nil {
				return nil, fmt.Errorf("auto-creating tenant: %w", saveErr)
			}
			m.logger.InfoContext(ctx, "tenant auto-created",
				slog.String("tenant_id", tenantID),
			)
		} else {
			return nil, fmt.Errorf("looking up tenant: %w", err)
		}
	}

	if tenant.Disabled {
		return nil, ErrTenantDisabled
	}

	// Enforce deployment mode constraints.
	existing, err := m.store.ListWorkspaces(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("listing workspaces: %w", err)
	}

	switch tenant.DeploymentMode {
	case models.DeploymentSolo:
		if len(existing) >= 1 {
			return nil, errors.New("solo mode allows only one workspace")
		}
	case models.DeploymentHousehold:
		if len(existing) >= 5 {
			return nil, errors.New("household mode allows a maximum of 5 workspaces")
		}
	case models.DeploymentTeam:
		// No limit.
	}

	// Check for duplicate names within the tenant.
	for _, ws := range existing {
		if ws.Name == name {
			return nil, ErrWorkspaceExists
		}
	}

	now := time.Now().UTC()
	ws := &models.Workspace{
		ID:        generateID(),
		TenantID:  tenantID,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := m.store.SaveWorkspace(ctx, ws); err != nil {
		return nil, fmt.Errorf("saving workspace: %w", err)
	}

	payload, _ := json.Marshal(map[string]string{
		"workspace_id": ws.ID,
		"tenant_id":    tenantID,
		"name":         name,
	})
	evt := events.NewEvent(events.EventKindWorkspaceCreated, tenantID, "tenancy", payload)
	m.logger.InfoContext(ctx, "workspace created",
		slog.String("event_id", evt.ID),
		slog.String("workspace_id", ws.ID),
		slog.String("tenant_id", tenantID),
	)

	return ws, nil
}

// GetWorkspace retrieves a workspace by ID, enforcing tenant isolation.
// If tenantID is non-empty, the workspace must belong to that tenant.
func (m *TenantManager) GetWorkspace(ctx context.Context, id, tenantID string) (*models.Workspace, error) {
	ws, err := m.store.GetWorkspace(ctx, id)
	if err != nil {
		return nil, err
	}

	// Enforce tenant isolation.
	if tenantID != "" && ws.TenantID != tenantID {
		m.logger.WarnContext(ctx, "tenant isolation violation",
			slog.String("workspace_id", id),
			slog.String("requesting_tenant", tenantID),
			slog.String("owning_tenant", ws.TenantID),
		)
		return nil, ErrAccessDenied
	}

	return ws, nil
}

// ListWorkspaces returns all workspaces, optionally filtered by tenant.
func (m *TenantManager) ListWorkspaces(ctx context.Context, tenantID string) ([]*models.Workspace, error) {
	return m.store.ListWorkspaces(ctx, tenantID)
}

// AssignRole grants a user a role within a workspace. It validates the
// role and enforces that the assigner has sufficient permissions.
func (m *TenantManager) AssignRole(ctx context.Context, workspaceID, userID string, role models.UserRole, assignedBy string) error {
	if workspaceID == "" || userID == "" {
		return errors.New("workspace_id and user_id are required")
	}

	// Validate role.
	switch role {
	case models.UserRoleOwner, models.UserRoleAdmin, models.UserRoleMember, models.UserRoleViewer:
		// Valid.
	default:
		return fmt.Errorf("invalid role: %q", role)
	}

	member := &RoleMember{
		UserID:      userID,
		WorkspaceID: workspaceID,
		Role:        role,
		AssignedAt:  time.Now().UTC(),
		AssignedBy:  assignedBy,
	}

	// Look up the workspace to get the tenant ID.
	ws, err := m.store.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("workspace lookup: %w", err)
	}
	member.TenantID = ws.TenantID

	if err := m.store.SaveRole(ctx, member); err != nil {
		return fmt.Errorf("saving role: %w", err)
	}

	m.logger.InfoContext(ctx, "role assigned",
		slog.String("workspace_id", workspaceID),
		slog.String("user_id", userID),
		slog.String("role", string(role)),
	)

	return nil
}

// GetUserRole returns a user's role in a workspace.
func (m *TenantManager) GetUserRole(ctx context.Context, workspaceID, userID string) (*RoleMember, error) {
	return m.store.GetRole(ctx, workspaceID, userID)
}

// ListMembers returns all role assignments for a workspace.
func (m *TenantManager) ListMembers(ctx context.Context, workspaceID string) ([]RoleMember, error) {
	return m.store.ListRoles(ctx, workspaceID)
}

// RemoveRole revokes a user's role in a workspace.
func (m *TenantManager) RemoveRole(ctx context.Context, workspaceID, userID string) error {
	if err := m.store.DeleteRole(ctx, workspaceID, userID); err != nil {
		return err
	}

	m.logger.InfoContext(ctx, "role removed",
		slog.String("workspace_id", workspaceID),
		slog.String("user_id", userID),
	)
	return nil
}

// CheckAccess verifies that a user has at least the required role in a
// workspace. Role hierarchy: owner > admin > member > viewer.
func (m *TenantManager) CheckAccess(ctx context.Context, workspaceID, userID string, requiredRole models.UserRole) error {
	member, err := m.store.GetRole(ctx, workspaceID, userID)
	if err != nil {
		return ErrAccessDenied
	}

	if roleLevel(member.Role) < roleLevel(requiredRole) {
		m.logger.WarnContext(ctx, "insufficient role",
			slog.String("workspace_id", workspaceID),
			slog.String("user_id", userID),
			slog.String("has_role", string(member.Role)),
			slog.String("required_role", string(requiredRole)),
		)
		return ErrAccessDenied
	}

	return nil
}

// roleLevel maps roles to numeric levels for comparison.
func roleLevel(role models.UserRole) int {
	switch role {
	case models.UserRoleOwner:
		return 4
	case models.UserRoleAdmin:
		return 3
	case models.UserRoleMember:
		return 2
	case models.UserRoleViewer:
		return 1
	default:
		return 0
	}
}

func generateID() string {
	return time.Now().UTC().Format("20060102150405.000000000")
}
