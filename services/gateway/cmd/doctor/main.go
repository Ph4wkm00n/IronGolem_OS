// Command doctor prints structured connector readiness for the active
// environment. v0.3 Step 1 of Plans/modular-puzzling-blum.md adopts the
// PlatformEntry registration pattern from NousResearch/hermes-agent and
// surfaces it as an operator-facing binary so setup-wizard UX + ops
// dashboards have a single source of truth for "is this connector
// reachable?"
//
// Usage:
//
//	irongolem-doctor              # default text output, one line per connector
//	irongolem-doctor --format=json # machine-readable for the wizard backend
//
// Exit code: 0 when every registered connector's CheckFn returns true;
// non-zero (1) when any CheckFn fails so CI / orchestrators can gate
// on it. JSON output always exits 0 — callers parse status per-entry.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"

	// Blank imports trigger each connector subpackage's init() so they
	// self-register in the connectors.registry. Add new connectors here
	// (and in services/gateway/cmd/main.go) so doctor mirrors what the
	// gateway actually loads.
	_ "github.com/Ph4wkm00n/IronGolem_OS/connectors/discord"
	_ "github.com/Ph4wkm00n/IronGolem_OS/connectors/email"
	_ "github.com/Ph4wkm00n/IronGolem_OS/connectors/signal"
	_ "github.com/Ph4wkm00n/IronGolem_OS/connectors/slack"
	_ "github.com/Ph4wkm00n/IronGolem_OS/connectors/telegram"
	_ "github.com/Ph4wkm00n/IronGolem_OS/connectors/webhook"
)

type doctorEntry struct {
	Type        string   `json:"type"`
	Label       string   `json:"label"`
	OK          bool     `json:"ok"`
	Source      string   `json:"source"`
	RequiredEnv []string `json:"required_env,omitempty"`
	MissingEnv  []string `json:"missing_env,omitempty"`
	InstallHint string   `json:"install_hint,omitempty"`
}

func main() {
	format := flag.String("format", "text", "output format: text|json")
	strict := flag.Bool("strict", true, "exit non-zero when any connector check fails (text mode only)")
	flag.Parse()

	entries := collect()

	switch strings.ToLower(*format) {
	case "json":
		// Always exit 0 in JSON mode — callers branch on per-entry .ok.
		if err := json.NewEncoder(os.Stdout).Encode(entries); err != nil {
			fmt.Fprintln(os.Stderr, "doctor: json encode failed:", err)
			os.Exit(2)
		}
		return
	case "text", "":
		anyFailed := printText(entries)
		if anyFailed && *strict {
			os.Exit(1)
		}
		return
	default:
		fmt.Fprintf(os.Stderr, "doctor: unknown --format %q (want text|json)\n", *format)
		os.Exit(2)
	}
}

func collect() []doctorEntry {
	regs := connectors.List()
	out := make([]doctorEntry, 0, len(regs))
	for _, r := range regs {
		entry := doctorEntry{
			Type:        string(r.Type),
			Label:       r.Label,
			Source:      string(r.Source),
			RequiredEnv: append([]string(nil), r.RequiredEnv...),
			InstallHint: r.InstallHint,
			OK:          r.CheckFn(),
		}
		if !entry.OK {
			entry.MissingEnv = missingEnv(r.RequiredEnv)
		}
		out = append(out, entry)
	}
	return out
}

func missingEnv(names []string) []string {
	var miss []string
	for _, n := range names {
		if strings.TrimSpace(os.Getenv(n)) == "" {
			miss = append(miss, n)
		}
	}
	return miss
}

func printText(entries []doctorEntry) bool {
	anyFailed := false
	fmt.Println("IronGolem Connector Doctor")
	fmt.Println("===========================")
	for _, e := range entries {
		status := "OK"
		if !e.OK {
			status = "MISSING"
			anyFailed = true
		}
		fmt.Printf("  %-12s [%-7s] %s\n", e.Type, status, e.Label)
		if !e.OK {
			if len(e.MissingEnv) > 0 {
				fmt.Printf("                missing env: %s\n", strings.Join(e.MissingEnv, ", "))
			}
			if e.InstallHint != "" {
				fmt.Printf("                hint: %s\n", e.InstallHint)
			}
		}
	}
	fmt.Println()
	if anyFailed {
		fmt.Println("One or more connectors are not configured. See hints above.")
	} else {
		fmt.Println("All registered connectors report ready.")
	}
	return anyFailed
}
