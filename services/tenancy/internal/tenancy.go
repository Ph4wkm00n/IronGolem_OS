// Package internal implements multi-tenant management for IronGolem OS.
//
// The tenancy manager handles:
//   - Tenant creation and lifecycle
//   - Workspace creation with isolation boundaries
//   - User management with role-based access control
//   - Deployment mode enforcement (Solo, Household, Team)
//
// In Solo and Household modes, the limits on tenants, workspaces, and users
// are restricted. Team mode supports full multi-tenant isolation with
// per-workspace database schemas.
package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/models"
)

// Deployment mode limits.
var modeLimits = map[models.DeploymentMode]struct {
	maxTenants    int
	maxWorkspaces int // per tenant
	maxUsers      int // per workspace
}{
	models.DeploymentSolo:      {maxTenants: 1, maxWorkspaces: 1, maxUsers: 1},
	models.DeploymentHousehold: {maxTenants: 1, maxWorkspaces: 3, maxUsers: 10},
	models.DeploymentTeam:      {maxTenants: 100, maxWorkspaces: 50, maxUsers: 500},
}

// TenancyManager handles tenant, workspace, and user lifecycle with
// isolation enforcement.
type TenancyManager struct {
	mu         sync.RWMutex
	tenants    map[string]*models.Tenant
	workspaces map[string]*models.Workspace   // key: workspace ID
	users      map[string]*models.User         // key: user ID
	logger     *slog.Logger
	nextID     int64
}

// NewTenancyManager creates a new TenancyManager.
func NewTenancyManager(logger *slog.Logger) *TenancyManager {
	return &TenancyManager{
		tenants:    make(map[string]*models.Tenant),
		workspaces: make(map[string]*models.Workspace),
		users:      make(map[string]*models.User),
		logger:     logger,
	}
}

func (m *TenancyManager) genID(prefix string) string {
	m.nextID++
	return fmt.Sprintf("%s_%d", prefix, m.nextID)
}

// --- Tenant Operations ---

// CreateTenantRequest is the input for creating a new tenant.
type CreateTenantRequest struct {
	Name           string                `json:"name"`
	DeploymentMode models.DeploymentMode `json:"deployment_mode"`
	Metadata       map[string]string     `json:"metadata,omitempty"`
}

// CreateTenant creates a new tenant if the deployment mode limits allow it.
func (m *TenancyManager) CreateTenant(req CreateTenantRequest) (*models.Tenant, error) {
	if req.Name == "" {
		return nil, errors.New("name is required")
	}

	mode := req.DeploymentMode
	if mode == "" {
		mode = models.DeploymentSolo
	}

	limits, ok := modeLimits[mode]
	if !ok {
		return nil, fmt.Errorf("unsupported deployment mode: %s", mode)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.tenants) >= limits.maxTenants {
		return nil, fmt.Errorf("tenant limit reached for %s mode (max %d)", mode, limits.maxTenants)
	}

	// Check for duplicate name.
	for _, t := range m.tenants {
		if t.Name == req.Name {
			return nil, fmt.Errorf("tenant %q already exists", req.Name)
		}
	}

	now := time.Now().UTC()
	tenant := &models.Tenant{
		ID:             m.genID("tenant"),
		Name:           req.Name,
		DeploymentMode: mode,
		CreatedAt:      now,
		UpdatedAt:      now,
		Metadata:       req.Metadata,
	}

	m.tenants[tenant.ID] = tenant

	m.logger.Info("tenant created",
		slog.String("tenant_id", tenant.ID),
		slog.String("name", tenant.Name),
		slog.String("mode", string(mode)),
	)

	return tenant, nil
}

// GetTenant returns a tenant by ID.
func (m *TenancyManager) GetTenant(id string) (*models.Tenant, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.tenants[id]
	if !ok {
		return nil, false
	}
	cp := *t
	return &cp, true
}

// ListTenants returns all tenants.
func (m *TenancyManager) ListTenants() []models.Tenant {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]models.Tenant, 0, len(m.tenants))
	for _, t := range m.tenants {
		result = append(result, *t)
	}
	return result
}

// --- Workspace Operations ---

// CreateWorkspaceRequest is the input for creating a new workspace.
type CreateWorkspaceRequest struct {
	Name string `json:"name"`
}

