# Connector Development Guide

Connectors are how IronGolem OS communicates with external services and
channels. This guide explains how to build a new connector.

## Connector Architecture

Every connector lives in `connectors/<name>/` and must implement:

1. **Event normalization** - Convert service-specific events into IronGolem OS event format
2. **Token lifecycle** - Manage authentication credentials
3. **Health signals** - Emit heartbeat data for the Health Center
4. **Policy boundaries** - Enforce connector-specific restrictions
5. **Registration metadata** - Self-register with the package-level registry so
   `irongolem-doctor` and the setup wizard can report readiness without
   per-connector special cases (v0.3 Step 1).

## Built-in Connectors (v1.2.0)

| Connector | Send | Receive | Auth |
|-----------|------|---------|------|
| `telegram` | ✓ | ✓ (long-poll) | Bot token |
| `email`    | ✓ | ✓ (IMAP)     | Username + password / app password |
| `webhook`  | ✓ | ✓ (HTTP)     | Configurable (bearer, basic, HMAC) |
| `slack`    | ✓ | stubbed (v0.4 Events API webhook) | Bot token (`xoxb-…`) |
| `discord`  | ✓ | stubbed (v0.4 Gateway WebSocket) | Bot token |
| `signal`   | ✓ | stubbed (v0.4 `signal-cli daemon`) | `signal-cli` bridge + verified account |

## Connector Categories

| Category | Examples |
|----------|---------|
| Messaging | Telegram, Slack, Discord, Signal, WhatsApp (v0.4+), Feishu/Lark (v0.4+) |
| Email | IMAP/SMTP email |
| Calendar | Google Calendar (v0.4+), CalDAV (v0.4+) |
| Filesystem | Local file access (v0.4+) |
| Browser | Web automation (v0.4+) |
| Docs | Knowledge source ingestion (v0.4+) |
| Generic | Webhooks, REST APIs |

## Registration Pattern (v0.3 Step 1)

Each connector subpackage exports a `Registration()` function and calls
`connectors.MustRegister(Registration())` from an `init()` so the
package-level registry sees it without explicit wiring in main.go.

```go
// connectors/<name>/metadata.go
package mychannel

import (
    "fmt"
    "os"
    "strings"

    connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
)

const envToken = "IRONGOLEM_MYCHANNEL_TOKEN"

func Registration() connectors.Registration {
    return connectors.Registration{
        Type:        connectors.ConnectorType("mychannel"),
        Label:       "My Channel",
        CheckFn:     func() bool { return strings.TrimSpace(os.Getenv(envToken)) != "" },
        RequiredEnv: []string{envToken},
        InstallHint: "Set IRONGOLEM_MYCHANNEL_TOKEN from the My Channel developer portal.",
        ValidateConfig: func(cfg map[string]string) error {
            if strings.TrimSpace(cfg["token"]) == "" {
                return fmt.Errorf("token is required")
            }
            return nil
        },
        Source: connectors.SourceBuiltin,
    }
}

func init() { connectors.MustRegister(Registration()) }
```

The fields:

| Field | Purpose |
|-------|---------|
| `Type` | Canonical wire identifier — also the registry key. |
| `Label` | Human-readable name shown by `doctor` and the setup wizard. |
| `CheckFn` | Cheap synchronous predicate. `true` when boot-level env is set. **Must not do network IO.** |
| `RequiredEnv` | Env vars surfaced by `doctor` when `CheckFn` fails. |
| `InstallHint` | One-line guidance shown alongside a failed `CheckFn`. |
| `ValidateConfig` | Optional second-stage check against an instantiated config map. |
| `Source` | `SourceBuiltin` (in-repo) or `SourcePlugin` (v0.4+ external). |

Wire the connector into both `services/gateway/cmd/main.go` AND
`services/gateway/cmd/doctor/main.go` via blank import so the doctor
binary's view always matches what the gateway loads.

## Connector Interface

Every connector must implement these capabilities:

### Ingress (Receiving)
- Accept events from the external service
- Normalize events into the standard event schema
- Route normalized events to the gateway service

### Egress (Sending)
- Accept outbound actions from agents
- Translate actions to service-specific API calls
- Report delivery status back to the system

### Health
- Respond to heartbeat check-ins
- Report connection status (connected, degraded, disconnected)
- Report credential freshness
- Emit recovery signals after failures

### Policy
- Declare available capabilities
- Enforce per-connector allowlists
- Respect per-channel restrictions from the policy engine

## Connector Lifecycle

```
Initialize → Connect → Healthy → (Failure → Recover → Healthy)
                                         ↓
                                   Escalate to user
```

### Health States

| State | Meaning |
|-------|---------|
| Healthy | Connected and operating normally |
| Degraded | Partially functional (e.g., rate limited) |
| Recovering | Self-healing in progress |
| Disconnected | Cannot reach external service |
| Credential expired | Authentication needs refresh |

## Self-Healing Integration

Connectors integrate with the self-healing loop:
1. Missed heartbeat triggers retry
2. Retry failure triggers credential refresh
3. Credential refresh failure triggers config restore
4. Config restore failure escalates to user via Health Center

## Testing a Connector

| Test Type | What to Verify |
|-----------|---------------|
| Connection | Successful auth and connection |
| Event normalization | Events correctly translated to standard schema |
| Failure recovery | Self-healing responds to simulated failures |
| Policy enforcement | Connector respects capability boundaries |
| Rate limiting | Graceful handling of rate limits |

## Canonical Reference

See the connector module section in
[specs/02-features-modules-and-agent-loops-v2.md](../specs/02-features-modules-and-agent-loops-v2.md).
