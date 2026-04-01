package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/models"
)

// pgSchema creates all tables needed by the PostgreSQL store.
// Uses native PostgreSQL types: UUID, TIMESTAMPTZ, JSONB.
// Tenant isolation is enforced via tenant_id column filtering on every query.
const pgSchema = `
CREATE TABLE IF NOT EXISTS tenants (
	id              UUID PRIMARY KEY NOT NULL,
	name            TEXT NOT NULL,
	deployment_mode TEXT NOT NULL DEFAULT 'team',
	created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	disabled        BOOLEAN NOT NULL DEFAULT FALSE,
	metadata        JSONB
);

CREATE TABLE IF NOT EXISTS workspaces (
	id         UUID PRIMARY KEY NOT NULL,
	tenant_id  UUID NOT NULL REFERENCES tenants(id),
	name       TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	disabled   BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX IF NOT EXISTS idx_workspaces_tenant ON workspaces(tenant_id);

CREATE TABLE IF NOT EXISTS members (
	id           UUID PRIMARY KEY NOT NULL,
	tenant_id    UUID NOT NULL REFERENCES tenants(id),
	workspace_id UUID NOT NULL REFERENCES workspaces(id),
	email        TEXT NOT NULL,
	display_name TEXT NOT NULL,
	role         TEXT NOT NULL DEFAULT 'member',
	created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	disabled     BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX IF NOT EXISTS idx_members_ws ON members(tenant_id, workspace_id);

CREATE TABLE IF NOT EXISTS events (
	id             UUID PRIMARY KEY NOT NULL,
	kind           TEXT NOT NULL,
	tenant_id      UUID NOT NULL REFERENCES tenants(id),
	workspace_id   UUID,
	source_service TEXT NOT NULL,
	correlation_id TEXT,
	causation_id   TEXT,
	timestamp      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	payload        JSONB,
	metadata       JSONB,
	version        INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_events_tenant_ts ON events(tenant_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_events_workspace ON events(workspace_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_events_kind ON events(tenant_id, kind);

CREATE TABLE IF NOT EXISTS recipes (
	id             UUID PRIMARY KEY NOT NULL,
	tenant_id      UUID NOT NULL REFERENCES tenants(id),
	workspace_id   UUID NOT NULL REFERENCES workspaces(id),
	name           TEXT NOT NULL,
	description    TEXT NOT NULL DEFAULT '',
	safety_summary TEXT NOT NULL DEFAULT '',
	squad_kind     TEXT NOT NULL DEFAULT '',
	status         TEXT NOT NULL DEFAULT 'draft',
	trigger_config JSONB,
	created_by     TEXT NOT NULL DEFAULT '',
	created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_recipes_tenant_ws ON recipes(tenant_id, workspace_id);

CREATE TABLE IF NOT EXISTS approvals (
	id           UUID PRIMARY KEY NOT NULL,
	recipe_id    TEXT NOT NULL,
	step_id      TEXT NOT NULL,
	description  TEXT NOT NULL DEFAULT '',
	risk_level   TEXT NOT NULL DEFAULT 'low',
	status       TEXT NOT NULL DEFAULT 'pending',
	tenant_id    UUID NOT NULL REFERENCES tenants(id),
	workspace_id UUID,
	requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	responded_at TIMESTAMPTZ,
	responded_by TEXT,
	reason       TEXT
);
CREATE INDEX IF NOT EXISTS idx_approvals_tenant ON approvals(tenant_id, status);
`

// PgStore implements Store using PostgreSQL via database/sql.
type PgStore struct {
	db *sql.DB
}

// NewPgStore opens a PostgreSQL connection pool and initializes the schema.
func NewPgStore(cfg Config) (*PgStore, error) {
	db, err := sql.Open("postgres", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("postgres open: %w", err)
	}

	// Apply connection pool settings.
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	} else {
		db.SetMaxOpenConns(25)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	} else {
		db.SetMaxIdleConns(5)
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	} else {
		db.SetConnMaxLifetime(5 * time.Minute)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}

	if _, err := db.Exec(pgSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("postgres schema init: %w", err)
	}

	return &PgStore{db: db}, nil
}