// CreateWorkspace creates a workspace within a tenant, enforcing mode limits.
func (m *TenancyManager) CreateWorkspace(tenantID string, req CreateWorkspaceRequest) (*models.Workspace, error) {
	if req.Name == "" {
		return nil, errors.New("name is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	tenant, ok := m.tenants[tenantID]
	if !ok {
		return nil, errors.New("tenant not found")
	}
	if tenant.Disabled {
		return nil, errors.New("tenant is disabled")
	}

	limits := modeLimits[tenant.DeploymentMode]

	// Count existing workspaces for this tenant.
	var count int
	for _, ws := range m.workspaces {
		if ws.TenantID == tenantID {
			count++
		}
	}
	if count >= limits.maxWorkspaces {
		return nil, fmt.Errorf("workspace limit reached for %s mode (max %d)", tenant.DeploymentMode, limits.maxWorkspaces)
	}

	now := time.Now().UTC()
	ws := &models.Workspace{
		ID:        m.genID("ws"),
		TenantID:  tenantID,
		Name:      req.Name,
		CreatedAt: now,
		UpdatedAt: now,
	}

	m.workspaces[ws.ID] = ws

	m.logger.Info("workspace created",
		slog.String("workspace_id", ws.ID),
		slog.String("tenant_id", tenantID),
		slog.String("name", ws.Name),
	)

	return ws, nil
}

// ListWorkspaces returns all workspaces for a tenant.
func (m *TenancyManager) ListWorkspaces(tenantID string) []models.Workspace {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []models.Workspace
	for _, ws := range m.workspaces {
		if ws.TenantID == tenantID {
			result = append(result, *ws)
		}
	}
	return result
}

// --- User Operations ---

// AddUserRequest is the input for adding a user to a workspace.
type AddUserRequest struct {
	Email       string          `json:"email"`
	DisplayName string          `json:"display_name"`
	Role        models.UserRole `json:"role"`
}

// AddUser adds a user to a workspace within a tenant, enforcing mode limits
// and tenant isolation.
func (m *TenancyManager) AddUser(tenantID, workspaceID string, req AddUserRequest) (*models.User, error) {
	if req.Email == "" {
		return nil, errors.New("email is required")
	}
	if req.Role == "" {
		req.Role = models.UserRoleMember
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	tenant, ok := m.tenants[tenantID]
	if !ok {
		return nil, errors.New("tenant not found")
	}
	if tenant.Disabled {
		return nil, errors.New("tenant is disabled")
	}

	// Verify workspace belongs to tenant.
	ws, ok := m.workspaces[workspaceID]
	if !ok || ws.TenantID != tenantID {
		return nil, errors.New("workspace not found in this tenant")
	}

	limits := modeLimits[tenant.DeploymentMode]

	// Count existing users in this workspace.
	var count int
	for _, u := range m.users {
		if u.TenantID == tenantID && u.WorkspaceID == workspaceID {
			count++
		}
	}
	if count >= limits.maxUsers {
		return nil, fmt.Errorf("user limit reached for %s mode (max %d)", tenant.DeploymentMode, limits.maxUsers)
	}

	// Check for duplicate email within the workspace.
	for _, u := range m.users {
		if u.TenantID == tenantID && u.WorkspaceID == workspaceID && u.Email == req.Email {
			return nil, fmt.Errorf("user %q already exists in this workspace", req.Email)
		}
	}

	now := time.Now().UTC()
	user := &models.User{
		ID:          m.genID("user"),
		TenantID:    tenantID,
		WorkspaceID: workspaceID,
		Email:       req.Email,
		DisplayName: req.DisplayName,
		Role:        req.Role,
		CreatedAt:   now,
		LastSeenAt:  now,
	}

	m.users[user.ID] = user

	m.logger.Info("user added",
		slog.String("user_id", user.ID),
		slog.String("tenant_id", tenantID),
		slog.String("workspace_id", workspaceID),
		slog.String("role", string(req.Role)),
	)

	return user, nil
}

// ListUsers returns all users in a workspace, enforcing tenant isolation.
func (m *TenancyManager) ListUsers(tenantID, workspaceID string) []models.User {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []models.User
	for _, u := range m.users {
		if u.TenantID == tenantID && u.WorkspaceID == workspaceID {
			result = append(result, *u)
		}
	}
	return result
}

// --- HTTP Handlers ---

// Handler provides HTTP handlers for the tenancy service API.
type Handler struct {
	logger *slog.Logger
	mgr    *TenancyManager
}

// NewHandler creates a new Handler.
func NewHandler(logger *slog.Logger, mgr *TenancyManager) *Handler {
	return &Handler{logger: logger, mgr: mgr}
}

// HealthCheck responds with the service health status.
func (h *Handler) HealthCheck(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "tenancy",
		"time":    time.Now().UTC(),
	})
}

// CreateTenant handles POST /api/v1/tenants.
func (h *Handler) CreateTenant(w http.ResponseWriter, r *http.Request) {
	var req CreateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
		return
	}

	tenant, err := h.mgr.CreateTenant(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusCreated, tenant)
}

// ListTenants handles GET /api/v1/tenants.
func (h *Handler) ListTenants(w http.ResponseWriter, _ *http.Request) {
	tenants := h.mgr.ListTenants()
	writeJSON(w, http.StatusOK, map[string]any{
		"tenants": tenants,
		"count":   len(tenants),
	})
}

// GetTenant handles GET /api/v1/tenants/{id}.
func (h *Handler) GetTenant(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tenant, ok := h.mgr.GetTenant(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "tenant not found",
		})
		return
	}
	writeJSON(w, http.StatusOK, tenant)
}

// CreateWorkspace handles POST /api/v1/tenants/{tenant_id}/workspaces.
func (h *Handler) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("tenant_id")

	var req CreateWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
		return
	}

	ws, err := h.mgr.CreateWorkspace(tenantID, req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusCreated, ws)
}

// ListWorkspaces handles GET /api/v1/tenants/{tenant_id}/workspaces.
func (h *Handler) ListWorkspaces(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("tenant_id")
	workspaces := h.mgr.ListWorkspaces(tenantID)
	if workspaces == nil {
		workspaces = []models.Workspace{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"workspaces": workspaces,
		"count":      len(workspaces),
	})
}

// AddUser handles POST /api/v1/tenants/{tenant_id}/workspaces/{workspace_id}/users.
func (h *Handler) AddUser(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("tenant_id")
	workspaceID := r.PathValue("workspace_id")

	var req AddUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
		return
	}

	user, err := h.mgr.AddUser(tenantID, workspaceID, req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

// ListUsers handles GET /api/v1/tenants/{tenant_id}/workspaces/{workspace_id}/users.
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("tenant_id")
	workspaceID := r.PathValue("workspace_id")

	users := h.mgr.ListUsers(tenantID, workspaceID)
	if users == nil {
		users = []models.User{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"users": users,
		"count": len(users),
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
