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

// sqliteSchema creates all tables needed by the SQLite store.
const sqliteSchema = `
CREATE TABLE IF NOT EXISTS tenants (
	id              TEXT PRIMARY KEY NOT NULL,
	name            TEXT NOT NULL,
	deployment_mode TEXT NOT NULL DEFAULT 'solo',
	created_at      TEXT NOT NULL,
	updated_at      TEXT NOT NULL,
	disabled        INTEGER NOT NULL DEFAULT 0,
	metadata        TEXT
);

CREATE TABLE IF NOT EXISTS workspaces (
	id         TEXT PRIMARY KEY NOT NULL,
	tenant_id  TEXT NOT NULL,
	name       TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	disabled   INTEGER NOT NULL DEFAULT 0,
	FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);

CREATE TABLE IF NOT EXISTS members (
	id           TEXT PRIMARY KEY NOT NULL,
	tenant_id    TEXT NOT NULL,
	workspace_id TEXT NOT NULL,
	email        TEXT NOT NULL,
	display_name TEXT NOT NULL,
	role         TEXT NOT NULL DEFAULT 'member',
	created_at   TEXT NOT NULL,
	last_seen_at TEXT NOT NULL,
	disabled     INTEGER NOT NULL DEFAULT 0,
	FOREIGN KEY (tenant_id) REFERENCES tenants(id),
	FOREIGN KEY (workspace_id) REFERENCES workspaces(id)
);

CREATE TABLE IF NOT EXISTS events (
	id             TEXT PRIMARY KEY NOT NULL,
	kind           TEXT NOT NULL,
	tenant_id      TEXT NOT NULL,
	workspace_id   TEXT,
	source_service TEXT NOT NULL,
	correlation_id TEXT,
	causation_id   TEXT,
	timestamp      TEXT NOT NULL,
	payload        TEXT,
	metadata       TEXT,
	version        INTEGER NOT NULL DEFAULT 1,
	FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);
CREATE INDEX IF NOT EXISTS idx_events_tenant_ts ON events(tenant_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_events_workspace ON events(workspace_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_events_kind ON events(tenant_id, kind);

CREATE TABLE IF NOT EXISTS recipes (
	id             TEXT PRIMARY KEY NOT NULL,
	tenant_id      TEXT NOT NULL,
	workspace_id   TEXT NOT NULL,
	name           TEXT NOT NULL,
	description    TEXT NOT NULL DEFAULT '',
	safety_summary TEXT NOT NULL DEFAULT '',
	squad_kind     TEXT NOT NULL DEFAULT '',
	status         TEXT NOT NULL DEFAULT 'draft',
	trigger_config TEXT,
	created_by     TEXT NOT NULL DEFAULT '',
	created_at     TEXT NOT NULL,
	updated_at     TEXT NOT NULL,
	FOREIGN KEY (tenant_id) REFERENCES tenants(id),
	FOREIGN KEY (workspace_id) REFERENCES workspaces(id)
);
CREATE INDEX IF NOT EXISTS idx_recipes_tenant_ws ON recipes(tenant_id, workspace_id);

CREATE TABLE IF NOT EXISTS approvals (
	id           TEXT PRIMARY KEY NOT NULL,
	recipe_id    TEXT NOT NULL,
	step_id      TEXT NOT NULL,
	description  TEXT NOT NULL DEFAULT '',
	risk_level   TEXT NOT NULL DEFAULT 'low',
	status       TEXT NOT NULL DEFAULT 'pending',
	tenant_id    TEXT NOT NULL,
	workspace_id TEXT,
	requested_at TEXT NOT NULL,
	responded_at TEXT,
	responded_by TEXT,
	reason       TEXT,
	FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);
CREATE INDEX IF NOT EXISTS idx_approvals_tenant ON approvals(tenant_id, status);
`

// SqliteStore implements Store using SQLite via database/sql.
type SqliteStore struct {
	db *sql.DB
}

// NewSqliteStore opens a SQLite database and initializes the schema.
func NewSqliteStore(cfg Config) (*SqliteStore, error) {
	dsn := cfg.DSN
	if dsn == "" {
		dsn = ":memory:"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}
	// SQLite should not use multiple connections (single-writer).
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(sqliteSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite schema init: %w", err)
	}
	return &SqliteStore{db: db}, nil
}