// Close releases the database connection pool.
func (s *PgStore) Close() error {
	return s.db.Close()
}

// --- Events ---

func (s *PgStore) SaveEvent(ctx context.Context, event *events.Event) error {
	var payloadJSON interface{}
	if event.Payload != nil {
		payloadJSON = json.RawMessage(event.Payload)
	}
	metaJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO events (id, kind, tenant_id, workspace_id, source_service, correlation_id, causation_id, timestamp, payload, metadata, version)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		event.ID, string(event.Kind), event.TenantID, nullIfEmpty(event.WorkspaceID),
		event.SourceService, nullIfEmpty(event.CorrelationID), nullIfEmpty(event.CausationID),
		event.Timestamp, payloadJSON, json.RawMessage(metaJSON), event.Version,
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

func (s *PgStore) ListEvents(ctx context.Context, tenantID, workspaceID string, limit int) ([]events.Event, error) {
	var rows *sql.Rows
	var err error
	if workspaceID != "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, kind, tenant_id, workspace_id, source_service, correlation_id, causation_id, timestamp, payload, metadata, version
			 FROM events WHERE tenant_id = $1 AND workspace_id = $2 ORDER BY timestamp DESC LIMIT $3`,
			tenantID, workspaceID, limit)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, kind, tenant_id, workspace_id, source_service, correlation_id, causation_id, timestamp, payload, metadata, version
			 FROM events WHERE tenant_id = $1 ORDER BY timestamp DESC LIMIT $2`,
			tenantID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()
	return scanPgEvents(rows)
}

// --- Recipes ---

func (s *PgStore) SaveRecipe(ctx context.Context, recipe *models.Recipe) error {
	triggerJSON, err := json.Marshal(recipe.TriggerConfig)
	if err != nil {
		return fmt.Errorf("marshal trigger config: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO recipes (id, tenant_id, workspace_id, name, description, safety_summary, squad_kind, status, trigger_config, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 ON CONFLICT(id) DO UPDATE SET name=EXCLUDED.name, description=EXCLUDED.description,
		   safety_summary=EXCLUDED.safety_summary, squad_kind=EXCLUDED.squad_kind,
		   status=EXCLUDED.status, trigger_config=EXCLUDED.trigger_config, updated_at=EXCLUDED.updated_at`,
		recipe.ID, recipe.TenantID, recipe.WorkspaceID, recipe.Name,
		recipe.Description, recipe.SafetySummary, string(recipe.SquadKind),
		string(recipe.Status), json.RawMessage(triggerJSON), recipe.CreatedBy,
		recipe.CreatedAt, recipe.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save recipe: %w", err)
	}
	return nil
}

func (s *PgStore) ListRecipes(ctx context.Context, tenantID, workspaceID string) ([]models.Recipe, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, workspace_id, name, description, safety_summary, squad_kind, status, trigger_config, created_by, created_at, updated_at
		 FROM recipes WHERE tenant_id = $1 AND workspace_id = $2`,
		tenantID, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list recipes: %w", err)
	}
	defer rows.Close()
	return scanPgRecipes(rows)
}

// --- Approvals ---

func (s *PgStore) SaveApproval(ctx context.Context, a *models.ApprovalRequest) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO approvals (id, recipe_id, step_id, description, risk_level, status, tenant_id, workspace_id, requested_at, responded_at, responded_by, reason)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 ON CONFLICT(id) DO UPDATE SET status=EXCLUDED.status, responded_at=EXCLUDED.responded_at,
		   responded_by=EXCLUDED.responded_by, reason=EXCLUDED.reason`,
		a.ID, a.RecipeID, a.StepID, a.Description, string(a.RiskLevel),
		string(a.Status), a.TenantID, nullIfEmpty(a.WorkspaceID),
		a.RequestedAt, a.RespondedAt, nullIfEmpty(a.RespondedBy), nullIfEmpty(a.Reason),
	)
	if err != nil {
		return fmt.Errorf("save approval: %w", err)
	}
	return nil
}

func (s *PgStore) GetApproval(ctx context.Context, id string) (*models.ApprovalRequest, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, recipe_id, step_id, description, risk_level, status, tenant_id, workspace_id, requested_at, responded_at, responded_by, reason
		 FROM approvals WHERE id = $1`, id)

	a, err := scanPgApproval(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get approval: %w", err)
	}
	return a, nil
}

