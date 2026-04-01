// Package internal implements destination allowlist management for the
// IronGolem OS Defense service. It provides per-workspace allow/deny lists
// for network destinations with a default deny list for dangerous endpoints.
package internal

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"
)

// AllowlistEntryType indicates whether an entry allows or denies access.
type AllowlistEntryType string

const (
	EntryTypeAllow AllowlistEntryType = "allow"
	EntryTypeDeny  AllowlistEntryType = "deny"
)

// AllowlistEntry represents a single allow or deny rule for a network
// destination.
type AllowlistEntry struct {
	ID          string             `json:"id"`
	Pattern     string             `json:"pattern"`
	Type        AllowlistEntryType `json:"type"`
	WorkspaceID string             `json:"workspace_id"`
	CreatedAt   time.Time          `json:"created_at"`
	CreatedBy   string             `json:"created_by"`
	Description string             `json:"description,omitempty"`
}

// AllowlistCheckResult describes the outcome of a destination check.
type AllowlistCheckResult struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason"`
	MatchedRule string `json:"matched_rule,omitempty"`
}

// AllowlistStore defines persistence operations for allowlist entries.
type AllowlistStore interface {
	Save(entry AllowlistEntry) error
	Get(id string) (AllowlistEntry, bool)
	List() []AllowlistEntry
	ListByWorkspace(workspaceID string) []AllowlistEntry
	Delete(id string) error
}

// InMemoryAllowlistStore is a thread-safe in-memory AllowlistStore.
type InMemoryAllowlistStore struct {
	mu      sync.RWMutex
	entries map[string]AllowlistEntry
}

// NewInMemoryAllowlistStore creates a new in-memory allowlist store.
func NewInMemoryAllowlistStore() *InMemoryAllowlistStore {
	return &InMemoryAllowlistStore{
		entries: make(map[string]AllowlistEntry),
	}
}

// Save persists an allowlist entry.
func (s *InMemoryAllowlistStore) Save(entry AllowlistEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[entry.ID] = entry
	return nil
}

// Get retrieves an allowlist entry by ID.
func (s *InMemoryAllowlistStore) Get(id string) (AllowlistEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[id]
	return e, ok
}

// List returns all allowlist entries.
func (s *InMemoryAllowlistStore) List() []AllowlistEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]AllowlistEntry, 0, len(s.entries))
	for _, e := range s.entries {
		result = append(result, e)
	}
	return result
}

// ListByWorkspace returns entries for a specific workspace.
func (s *InMemoryAllowlistStore) ListByWorkspace(workspaceID string) []AllowlistEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []AllowlistEntry
	for _, e := range s.entries {
		if e.WorkspaceID == workspaceID {
			result = append(result, e)
		}
	}
	return result
}

// Delete removes an allowlist entry by ID.
func (s *InMemoryAllowlistStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, id)
	return nil
}

// defaultDenyPatterns are destinations that are always blocked regardless of
// workspace configuration.
var defaultDenyPatterns = []AllowlistEntry{
	{
		ID:          "default-deny-metadata-aws",
		Pattern:     "169.254.169.254",
		Type:        EntryTypeDeny,
		Description: "AWS metadata endpoint",
	},
	{
		ID:          "default-deny-metadata-gcp",
		Pattern:     "metadata.google.internal",
		Type:        EntryTypeDeny,
		Description: "GCP metadata endpoint",
	},
	{
		ID:          "default-deny-metadata-azure",
		Pattern:     "169.254.169.254",
		Type:        EntryTypeDeny,
		Description: "Azure metadata endpoint",
	},
	{
		ID:          "default-deny-loopback",
		Pattern:     "127.0.0.0/8",
		Type:        EntryTypeDeny,
		Description: "Loopback addresses",
	},
	{
		ID:          "default-deny-private-10",
		Pattern:     "10.0.0.0/8",
		Type:        EntryTypeDeny,
		Description: "RFC 1918 private network (10.x)",
	},
	{
		ID:          "default-deny-private-172",
		Pattern:     "172.16.0.0/12",
		Type:        EntryTypeDeny,
		Description: "RFC 1918 private network (172.16.x)",
	},
	{
		ID:          "default-deny-private-192",
		Pattern:     "192.168.0.0/16",
		Type:        EntryTypeDeny,
		Description: "RFC 1918 private network (192.168.x)",
	},
	{
		ID:          "default-deny-link-local",
		Pattern:     "169.254.0.0/16",
		Type:        EntryTypeDeny,
		Description: "Link-local addresses",
	},
}

