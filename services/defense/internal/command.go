// Package internal implements dangerous command filtering for the IronGolem OS
// Defense service. It checks shell commands against deny patterns and manages
// approval workflows for blocked commands that may need admin override.
package internal

import (
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"time"
)

// CommandSeverity indicates how dangerous a denied command is.
type CommandSeverity string

const (
	CommandSeverityLow      CommandSeverity = "low"
	CommandSeverityMedium   CommandSeverity = "medium"
	CommandSeverityHigh     CommandSeverity = "high"
	CommandSeverityCritical CommandSeverity = "critical"
)

// DeniedCommand describes a pattern that should be blocked.
type DeniedCommand struct {
	Pattern          string          `json:"pattern"`
	Reason           string          `json:"reason"`
	Severity         CommandSeverity `json:"severity"`
	RequiresApproval bool            `json:"requires_approval"`
	compiled         *regexp.Regexp
}

// CommandCheckResult describes the outcome of a command check.
type CommandCheckResult struct {
	Allowed          bool            `json:"allowed"`
	Command          string          `json:"command"`
	MatchedPattern   string          `json:"matched_pattern,omitempty"`
	Reason           string          `json:"reason"`
	Severity         CommandSeverity `json:"severity,omitempty"`
	RequiresApproval bool            `json:"requires_approval"`
	ApprovalID       string          `json:"approval_id,omitempty"`
}

// ApprovalStatus tracks the state of a command approval request.
type ApprovalStatus string

const (
	ApprovalPending  ApprovalStatus = "pending"
	ApprovalApproved ApprovalStatus = "approved"
	ApprovalDenied   ApprovalStatus = "denied"
	ApprovalExpired  ApprovalStatus = "expired"
)

// CommandApprovalRequest represents a request for admin approval of a
// blocked command.
type CommandApprovalRequest struct {
	ID          string         `json:"id"`
	Command     string         `json:"command"`
	Pattern     string         `json:"pattern"`
	Reason      string         `json:"reason"`
	Severity    CommandSeverity `json:"severity"`
	RequestedBy string         `json:"requested_by"`
	RequestedAt time.Time      `json:"requested_at"`
	Status      ApprovalStatus `json:"status"`
	ReviewedBy  string         `json:"reviewed_by,omitempty"`
	ReviewedAt  *time.Time     `json:"reviewed_at,omitempty"`
	TTL         time.Duration  `json:"ttl_ns"`
}

