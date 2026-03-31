# Connector Development Guide

Connectors are how IronGolem OS communicates with external services and
channels. This guide explains how to build a new connector.

## Connector Architecture

Every connector lives in `connectors/<name>/` and must implement:

1. **Event normalization** - Convert service-specific events into IronGolem OS event format
2. **Token lifecycle** - Manage authentication credentials
3. **Health signals** - Emit heartbeat data for the Health Center
4. **Policy boundaries** - Enforce connector-specific restrictions

## Connector Categories

| Category | Examples |
|----------|---------|
| Messaging | Telegram, Slack, Discord, WhatsApp, Feishu/Lark |
| Email | IMAP/SMTP email |
| Calendar | Google Calendar, CalDAV |
| Filesystem | Local file access |
| Browser | Web automation |
| Docs | Knowledge source ingestion |
| Generic | Webhooks, REST APIs |

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