// Close releases the database connection.
func (s *SqliteStore) Close() error {
	return s.db.Close()
}

// --- Events ---

func (s *SqliteStore) SaveEvent(ctx context.Context, event *events.Event) error {
	payload := string(event.Payload)
	metaJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO events (id, kind, tenant_id, workspace_id, source_service, correlation_id, causation_id, timestamp, payload, metadata, version)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID, string(event.Kind), event.TenantID, event.WorkspaceID,
		event.SourceService, event.CorrelationID, event.CausationID,
		event.Timestamp.Format(time.RFC3339Nano), payload, string(metaJSON), event.Version,
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

func (s *SqliteStore) ListEvents(ctx context.Context, tenantID, workspaceID string, limit int) ([]events.Event, error) {
	var rows *sql.Rows
	var err error
	if workspaceID != "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, kind, tenant_id, workspace_id, source_service, correlation_id, causation_id, timestamp, payload, metadata, version
			 FROM events WHERE tenant_id = ? AND workspace_id = ? ORDER BY timestamp DESC LIMIT ?`,
			tenantID, workspaceID, limit)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, kind, tenant_id, workspace_id, source_service, correlation_id, causation_id, timestamp, payload, metadata, version
			 FROM events WHERE tenant_id = ? ORDER BY timestamp DESC LIMIT ?`,
			tenantID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()
	return scanEvents(rows)
}

// --- Recipes ---

func (s *SqliteStore) SaveRecipe(ctx context.Context, recipe *models.Recipe) error {
	triggerJSON, err := json.Marshal(recipe.TriggerConfig)
	if err != nil {
		return fmt.Errorf("marshal trigger config: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO recipes (id, tenant_id, workspace_id, name, description, safety_summary, squad_kind, status, trigger_config, created_by, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET name=excluded.name, description=excluded.description,
		   safety_summary=excluded.safety_summary, squad_kind=excluded.squad_kind,
		   status=excluded.status, trigger_config=excluded.trigger_config, updated_at=excluded.updated_at`,
		recipe.ID, recipe.TenantID, recipe.WorkspaceID, recipe.Name,
		recipe.Description, recipe.SafetySummary, string(recipe.SquadKind),
		string(recipe.Status), string(triggerJSON), recipe.CreatedBy,
		recipe.CreatedAt.Format(time.RFC3339Nano), recipe.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("save recipe: %w", err)
	}
	return nil
}

func (s *SqliteStore) ListRecipes(ctx context.Context, tenantID, workspaceID string) ([]models.Recipe, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, workspace_id, name, description, safety_summary, squad_kind, status, trigger_config, created_by, created_at, updated_at
		 FROM recipes WHERE tenant_id = ? AND workspace_id = ?`,
		tenantID, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list recipes: %w", err)
	}
	defer rows.Close()
	return scanRecipes(rows)
}

// --- Approvals ---

func (s *SqliteStore) SaveApproval(ctx context.Context, a *models.ApprovalRequest) error {
	var respondedAt *string
	if a.RespondedAt != nil {
		v := a.RespondedAt.Format(time.RFC3339Nano)
		respondedAt = &v
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO approvals (id, recipe_id, step_id, description, risk_level, status, tenant_id, workspace_id, requested_at, responded_at, responded_by, reason)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET status=excluded.status, responded_at=excluded.responded_at,
		   responded_by=excluded.responded_by, reason=excluded.reason`,
		a.ID, a.RecipeID, a.StepID, a.Description, string(a.RiskLevel),
		string(a.Status), a.TenantID, a.WorkspaceID,
		a.RequestedAt.Format(time.RFC3339Nano), respondedAt, a.RespondedBy, a.Reason,
	)
	if err != nil {
		return fmt.Errorf("save approval: %w", err)
	}
	return nil
}

func (s *SqliteStore) GetApproval(ctx context.Context, id string) (*models.ApprovalRequest, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, recipe_id, step_id, description, risk_level, status, tenant_id, workspace_id, requested_at, responded_at, responded_by, reason
		 FROM approvals WHERE id = ?`, id)
	a, err := scanApproval(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get approval: %w", err)
	}
	return a, nil
}

func (s *SqliteStore) ListPendingApprovals(ctx context.Context, tenantID string) ([]models.ApprovalRequest, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, recipe_id, step_id, description, risk_level, status, tenant_id, workspace_id, requested_at, responded_at, responded_by, reason
		 FROM approvals WHERE tenant_id = ? AND status = 'pending' ORDER BY requested_at DESC`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list pending approvals: %w", err)
	}
	defer rows.Close()
	return scanApprovals(rows)
}

// --- Workspaces ---

func (s *SqliteStore) SaveWorkspace(ctx context.Context, ws *models.Workspace) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO workspaces (id, tenant_id, name, created_at, updated_at, disabled)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET name=excluded.name, updated_at=excluded.updated_at, disabled=excluded.disabled`,
		ws.ID, ws.TenantID, ws.Name,
		ws.CreatedAt.Format(time.RFC3339Nano), ws.UpdatedAt.Format(time.RFC3339Nano),
		boolToInt(ws.Disabled),
	)
	if err != nil {
		return fmt.Errorf("save workspace: %w", err)
	}
	return nil
}

func (s *SqliteStore) ListWorkspaces(ctx context.Context, tenantID string) ([]models.Workspace, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, name, created_at, updated_at, disabled
		 FROM workspaces WHERE tenant_id = ?`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	defer rows.Close()

	var result []models.Workspace
	for rows.Next() {
		var ws models.Workspace
		var createdAt, updatedAt string
		var disabled int
		if err := rows.Scan(&ws.ID, &ws.TenantID, &ws.Name, &createdAt, &updatedAt, &disabled); err != nil {
			return nil, fmt.Errorf("scan workspace: %w", err)
		}
		ws.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		ws.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		ws.Disabled = disabled != 0
		result = append(result, ws)
	}
	return result, rows.Err()
}

// --- Tenants ---

func (s *SqliteStore) SaveTenant(ctx context.Context, tenant *models.Tenant) error {
	metaJSON, err := json.Marshal(tenant.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO tenants (id, name, deployment_mode, created_at, updated_at, disabled, metadata)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET name=excluded.name, deployment_mode=excluded.deployment_mode,
		   updated_at=excluded.updated_at, disabled=excluded.disabled, metadata=excluded.metadata`,
		tenant.ID, tenant.Name, string(tenant.DeploymentMode),
		tenant.CreatedAt.Format(time.RFC3339Nano), tenant.UpdatedAt.Format(time.RFC3339Nano),
		boolToInt(tenant.Disabled), string(metaJSON),
	)
	if err != nil {
		return fmt.Errorf("save tenant: %w", err)
	}
	return nil
}