// CommandAuditEntry records a single command execution attempt.
type CommandAuditEntry struct {
	ID        string          `json:"id"`
	Command   string          `json:"command"`
	UserID    string          `json:"user_id"`
	TenantID  string          `json:"tenant_id"`
	Allowed   bool            `json:"allowed"`
	Reason    string          `json:"reason"`
	Severity  CommandSeverity `json:"severity,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// defaultDenyCommands are the built-in dangerous command patterns.
var defaultDenyCommands = []DeniedCommand{
	{
		Pattern:          `rm\s+(-[a-zA-Z]*f[a-zA-Z]*\s+)?-[a-zA-Z]*r|rm\s+(-[a-zA-Z]*r[a-zA-Z]*\s+)?-[a-zA-Z]*f|rm\s+-rf|rm\s+-fr`,
		Reason:           "recursive force delete can destroy data irreversibly",
		Severity:         CommandSeverityCritical,
		RequiresApproval: true,
	},
	{
		Pattern:          `mkfs\b`,
		Reason:           "filesystem formatting destroys all data on the target device",
		Severity:         CommandSeverityCritical,
		RequiresApproval: false,
	},
	{
		Pattern:          `dd\s+.*of=`,
		Reason:           "raw disk write can overwrite critical data or devices",
		Severity:         CommandSeverityCritical,
		RequiresApproval: true,
	},
	{
		Pattern:          `chmod\s+777`,
		Reason:           "world-writable permissions are a security risk",
		Severity:         CommandSeverityHigh,
		RequiresApproval: true,
	},
	{
		Pattern:          `curl\s.*\|\s*(ba)?sh|wget\s.*\|\s*(ba)?sh`,
		Reason:           "piping remote content to shell enables arbitrary code execution",
		Severity:         CommandSeverityCritical,
		RequiresApproval: false,
	},
	{
		Pattern:          `>\s*/dev/sd[a-z]|>\s*/dev/nvme`,
		Reason:           "writing directly to block devices can destroy filesystems",
		Severity:         CommandSeverityCritical,
		RequiresApproval: false,
	},
	{
		Pattern:          `:(){ :\|:& };:`,
		Reason:           "fork bomb denial of service",
		Severity:         CommandSeverityCritical,
		RequiresApproval: false,
	},
	{
		Pattern:          `chmod\s+[0-7]*s|chmod\s+u\+s|chmod\s+g\+s`,
		Reason:           "setuid/setgid modification is a privilege escalation risk",
		Severity:         CommandSeverityHigh,
		RequiresApproval: true,
	},
	{
		Pattern:          `iptables\s+-F|iptables\s+--flush`,
		Reason:           "flushing firewall rules removes all network protections",
		Severity:         CommandSeverityHigh,
		RequiresApproval: true,
	},
	{
		Pattern:          `shutdown|reboot|init\s+[0-6]|poweroff`,
		Reason:           "system shutdown or reboot causes service interruption",
		Severity:         CommandSeverityMedium,
		RequiresApproval: true,
	},
}

// CommandFilter checks shell commands against deny patterns and manages
// the approval workflow for blocked commands that may need admin override.
type CommandFilter struct {
	logger   *slog.Logger
	patterns []DeniedCommand

	mu        sync.RWMutex
	approvals map[string]CommandApprovalRequest
	auditLog  []CommandAuditEntry
}

// NewCommandFilter creates a CommandFilter with the default deny patterns.
func NewCommandFilter(logger *slog.Logger) *CommandFilter {
	compiled := make([]DeniedCommand, len(defaultDenyCommands))
	for i, dc := range defaultDenyCommands {
		compiled[i] = dc
		compiled[i].compiled = regexp.MustCompile(dc.Pattern)
	}

	return &CommandFilter{
		logger:    logger,
		patterns:  compiled,
		approvals: make(map[string]CommandApprovalRequest),
		auditLog:  make([]CommandAuditEntry, 0),
	}
}

// AddPattern registers an additional deny pattern.
func (f *CommandFilter) AddPattern(dc DeniedCommand) error {
	compiled, err := regexp.Compile(dc.Pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern %q: %w", dc.Pattern, err)
	}
	dc.compiled = compiled

	f.mu.Lock()
	f.patterns = append(f.patterns, dc)
	f.mu.Unlock()

	f.logger.Info("command deny pattern added",
		slog.String("pattern", dc.Pattern),
		slog.String("severity", string(dc.Severity)),
	)

	return nil
}

// Check evaluates a command against all deny patterns and records the
// attempt in the audit log.
func (f *CommandFilter) Check(command, userID, tenantID string) CommandCheckResult {
	f.mu.RLock()
	patterns := make([]DeniedCommand, len(f.patterns))
	copy(patterns, f.patterns)
	f.mu.RUnlock()

	result := CommandCheckResult{
		Allowed: true,
		Command: command,
		Reason:  "no matching deny patterns",
	}

	for _, dc := range patterns {
		if dc.compiled != nil && dc.compiled.MatchString(command) {
			result.Allowed = false
			result.MatchedPattern = dc.Pattern
			result.Reason = dc.Reason
			result.Severity = dc.Severity
			result.RequiresApproval = dc.RequiresApproval

			// Create an approval request if the pattern allows it.
			if dc.RequiresApproval {
				approval := f.createApprovalRequest(command, dc, userID)
				result.ApprovalID = approval.ID
			}

			f.logger.Warn("dangerous command blocked",
				slog.String("command", truncate(command, 100)),
				slog.String("pattern", dc.Pattern),
				slog.String("severity", string(dc.Severity)),
				slog.String("user_id", userID),
			)

			break
		}
	}

	// Record in audit log.
	entry := CommandAuditEntry{
		ID:        generateID(),
		Command:   command,
		UserID:    userID,
		TenantID:  tenantID,
		Allowed:   result.Allowed,
		Reason:    result.Reason,
		Severity:  result.Severity,
		Timestamp: time.Now().UTC(),
	}

	f.mu.Lock()
	f.auditLog = append(f.auditLog, entry)
	f.mu.Unlock()

	return result
}

// createApprovalRequest creates a pending approval request for a blocked
// command.
func (f *CommandFilter) createApprovalRequest(command string, dc DeniedCommand, requestedBy string) CommandApprovalRequest {
	approval := CommandApprovalRequest{
		ID:          generateID(),
		Command:     command,
		Pattern:     dc.Pattern,
		Reason:      dc.Reason,
		Severity:    dc.Severity,
		RequestedBy: requestedBy,
		RequestedAt: time.Now().UTC(),
		Status:      ApprovalPending,
		TTL:         1 * time.Hour,
	}

	f.mu.Lock()
	f.approvals[approval.ID] = approval
	f.mu.Unlock()

	f.logger.Info("command approval request created",
		slog.String("id", approval.ID),
		slog.String("command", truncate(command, 100)),
		slog.String("requested_by", requestedBy),
	)

	return approval
}

// ApproveCommand approves a pending command approval request.
func (f *CommandFilter) ApproveCommand(approvalID, reviewer string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	approval, ok := f.approvals[approvalID]
	if !ok {
		return fmt.Errorf("approval request not found: %s", approvalID)
	}

	if approval.Status != ApprovalPending {
		return fmt.Errorf("approval request is not pending: %s", approval.Status)
	}

	now := time.Now().UTC()
	approval.Status = ApprovalApproved
	approval.ReviewedBy = reviewer
	approval.ReviewedAt = &now
	f.approvals[approvalID] = approval

	f.logger.Info("command approved",
		slog.String("id", approvalID),
		slog.String("reviewer", reviewer),
	)

	return nil
}

// DenyCommand denies a pending command approval request.
func (f *CommandFilter) DenyCommand(approvalID, reviewer string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	approval, ok := f.approvals[approvalID]
	if !ok {
		return fmt.Errorf("approval request not found: %s", approvalID)
	}

	if approval.Status != ApprovalPending {
		return fmt.Errorf("approval request is not pending: %s", approval.Status)
	}

	now := time.Now().UTC()
	approval.Status = ApprovalDenied
	approval.ReviewedBy = reviewer
	approval.ReviewedAt = &now
	f.approvals[approvalID] = approval

	f.logger.Info("command denied",
		slog.String("id", approvalID),
		slog.String("reviewer", reviewer),
	)

	return nil
}

// GetApproval returns an approval request by ID.
func (f *CommandFilter) GetApproval(id string) (CommandApprovalRequest, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	a, ok := f.approvals[id]
	return a, ok
}

// ListApprovals returns all approval requests.
func (f *CommandFilter) ListApprovals() []CommandApprovalRequest {
	f.mu.RLock()
	defer f.mu.RUnlock()
	result := make([]CommandApprovalRequest, 0, len(f.approvals))
	for _, a := range f.approvals {
		result = append(result, a)
	}
	return result
}

// AuditLog returns all command audit entries.
func (f *CommandFilter) AuditLog() []CommandAuditEntry {
	f.mu.RLock()
	defer f.mu.RUnlock()
	result := make([]CommandAuditEntry, len(f.auditLog))
	copy(result, f.auditLog)
	return result
}

// ExpireStaleApprovals expires pending approval requests past their TTL.
func (f *CommandFilter) ExpireStaleApprovals() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now().UTC()
	expired := 0

	for id, approval := range f.approvals {
		if approval.Status != ApprovalPending {
			continue
		}
		if now.After(approval.RequestedAt.Add(approval.TTL)) {
			approval.Status = ApprovalExpired
			f.approvals[id] = approval
			expired++
		}
	}

	return expired
}
