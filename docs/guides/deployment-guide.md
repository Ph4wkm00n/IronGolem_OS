# IronGolem OS Deployment Guide

## Prerequisites

| Requirement | Minimum | Recommended |
|-------------|---------|-------------|
| CPU | 2 cores | 4 cores |
| RAM | 2 GB | 4 GB |
| Disk | 5 GB | 20 GB |
| Rust | 1.75+ | latest stable |
| Go | 1.22+ | latest stable |
| Node.js | 20+ | 20 LTS |
| pnpm | 9+ | latest |
| Docker | 24+ (optional) | latest |

## Solo Mode (SQLite, single machine)

Solo mode is the simplest deployment. All data lives in a single SQLite file.

### Option A: Docker Compose (recommended)

```bash
git clone https://github.com/Ph4wkm00n/IronGolem_OS.git
cd IronGolem_OS
docker compose -f infra/docker/docker-compose.yml up -d
```

Services start on their default ports. The web UI is at `http://localhost:3000`.

To stop: `docker compose -f infra/docker/docker-compose.yml down`

Data persists in the `irongolem-data` Docker volume.

### Option B: Direct install

```bash
# 1. Build all components
make build

# 2. Start Go services and web app
make dev
```

This runs the gateway (port 8080) and web frontend (port 3000).

To run all services individually, start each from its `cmd/` directory:

```bash
cd services && go run ./gateway/cmd &
cd services && go run ./scheduler/cmd &
cd services && go run ./health/cmd &
cd services && go run ./defense/cmd &
```

### Option C: Tauri desktop app

```bash
pnpm --filter @irongolem/desktop tauri dev    # development
pnpm --filter @irongolem/desktop tauri build   # production binary
```

The Tauri app bundles the web UI and connects to locally running Go services.

## Household Mode

Household mode uses the same SQLite backend as Solo but adds role boundaries for shared family use. Set the deployment mode environment variable:

```bash
export IRONGOLEM_MODE=household
```

In Docker Compose, edit the `IRONGOLEM_MODE` value in `infra/docker/docker-compose.yml`.

## Team Mode (PostgreSQL)

### Database setup

```bash
# Create the database
createdb irongolem

# Set connection string
export IRONGOLEM_DB_URL=postgres://user:pass@localhost:5432/irongolem?sslmode=require
export IRONGOLEM_MODE=team
```

### Service configuration

Each service reads the mode and database URL from environment variables. In Team mode, all services must point to the same PostgreSQL instance.

### Multi-instance deployment

For high availability, run multiple instances of each service behind a load balancer. Services are stateless; shared state lives in PostgreSQL.

```yaml
# Example: scale gateway to 3 instances
docker compose -f infra/docker/docker-compose.yml up -d --scale gateway=3
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `IRONGOLEM_MODE` | `solo` | Deployment mode: `solo`, `household`, `team` |
| `IRONGOLEM_DB_PATH` | `/data/irongolem.db` | SQLite file path (solo/household) |
| `IRONGOLEM_DB_URL` | - | PostgreSQL connection string (team) |
| `IRONGOLEM_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `GATEWAY_ADDR` | `:8080` | Gateway listen address |
| `IRONGOLEM_OTLP_ENDPOINT` | - | OpenTelemetry collector endpoint |
| `IRONGOLEM_TLS_CERT` | - | Path to TLS certificate |
| `IRONGOLEM_TLS_KEY` | - | Path to TLS private key |
| `DEPLOYMENT_MODE` | `solo` | Middleware deployment mode flag |

## TLS / HTTPS

For production, terminate TLS at a reverse proxy (nginx, Caddy) or provide cert paths:

```bash
export IRONGOLEM_TLS_CERT=/etc/irongolem/tls.crt
export IRONGOLEM_TLS_KEY=/etc/irongolem/tls.key
```

The included `infra/docker/nginx.conf` can be used as a starting point for reverse proxy configuration.

## Backup and Restore

### Solo / Household (SQLite)

```bash
# Backup
cp /data/irongolem.db /backups/irongolem-$(date +%Y%m%d).db

# Restore
cp /backups/irongolem-20260401.db /data/irongolem.db
```

### Team (PostgreSQL)

```bash
# Backup
pg_dump irongolem > /backups/irongolem-$(date +%Y%m%d).sql

# Restore
psql irongolem < /backups/irongolem-20260401.sql
```

## Monitoring

### Health endpoints

Every service exposes `GET /healthz` returning `{"status": "ok"}`.

| Service | Health endpoint |
|---------|----------------|
| Gateway | `http://localhost:8080/healthz` |
| Scheduler | `http://localhost:8081/healthz` |
| Health | `http://localhost:8082/healthz` |
| Defense | `http://localhost:8083/healthz` |
| Research | `http://localhost:8085/healthz` |
| Optimizer | `http://localhost:8086/healthz` |
| Fleet | `http://localhost:8087/healthz` |

### OpenTelemetry

Set `IRONGOLEM_OTLP_ENDPOINT` to send traces and metrics to any OTLP-compatible collector (Jaeger, Grafana Tempo, etc.).

## Troubleshooting

| Problem | Solution |
|---------|----------|
| Port already in use | Change the service address env var (e.g. `GATEWAY_ADDR=:9080`) |
| SQLite locked | Ensure only one process writes to the database at a time |
| Docker volume missing data | Check that the `irongolem-data` volume was not pruned |
| Services cannot reach each other | In Docker, services use container names; outside Docker, use `localhost` |
| TLS handshake failures | Verify cert and key paths; check cert is not expired |
| High memory usage | Reduce log level to `warn`; check for runaway research topics |