func (s *PgStore) ListPendingApprovals(ctx context.Context, tenantID string) ([]models.ApprovalRequest, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, recipe_id, step_id, description, risk_level, status, tenant_id, workspace_id, requested_at, responded_at, responded_by, reason
		 FROM approvals WHERE tenant_id = $1 AND status = 'pending' ORDER BY requested_at DESC`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list pending approvals: %w", err)
	}
	defer rows.Close()
	return scanPgApprovals(rows)
}

// --- Workspaces ---

func (s *PgStore) SaveWorkspace(ctx context.Context, ws *models.Workspace) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO workspaces (id, tenant_id, name, created_at, updated_at, disabled)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT(id) DO UPDATE SET name=EXCLUDED.name, updated_at=EXCLUDED.updated_at, disabled=EXCLUDED.disabled`,
		ws.ID, ws.TenantID, ws.Name, ws.CreatedAt, ws.UpdatedAt, ws.Disabled,
	)
	if err != nil {
		return fmt.Errorf("save workspace: %w", err)
	}
	return nil
}

func (s *PgStore) ListWorkspaces(ctx context.Context, tenantID string) ([]models.Workspace, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, name, created_at, updated_at, disabled
		 FROM workspaces WHERE tenant_id = $1`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	defer rows.Close()

	var result []models.Workspace
	for rows.Next() {
		var ws models.Workspace
		if err := rows.Scan(&ws.ID, &ws.TenantID, &ws.Name, &ws.CreatedAt, &ws.UpdatedAt, &ws.Disabled); err != nil {
			return nil, fmt.Errorf("scan workspace: %w", err)
		}
		result = append(result, ws)
	}
	return result, rows.Err()
}

// --- Tenants ---

func (s *PgStore) SaveTenant(ctx context.Context, tenant *models.Tenant) error {
	metaJSON, err := json.Marshal(tenant.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO tenants (id, name, deployment_mode, created_at, updated_at, disabled, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT(id) DO UPDATE SET name=EXCLUDED.name, deployment_mode=EXCLUDED.deployment_mode,
		   updated_at=EXCLUDED.updated_at, disabled=EXCLUDED.disabled, metadata=EXCLUDED.metadata`,
		tenant.ID, tenant.Name, string(tenant.DeploymentMode),
		tenant.CreatedAt, tenant.UpdatedAt, tenant.Disabled, json.RawMessage(metaJSON),
	)
	if err != nil {
		return fmt.Errorf("save tenant: %w", err)
	}
	return nil
}

func (s *PgStore) GetTenant(ctx context.Context, id string) (*models.Tenant, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, deployment_mode, created_at, updated_at, disabled, metadata
		 FROM tenants WHERE id = $1`, id)

	var t models.Tenant
	var metaJSON []byte
	err := row.Scan(&t.ID, &t.Name, &t.DeploymentMode, &t.CreatedAt, &t.UpdatedAt, &t.Disabled, &metaJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get tenant: %w", err)
	}
	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &t.Metadata)
	}
	return &t, nil
}

// --- Members ---

func (s *PgStore) SaveMember(ctx context.Context, user *models.User) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO members (id, tenant_id, workspace_id, email, display_name, role, created_at, last_seen_at, disabled)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT(id) DO UPDATE SET email=EXCLUDED.email, display_name=EXCLUDED.display_name,
		   role=EXCLUDED.role, last_seen_at=EXCLUDED.last_seen_at, disabled=EXCLUDED.disabled`,
		user.ID, user.TenantID, user.WorkspaceID, user.Email, user.DisplayName,
		string(user.Role), user.CreatedAt, user.LastSeenAt, user.Disabled,
	)
	if err != nil {
		return fmt.Errorf("save member: %w", err)
	}
	return nil
}

