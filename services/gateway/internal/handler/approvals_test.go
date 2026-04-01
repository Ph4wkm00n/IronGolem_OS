package handler_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/handler"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/models"
)

// approvalTestEnv bundles the test server and approval store so tests can
// seed data directly.
type approvalTestEnv struct {
	srv   *httptest.Server
	store *handler.InMemoryApprovalStore
}

func newApprovalTestEnv(t *testing.T) *approvalTestEnv {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	eventStore := handler.NewInMemoryEventStore()
	approvalStore := handler.NewInMemoryApprovalStore()
	ah := handler.NewApprovalHandler(logger, approvalStore, eventStore)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/approvals", ah.ListApprovals)
	mux.HandleFunc("GET /api/v1/approvals/{id}", ah.GetApproval)
	mux.HandleFunc("POST /api/v1/approvals/{id}/approve", ah.ApproveAction)
	mux.HandleFunc("POST /api/v1/approvals/{id}/deny", ah.DenyAction)

	return &approvalTestEnv{
		srv:   httptest.NewServer(mux),
		store: approvalStore,
	}
}

// seedApproval creates a pending approval in the store and returns its ID.
func (env *approvalTestEnv) seedApproval(t *testing.T) string {
	t.Helper()

	req := models.ApprovalRequest{
		ID:          "approval-test-001",
		RecipeID:    "recipe-email-triage-001",
		StepID:      "step-et-004",
		Description: "Send drafted replies after approval",
		RiskLevel:   models.RiskLevelHigh,
		Status:      models.ApprovalStatusPending,
		TenantID:    "tenant-001",
		RequestedAt: time.Now().UTC(),
	}
	env.store.Create(req)
	return req.ID
}

func TestListApprovalsEmpty(t *testing.T) {
	env := newApprovalTestEnv(t)
	defer env.srv.Close()

	resp, err := http.Get(env.srv.URL + "/api/v1/approvals")
	if err != nil {
		t.Fatalf("GET /api/v1/approvals failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body struct {
		Approvals []models.ApprovalRequest `json:"approvals"`
		Total     int                      `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Total != 0 {
		t.Errorf("expected 0 approvals, got %d", body.Total)
	}
	if body.Approvals != nil && len(body.Approvals) != 0 {
		t.Errorf("expected empty approvals list, got %d items", len(body.Approvals))
	}
}

func TestApproveAction(t *testing.T) {
	env := newApprovalTestEnv(t)
	defer env.srv.Close()

	approvalID := env.seedApproval(t)

	reqBody, _ := json.Marshal(map[string]string{
		"responded_by": "admin-user",
	})

	resp, err := http.Post(
		env.srv.URL+"/api/v1/approvals/"+approvalID+"/approve",
		"application/json",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		t.Fatalf("POST approve failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body struct {
		Approval models.ApprovalRequest `json:"approval"`
		Status   string                 `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Status != "approved" {
		t.Errorf("expected status %q, got %q", "approved", body.Status)
	}
	if body.Approval.Status != models.ApprovalStatusApproved {
		t.Errorf("expected approval status %q, got %q", models.ApprovalStatusApproved, body.Approval.Status)
	}
	if body.Approval.RespondedBy != "admin-user" {
		t.Errorf("expected responded_by %q, got %q", "admin-user", body.Approval.RespondedBy)
	}
	if body.Approval.RespondedAt == nil {
		t.Error("expected non-nil responded_at")
	}
}

func TestDenyAction(t *testing.T) {
	env := newApprovalTestEnv(t)
	defer env.srv.Close()

	approvalID := env.seedApproval(t)

	reqBody, _ := json.Marshal(map[string]string{
		"responded_by": "security-admin",
		"reason":       "risk too high for automated execution",
	})

	resp, err := http.Post(
		env.srv.URL+"/api/v1/approvals/"+approvalID+"/deny",
		"application/json",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		t.Fatalf("POST deny failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body struct {
		Approval models.ApprovalRequest `json:"approval"`
		Status   string                 `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Status != "denied" {
		t.Errorf("expected status %q, got %q", "denied", body.Status)
	}
	if body.Approval.Status != models.ApprovalStatusDenied {
		t.Errorf("expected approval status %q, got %q", models.ApprovalStatusDenied, body.Approval.Status)
	}
	if body.Approval.Reason != "risk too high for automated execution" {
		t.Errorf("expected reason to be saved, got %q", body.Approval.Reason)
	}
	if body.Approval.RespondedBy != "security-admin" {
		t.Errorf("expected responded_by %q, got %q", "security-admin", body.Approval.RespondedBy)
	}
}

func TestDenyRequiresReason(t *testing.T) {
	env := newApprovalTestEnv(t)
	defer env.srv.Close()

	approvalID := env.seedApproval(t)

	// Send deny without a reason field.
	reqBody, _ := json.Marshal(map[string]string{
		"responded_by": "admin",
	})

	resp, err := http.Post(
		env.srv.URL+"/api/v1/approvals/"+approvalID+"/deny",
		"application/json",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		t.Fatalf("POST deny failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body["error"] != "reason is required when denying an approval" {
		t.Errorf("expected reason-required error, got %q", body["error"])
	}
}
