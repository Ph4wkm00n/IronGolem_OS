# Security Model

IronGolem OS implements a **five-layer permission architecture** that every
action must pass through. Security is foundational, not bolted on.

## Five Permission Layers

Every action in the system passes through these five layers in order:

### Layer 1: Gateway Identity and Authentication
- Who is making this request?
- What device and session are they using?
- Is the session valid and trusted?

### Layer 2: Global Tool Policy
- Which tools are allowed in this deployment?
- Are there global restrictions on certain tool categories?
- Deployment-wide safety boundaries

### Layer 3: Per-Agent Permissions
- What can this specific agent role do?
- Which tools is this agent authorized to use?
- What scope of data can this agent access?

### Layer 4: Per-Channel Restrictions
- What actions are allowed from this specific channel?
- Channel-specific rate limits and capability restrictions
- Connector-level policy boundaries

### Layer 5: Owner/Admin-Only Controls
- Is this a privileged action requiring admin approval?
- Owner-only escalation for high-risk operations
- Final gate before sensitive actions execute

## UI Representation

The security model is translated into plain-language UI elements:

- **Policy cards**: Explain each layer in human-readable terms
- **Risk badges**: Calm visual indicators (not alarming)
- **Safety blocks**: Per-recipe/squad summary of permissions
- **Admin shields**: Visual indicators for admin-only actions

## Security Hardening

### Prompt Injection Defense
- Detection corpus and scoring engine
- Suspicious content isolation
- Quarantine for flagged inputs
- Explainable incident summaries

### SSRF Protection
- Destination allowlists for all outbound requests
- URL validation before any fetch operation
- Connector-specific network boundaries

### Command Safety
- Shell deny patterns for dangerous commands
- Approval workflows for system-level operations
- Rate limiting on tool execution

### Data Protection
- Secret encryption at rest
- Per-tool and per-channel allowlists
- Tamper-evident audit trails
- Safe degradation (failures reduce capabilities, never silently break boundaries)

## Self-Defending Loop

The defense module continuously monitors for threats:

**Triggers**: Injection indicators, allowlist failures, unusual volume,
privilege escalation attempts, suspicious cross-tenant access

**Actions**: Deny/sandbox, quarantine, reduce privileges, require admin review,
produce explainable incident summary

## Canonical Reference

See the five-layer permission model in
[specs/02-features-modules-and-agent-loops-v2.md](../specs/02-features-modules-and-agent-loops-v2.md)
and security hardening requirements in
[specs/01-product-requirements-document-v2.md](../specs/01-product-requirements-document-v2.md).
