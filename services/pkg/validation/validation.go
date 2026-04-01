// Package validation provides input validation and sanitization helpers
// for the IronGolem OS services. These functions should be used at API
// boundaries to ensure all user input is validated before processing.
package validation

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

// MaxStringLength is the default maximum length for sanitized strings.
const MaxStringLength = 10000

// MaxJSONPayloadBytes is the default maximum size for JSON payloads (1 MB).
const MaxJSONPayloadBytes = 1 << 20

// MaxPageSize is the upper bound for pagination page sizes.
const MaxPageSize = 100

// MinPage is the minimum page number.
const MinPage = 1

// emailRegex is a simplified but practical email validation pattern.
// It covers the vast majority of valid email addresses without being
// overly permissive.
var emailRegex = regexp.MustCompile(
	`^[a-zA-Z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`,
)

// uuidRegex matches UUID v4 format (with or without hyphens).
var uuidRegex = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`,
)

// ValidateEmail checks that the given string is a plausible email address.
// It does not verify deliverability.
func ValidateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email address is required")
	}
	if len(email) > 254 {
		return fmt.Errorf("email address exceeds maximum length of 254 characters")
	}
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email address format")
	}
	return nil
}

// ValidateURL checks that the given string is a valid HTTP or HTTPS URL.
// It rejects non-HTTP schemes, empty hosts, and URLs that are too long.
func ValidateURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("URL is required")
	}
	if len(rawURL) > 2048 {
		return fmt.Errorf("URL exceeds maximum length of 2048 characters")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https, got %q", parsed.Scheme)
	}

	if parsed.Host == "" {
		return fmt.Errorf("URL must have a host")
	}

	return nil
}

// ValidateUUID checks that the given string is a valid UUID (v4 format with hyphens).
func ValidateUUID(id string) error {
	if id == "" {
		return fmt.Errorf("UUID is required")
	}
	if !uuidRegex.MatchString(id) {
		return fmt.Errorf("invalid UUID format: %q", id)
	}
	return nil
}

// SanitizeString strips control characters (except newline and tab),
// trims leading/trailing whitespace, and enforces a maximum length.
// If maxLen is 0 or negative, MaxStringLength is used.
func SanitizeString(s string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = MaxStringLength
	}

	// Strip control characters except newline (\n) and tab (\t).
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1 // drop the character
		}
		return r
	}, s)

	// Trim whitespace.
	cleaned = strings.TrimSpace(cleaned)

	// Enforce max length (rune-aware to avoid splitting multi-byte chars).
	runes := []rune(cleaned)
	if len(runes) > maxLen {
		runes = runes[:maxLen]
	}

	return string(runes)
}

// PaginationParams holds validated pagination parameters.
type PaginationParams struct {
	Page     int
	PageSize int
}

// ValidatePagination ensures page and pageSize are within acceptable bounds.
// It returns corrected values, clamping to [1, MaxPageSize] for pageSize
// and [1, ...] for page.
func ValidatePagination(page, pageSize int) PaginationParams {
	if page < MinPage {
		page = MinPage
	}

	if pageSize < 1 {
		pageSize = 20 // default
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}

	return PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}
}

// ValidateJSON checks that the given byte slice is valid JSON and does not
// exceed maxBytes. If maxBytes is 0 or negative, MaxJSONPayloadBytes is used.
func ValidateJSON(data []byte, maxBytes int) error {
	if maxBytes <= 0 {
		maxBytes = MaxJSONPayloadBytes
	}

	if len(data) > maxBytes {
		return fmt.Errorf("JSON payload exceeds maximum size of %d bytes", maxBytes)
	}

	if len(data) == 0 {
		return fmt.Errorf("JSON payload is empty")
	}

	if !json.Valid(data) {
		return fmt.Errorf("invalid JSON payload")
	}

	return nil
}

// ValidateStringNotEmpty checks that a named string field is not empty after
// trimming whitespace.
func ValidateStringNotEmpty(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

// ValidateStringMaxLength checks that a string does not exceed the given
// maximum length.
func ValidateStringMaxLength(name, value string, maxLen int) error {
	if len(value) > maxLen {
		return fmt.Errorf("%s exceeds maximum length of %d characters", name, maxLen)
	}
	return nil
}
