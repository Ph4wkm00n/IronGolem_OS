# Phase 1: Trustworthy Local Core

**Timeline**: Month 2-5
**Theme**: Trust

## Goals

- Ship a solid solo-mode foundation
- Deliver visible trust features from day one

## Scope

- Rust runtime baseline (plan engine, checkpoint manager, verifier)
- Go control plane baseline (gateway, connector manager, scheduler)
- Desktop shell via Tauri
- Guided onboarding wizard
- Recipe gallery v1 with safety summaries
- Inbox, approvals, and action timeline
- Connector support: email, calendar, Telegram, filesystem
- Self-healing baseline with retries and heartbeat checks
- Security center baseline with injection filtering and blocked action reporting
- Provider abstraction and basic telemetry

## KPIs

- Time to first recipe completion
- Install success rate
- Approval speed
- Heartbeat visibility comprehension

## Exit Criteria

- Solo mode install is reliable
- First recipes usable by non-technical testers
- Heartbeat status visible in Health Center

## Canonical Reference

See [specs/03-roadmap-and-release-plan-v2.md](../specs/03-roadmap-and-release-plan-v2.md).