func (s *SqliteStore) GetTenant(ctx context.Context, id string) (*models.Tenant, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, deployment_mode, created_at, updated_at, disabled, metadata
		 FROM tenants WHERE id = ?`, id)

	var t models.Tenant
	var createdAt, updatedAt string
	var disabled int
	var metaJSON sql.NullString
	err := row.Scan(&t.ID, &t.Name, &t.DeploymentMode, &createdAt, &updatedAt, &disabled, &metaJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get tenant: %w", err)
	}
	t.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	t.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	t.Disabled = disabled != 0
	if metaJSON.Valid && metaJSON.String != "" {
		_ = json.Unmarshal([]byte(metaJSON.String), &t.Metadata)
	}
	return &t, nil
}

// --- Members ---

func (s *SqliteStore) SaveMember(ctx context.Context, user *models.User) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO members (id, tenant_id, workspace_id, email, display_name, role, created_at, last_seen_at, disabled)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET email=excluded.email, display_name=excluded.display_name,
		   role=excluded.role, last_seen_at=excluded.last_seen_at, disabled=excluded.disabled`,
		user.ID, user.TenantID, user.WorkspaceID, user.Email, user.DisplayName,
		string(user.Role), user.CreatedAt.Format(time.RFC3339Nano),
		user.LastSeenAt.Format(time.RFC3339Nano), boolToInt(user.Disabled),
	)
	if err != nil {
		return fmt.Errorf("save member: %w", err)
	}
	return nil
}

