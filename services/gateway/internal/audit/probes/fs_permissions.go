package probes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/audit"
)

// FSPermissions ports openclaw's fs.state_dir/fs.config permission
// probes (2026-07 scan): the gateway SQLite database holds tokens,
// channel policies, commitments, and the full event timeline — if
// another local user can read or write it, every other security layer
// is decorative. Checks the DB file and its parent directory each tick:
//
//	world-writable  -> critical (any local user can tamper with history)
//	world-readable  -> critical for the file (token/PII exfiltration),
//	                   warning for the directory
//	group-writable  -> warning
//
// A missing file is info — fresh installs haven't created it yet.
type FSPermissions struct {
	dbPath string
}

// NewFSPermissions takes the resolved gateway DB path (the same value
// handed to persist.Open, ~ expansion included).
func NewFSPermissions(dbPath string) *FSPermissions {
	return &FSPermissions{dbPath: expandHome(dbPath)}
}

func (FSPermissions) ID() string { return "fs_permissions" }

func (p *FSPermissions) Run(_ context.Context) audit.Finding {
	if p.dbPath == "" {
		return audit.Finding{
			ProbeID:  "fs_permissions",
			Severity: audit.SeverityWarning,
			Reason:   "no database path configured for probe",
		}
	}

	fileInfo, fileErr := os.Stat(p.dbPath)
	dirInfo, dirErr := os.Stat(filepath.Dir(p.dbPath))

	if fileErr != nil && dirErr != nil {
		return audit.Finding{
			ProbeID:  "fs_permissions",
			Severity: audit.SeverityInfo,
			Reason:   "gateway database not created yet; nothing to check",
			Evidence: map[string]any{"path": p.dbPath},
		}
	}

	var problems []map[string]any
	worst := audit.SeverityInfo

	record := func(sev audit.Severity, target, mode, why string) {
		problems = append(problems, map[string]any{
			"target": target, "mode": mode, "issue": why,
		})
		if sev == audit.SeverityCritical || (sev == audit.SeverityWarning && worst == audit.SeverityInfo) {
			worst = sev
		}
	}

	if fileErr == nil {
		mode := fileInfo.Mode().Perm()
		modeStr := fmt.Sprintf("%04o", mode)
		switch {
		case mode&0o002 != 0:
			record(audit.SeverityCritical, p.dbPath, modeStr, "database file is world-writable")
		case mode&0o004 != 0:
			record(audit.SeverityCritical, p.dbPath, modeStr, "database file is world-readable (tokens and event history exposed)")
		case mode&0o020 != 0:
			record(audit.SeverityWarning, p.dbPath, modeStr, "database file is group-writable")
		}
	}
	if dirErr == nil {
		dir := filepath.Dir(p.dbPath)
		mode := dirInfo.Mode().Perm()
		modeStr := fmt.Sprintf("%04o", mode)
		switch {
		case mode&0o002 != 0:
			record(audit.SeverityCritical, dir, modeStr, "state directory is world-writable")
		case mode&0o004 != 0:
			record(audit.SeverityWarning, dir, modeStr, "state directory is world-readable")
		case mode&0o020 != 0:
			record(audit.SeverityWarning, dir, modeStr, "state directory is group-writable")
		}
	}

	if len(problems) == 0 {
		return audit.Finding{
			ProbeID:  "fs_permissions",
			Severity: audit.SeverityInfo,
			Reason:   "gateway database and state directory permissions look sane",
		}
	}
	return audit.Finding{
		ProbeID:  "fs_permissions",
		Severity: worst,
		Reason:   fmt.Sprintf("%d filesystem permission issue(s) on gateway state", len(problems)),
		Evidence: map[string]any{
			"problems":    problems,
			"remediation": "chmod 700 the state directory and 600 the database file",
		},
	}
}

// expandHome resolves a leading "~/" against $HOME, mirroring
// persist.Open's handling so the probe stats the same file the gateway
// actually opened.
func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}