// AllowlistManager manages allowed and denied network destinations per
// workspace with a built-in default deny list for dangerous endpoints.
type AllowlistManager struct {
	logger *slog.Logger
	store  AllowlistStore
}

// NewAllowlistManager creates an AllowlistManager with the given dependencies.
func NewAllowlistManager(logger *slog.Logger, store AllowlistStore) *AllowlistManager {
	return &AllowlistManager{
		logger: logger,
		store:  store,
	}
}

// AddEntry adds an allow or deny entry for a workspace.
func (m *AllowlistManager) AddEntry(entry AllowlistEntry) (AllowlistEntry, error) {
	if entry.ID == "" {
		entry.ID = generateID()
	}
	entry.CreatedAt = time.Now().UTC()

	if err := m.store.Save(entry); err != nil {
		return AllowlistEntry{}, fmt.Errorf("saving allowlist entry: %w", err)
	}

	m.logger.Info("allowlist entry added",
		slog.String("id", entry.ID),
		slog.String("pattern", entry.Pattern),
		slog.String("type", string(entry.Type)),
		slog.String("workspace_id", entry.WorkspaceID),
	)

	return entry, nil
}

// RemoveEntry deletes an allowlist entry.
func (m *AllowlistManager) RemoveEntry(id string) error {
	return m.store.Delete(id)
}

// List returns all entries, optionally filtered by workspace.
func (m *AllowlistManager) List(workspaceID string) []AllowlistEntry {
	if workspaceID != "" {
		return m.store.ListByWorkspace(workspaceID)
	}
	return m.store.List()
}

// CheckDestination evaluates whether a URL is allowed for the given workspace.
// Deny rules (including defaults) are checked first. If any deny rule matches,
// the destination is blocked. Then allow rules are checked; if allow rules
// exist and none match, the destination is also blocked.
func (m *AllowlistManager) CheckDestination(rawURL, workspaceID string) AllowlistCheckResult {
	// Parse the URL to extract the hostname.
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return AllowlistCheckResult{
			Allowed: false,
			Reason:  "malformed URL",
		}
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		return AllowlistCheckResult{
			Allowed: false,
			Reason:  "empty hostname",
		}
	}

	// Check default deny list first.
	for _, deny := range defaultDenyPatterns {
		if matchesPattern(hostname, deny.Pattern) {
			return AllowlistCheckResult{
				Allowed:     false,
				Reason:      fmt.Sprintf("blocked by default deny rule: %s", deny.Description),
				MatchedRule: deny.ID,
			}
		}
	}

	// Check workspace-specific rules.
	entries := m.store.ListByWorkspace(workspaceID)

	// Check explicit deny rules.
	for _, entry := range entries {
		if entry.Type == EntryTypeDeny && matchesPattern(hostname, entry.Pattern) {
			return AllowlistCheckResult{
				Allowed:     false,
				Reason:      fmt.Sprintf("blocked by workspace deny rule: %s", entry.Pattern),
				MatchedRule: entry.ID,
			}
		}
	}

	// Check allow rules. If allow rules exist, only matching destinations
	// are permitted.
	var hasAllowRules bool
	for _, entry := range entries {
		if entry.Type == EntryTypeAllow {
			hasAllowRules = true
			if matchesPattern(hostname, entry.Pattern) {
				return AllowlistCheckResult{
					Allowed:     true,
					Reason:      fmt.Sprintf("allowed by workspace rule: %s", entry.Pattern),
					MatchedRule: entry.ID,
				}
			}
		}
	}

	if hasAllowRules {
		return AllowlistCheckResult{
			Allowed: false,
			Reason:  "destination not in workspace allowlist",
		}
	}

	// No explicit allow rules: permit by default (deny rules already checked).
	return AllowlistCheckResult{
		Allowed: true,
		Reason:  "no restrictions configured for workspace",
	}
}

// matchesPattern checks whether a hostname matches a pattern, supporting
// exact domain matches, wildcard prefixes (*.example.com), and CIDR ranges.
func matchesPattern(hostname, pattern string) bool {
	hostname = strings.ToLower(hostname)
	pattern = strings.ToLower(pattern)

	// Exact match.
	if hostname == pattern {
		return true
	}

	// Wildcard match (*.example.com matches sub.example.com).
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // ".example.com"
		if strings.HasSuffix(hostname, suffix) {
			return true
		}
	}

	// Suffix match (example.com matches sub.example.com).
	if strings.HasSuffix(hostname, "."+pattern) {
		return true
	}

	// CIDR match.
	_, cidr, err := net.ParseCIDR(pattern)
	if err == nil {
		ip := net.ParseIP(hostname)
		if ip != nil && cidr.Contains(ip) {
			return true
		}
	}

	return false
}