func (s *PgStore) ListMembers(ctx context.Context, tenantID, workspaceID string) ([]models.User, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, workspace_id, email, display_name, role, created_at, last_seen_at, disabled
		 FROM members WHERE tenant_id = $1 AND workspace_id = $2`, tenantID, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()

	var result []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.TenantID, &u.WorkspaceID, &u.Email, &u.DisplayName, &u.Role, &u.CreatedAt, &u.LastSeenAt, &u.Disabled); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		result = append(result, u)
	}
	return result, rows.Err()
}

// --- Helpers ---

// nullIfEmpty returns nil if s is empty, allowing PostgreSQL to store NULL
// instead of an empty string for optional UUID/TEXT columns.
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func scanPgEvents(rows *sql.Rows) ([]events.Event, error) {
	var result []events.Event
	for rows.Next() {
		var e events.Event
		var wsID, corrID, causeID sql.NullString
		var payload, metaJSON []byte
		if err := rows.Scan(&e.ID, &e.Kind, &e.TenantID, &wsID, &e.SourceService,
			&corrID, &causeID, &e.Timestamp, &payload, &metaJSON, &e.Version); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		if wsID.Valid {
			e.WorkspaceID = wsID.String
		}
		if corrID.Valid {
			e.CorrelationID = corrID.String
		}
		if causeID.Valid {
			e.CausationID = causeID.String
		}
		if len(payload) > 0 {
			e.Payload = json.RawMessage(payload)
		}
		if len(metaJSON) > 0 {
			_ = json.Unmarshal(metaJSON, &e.Metadata)
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

func scanPgRecipes(rows *sql.Rows) ([]models.Recipe, error) {
	var result []models.Recipe
	for rows.Next() {
		var r models.Recipe
		var triggerJSON []byte
		if err := rows.Scan(&r.ID, &r.TenantID, &r.WorkspaceID, &r.Name,
			&r.Description, &r.SafetySummary, &r.SquadKind, &r.Status,
			&triggerJSON, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan recipe: %w", err)
		}
		if len(triggerJSON) > 0 {
			_ = json.Unmarshal(triggerJSON, &r.TriggerConfig)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func scanPgApproval(row *sql.Row) (*models.ApprovalRequest, error) {
	var a models.ApprovalRequest
	var wsID, respondedBy, reason sql.NullString
	var respondedAt *time.Time
	err := row.Scan(&a.ID, &a.RecipeID, &a.StepID, &a.Description,
		&a.RiskLevel, &a.Status, &a.TenantID, &wsID,
		&a.RequestedAt, &respondedAt, &respondedBy, &reason)
	if err != nil {
		return nil, err
	}
	if wsID.Valid {
		a.WorkspaceID = wsID.String
	}
	a.RespondedAt = respondedAt
	if respondedBy.Valid {
		a.RespondedBy = respondedBy.String
	}
	if reason.Valid {
		a.Reason = reason.String
	}
	return &a, nil
}

func scanPgApprovals(rows *sql.Rows) ([]models.ApprovalRequest, error) {
	var result []models.ApprovalRequest
	for rows.Next() {
		var a models.ApprovalRequest
		var wsID, respondedBy, reason sql.NullString
		var respondedAt *time.Time
		if err := rows.Scan(&a.ID, &a.RecipeID, &a.StepID, &a.Description,
			&a.RiskLevel, &a.Status, &a.TenantID, &wsID,
			&a.RequestedAt, &respondedAt, &respondedBy, &reason); err != nil {
			return nil, fmt.Errorf("scan approval: %w", err)
		}
		if wsID.Valid {
			a.WorkspaceID = wsID.String
		}
		a.RespondedAt = respondedAt
		if respondedBy.Valid {
			a.RespondedBy = respondedBy.String
		}
		if reason.Valid {
			a.Reason = reason.String
		}
		result = append(result, a)
	}
	return result, rows.Err()
}