func (s *SqliteStore) ListMembers(ctx context.Context, tenantID, workspaceID string) ([]models.User, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, workspace_id, email, display_name, role, created_at, last_seen_at, disabled
		 FROM members WHERE tenant_id = ? AND workspace_id = ?`, tenantID, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()

	var result []models.User
	for rows.Next() {
		var u models.User
		var createdAt, lastSeen string
		var disabled int
		if err := rows.Scan(&u.ID, &u.TenantID, &u.WorkspaceID, &u.Email, &u.DisplayName, &u.Role, &createdAt, &lastSeen, &disabled); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		u.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		u.LastSeenAt, _ = time.Parse(time.RFC3339Nano, lastSeen)
		u.Disabled = disabled != 0
		result = append(result, u)
	}
	return result, rows.Err()
}

// --- Helpers ---

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func scanEvents(rows *sql.Rows) ([]events.Event, error) {
	var result []events.Event
	for rows.Next() {
		var e events.Event
		var ts string
		var payload, metaJSON sql.NullString
		var wsID, corrID, causeID sql.NullString
		if err := rows.Scan(&e.ID, &e.Kind, &e.TenantID, &wsID, &e.SourceService,
			&corrID, &causeID, &ts, &payload, &metaJSON, &e.Version); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		e.Timestamp, _ = time.Parse(time.RFC3339Nano, ts)
		if wsID.Valid {
			e.WorkspaceID = wsID.String
		}
		if corrID.Valid {
			e.CorrelationID = corrID.String
		}
		if causeID.Valid {
			e.CausationID = causeID.String
		}
		if payload.Valid {
			e.Payload = json.RawMessage(payload.String)
		}
		if metaJSON.Valid && metaJSON.String != "" {
			_ = json.Unmarshal([]byte(metaJSON.String), &e.Metadata)
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

func scanRecipes(rows *sql.Rows) ([]models.Recipe, error) {
	var result []models.Recipe
	for rows.Next() {
		var r models.Recipe
		var createdAt, updatedAt string
		var triggerJSON sql.NullString
		if err := rows.Scan(&r.ID, &r.TenantID, &r.WorkspaceID, &r.Name,
			&r.Description, &r.SafetySummary, &r.SquadKind, &r.Status,
			&triggerJSON, &r.CreatedBy, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan recipe: %w", err)
		}
		r.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		r.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		if triggerJSON.Valid && triggerJSON.String != "" {
			_ = json.Unmarshal([]byte(triggerJSON.String), &r.TriggerConfig)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func scanApproval(row *sql.Row) (*models.ApprovalRequest, error) {
	var a models.ApprovalRequest
	var requestedAt string
	var respondedAt, respondedBy, reason, wsID sql.NullString
	err := row.Scan(&a.ID, &a.RecipeID, &a.StepID, &a.Description,
		&a.RiskLevel, &a.Status, &a.TenantID, &wsID,
		&requestedAt, &respondedAt, &respondedBy, &reason)
	if err != nil {
		return nil, err
	}
	a.RequestedAt, _ = time.Parse(time.RFC3339Nano, requestedAt)
	if wsID.Valid {
		a.WorkspaceID = wsID.String
	}
	if respondedAt.Valid {
		t, _ := time.Parse(time.RFC3339Nano, respondedAt.String)
		a.RespondedAt = &t
	}
	if respondedBy.Valid {
		a.RespondedBy = respondedBy.String
	}
	if reason.Valid {
		a.Reason = reason.String
	}
	return &a, nil
}

func scanApprovals(rows *sql.Rows) ([]models.ApprovalRequest, error) {
	var result []models.ApprovalRequest
	for rows.Next() {
		var a models.ApprovalRequest
		var requestedAt string
		var respondedAt, respondedBy, reason, wsID sql.NullString
		if err := rows.Scan(&a.ID, &a.RecipeID, &a.StepID, &a.Description,
			&a.RiskLevel, &a.Status, &a.TenantID, &wsID,
			&requestedAt, &respondedAt, &respondedBy, &reason); err != nil {
			return nil, fmt.Errorf("scan approval: %w", err)
		}
		a.RequestedAt, _ = time.Parse(time.RFC3339Nano, requestedAt)
		if wsID.Valid {
			a.WorkspaceID = wsID.String
		}
		if respondedAt.Valid {
			t, _ := time.Parse(time.RFC3339Nano, respondedAt.String)
			a.RespondedAt = &t
		}
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
