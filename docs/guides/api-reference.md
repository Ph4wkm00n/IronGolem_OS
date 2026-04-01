# IronGolem OS API Reference

All services accept and return JSON. Authentication is handled by the Gateway's identity layer.

## Gateway Service (port 8080)

### Messages

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/messages` | Send a message to the assistant |
| GET | `/api/v1/messages` | List recent messages |

**POST /api/v1/messages** -- Request body:
```json
{ "channel": "web", "content": "Check my inbox", "metadata": {} }
```
Response: `201 Created` with `{ "id": "...", "status": "accepted" }`

### Connectors

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/connectors` | List all connectors and their status |
| POST | `/api/v1/connectors` | Register a new connector |
| DELETE | `/api/v1/connectors/:id` | Remove a connector |

### Recipes

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/recipes` | List available recipes |
| POST | `/api/v1/recipes/:id/activate` | Activate a recipe |
| POST | `/api/v1/recipes/:id/deactivate` | Deactivate a recipe |
| GET | `/api/v1/recipes/:id/status` | Get recipe run status |

### Approvals

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/approvals` | List pending approvals |
| POST | `/api/v1/approvals/:id/approve` | Approve an action |
| POST | `/api/v1/approvals/:id/reject` | Reject an action |

### Events

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/events` | Query the event stream (supports `?since=` and `?limit=`) |

### Squads

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/squads` | List available squads |
| GET | `/api/v1/squads/:id` | Get squad details and member agents |
| POST | `/api/v1/squads/:id/delegate` | Delegate a task to a squad |

### Audit

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/audit` | Query audit trail (supports `?from=`, `?to=`, `?agent=`) |

### Health

| Method | Path | Description |
|--------|------|-------------|
| GET | `/healthz` | Service health check. Returns `{"status": "ok"}` |

---

## Scheduler Service (port 8081)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/jobs` | List scheduled jobs |
| POST | `/api/v1/jobs` | Create a scheduled job |
| DELETE | `/api/v1/jobs/:id` | Cancel a scheduled job |
| GET | `/healthz` | Service health check |

**POST /api/v1/jobs** -- Request body:
```json
{ "recipe_id": "...", "cron": "0 9 * * *", "enabled": true }
```
Response: `201 Created` with `{ "id": "...", "next_run": "..." }`

---

## Health Service (port 8082)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/health` | Aggregated system health overview |
| GET | `/api/v1/heartbeats` | List all component heartbeats |
| GET | `/api/v1/heartbeats/:id` | Get heartbeat history for a component |
| POST | `/api/v1/canaries` | Register a canary check |
| GET | `/api/v1/canaries` | List canary results |
| GET | `/healthz` | Service health check |

Heartbeat states: `healthy`, `quietly_recovering`, `needs_attention`, `paused`, `quarantined`.

---

## Defense Service (port 8083)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/check` | Check an action against security policies |
| GET | `/api/v1/blocked` | List blocked actions |
| POST | `/api/v1/quarantine/:id` | Quarantine a component |
| DELETE | `/api/v1/quarantine/:id` | Release a component from quarantine |
| GET | `/api/v1/incidents` | List security incidents |
| POST | `/api/v1/rollback` | Roll back to a previous checkpoint |
| POST | `/api/v1/allowlist` | Add an entry to the allowlist |
| GET | `/api/v1/commands` | List available defense commands |
| GET | `/healthz` | Service health check |

**POST /api/v1/check** -- Request body:
```json
{ "agent": "executor", "tool": "email.send", "target": "user@example.com" }
```
Response: `200 OK` with `{ "allowed": true }` or `{ "allowed": false, "reason": "..." }`

---

## Research Service (port 8085)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/topics` | List tracked research topics |
| POST | `/api/v1/topics` | Add a research topic |
| DELETE | `/api/v1/topics/:id` | Remove a topic |
| GET | `/api/v1/briefs` | List research briefs |
| GET | `/api/v1/briefs/:id` | Get a specific brief with sources |
| GET | `/api/v1/contradictions` | List detected contradictions |
| GET | `/healthz` | Service health check |

---

## Optimizer Service (port 8086)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/preferences` | Get learned user preferences |
| PUT | `/api/v1/preferences` | Update preferences manually |
| GET | `/api/v1/experiments` | List active A/B experiments |
| POST | `/api/v1/experiments` | Create an experiment |
| GET | `/api/v1/cache` | View prompt cache stats |
| DELETE | `/api/v1/cache` | Clear prompt cache |
| POST | `/api/v1/benchmark` | Run a prompt benchmark |
| GET | `/healthz` | Service health check |

---

## Fleet Service (port 8087)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/instances` | List all running instances (Team mode) |
| GET | `/api/v1/instances/:id` | Get instance details |
| GET | `/api/v1/overview` | Fleet-wide status overview |
| GET | `/healthz` | Service health check |

---

## Common Response Codes

| Code | Meaning |
|------|---------|
| 200 | Success |
| 201 | Created |
| 400 | Bad request (check request body) |
| 401 | Unauthorized (missing or invalid identity) |
| 403 | Forbidden (policy violation) |
| 404 | Resource not found |
| 409 | Conflict (e.g., duplicate activation) |
| 429 | Rate limited |
| 500 | Internal server error |
