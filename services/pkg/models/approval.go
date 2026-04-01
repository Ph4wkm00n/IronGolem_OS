// Package models - approval.go defines the ApprovalRequest model and status
// types for the IronGolem OS approval workflow.
//
// Approval requests are generated when a recipe step with RequiresApproval=true
// is about to execute, or when the policy engine flags an action for review.
// Every approval decision is recorded in the event log for the audit trail.
package models

import "time"

// ApprovalStatus represents the lifecycle state of an approval request.
type ApprovalStatus string

const (
	ApprovalStatusPending  ApprovalStatus = "pending"
	ApprovalStatusApproved ApprovalStatus = "approved"
	ApprovalStatusDenied   ApprovalStatus = "denied"
	ApprovalStatusExpired  ApprovalStatus = "expired"
)

// ApprovalRequest captures a pending decision that requires human review
// before an autonomous action can proceed.
type ApprovalRequest struct {
	// ID is the unique identifier for this approval request.
	ID string `json:"id"`

	// RecipeID links this approval to the recipe that triggered it.
	RecipeID string `json:"recipe_id"`

	// StepID identifies the specific recipe step awaiting approval.
	StepID string `json:"step_id"`

	// Description is a human-readable explanation of what will happen
	// if this request is approved.
	Description string `json:"description"`

	// RiskLevel indicates how risky the pending action is.
	RiskLevel RiskLevel `json:"risk_level"`

	// Status tracks whether the request is pending, approved, denied, or expired.
	Status ApprovalStatus `json:"status"`

	// TenantID scopes this request to a tenant for isolation.
	TenantID string `json:"tenant_id"`

	// WorkspaceID scopes this request within a tenant.
	WorkspaceID string `json:"workspace_id,omitempty"`

	// RequestedAt records when the approval was requested.
	RequestedAt time.Time `json:"requested_at"`

	// RespondedAt records when a decision was made (zero if still pending).
	RespondedAt *time.Time `json:"responded_at,omitempty"`

	// RespondedBy identifies who approved or denied the request.
	RespondedBy string `json:"responded_by,omitempty"`

	// Reason captures the justification for a denial.
	Reason string `json:"reason,omitempty"`
}
