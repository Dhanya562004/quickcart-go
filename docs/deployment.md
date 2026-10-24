# QuickCart — Production Deployment Guide

This guide details the deployment architecture, configuration prerequisites, security checklist, and step-by-step procedures for deploying QuickCart locally and to production cloud environments.

---

## 1. Deployment Status

> [!NOTE]
> **Current Status**: **Local Multi-Instance Production Topology Validated**.
> Cloud deployment to paid hosted platforms is currently **pending explicit user authorization**. No external cloud credentials or paid resources were provisioned without explicit approval. No simulated or fictional public URLs are provided.

---

## 2. Option A: Local Multi-Instance Cluster (Docker Compose)

The primary local deployment runs the full production topology: **3 Go API instances behind Nginx, shared PostgreSQL 16, Redis 7, Prometheus, and Grafana**.

### Architecture
* **Nginx Load Balancer**: Listens on host `http://localhost:8080`.
* **QuickCart API (Replicas 1, 2, 3)**: Stateless Go microservices on internal bridge network `quickcart-net`.
* **PostgreSQL 16**: Relational store on port `5433` (internal `5432`).
* **Redis 7**: Cache & Pub/Sub on port `6379`.
* **Prometheus**: Metrics aggregator on `http://localhost:9090`.
* **Grafana**: Pre-provisioned dashboards on `http://localhost:3000` (user: `admin`, pass: `admin`).

### Step-by-Step Launch Commands

```bash
# 1. Clone repository and navigate to workspace
cd "QuickCart — Concurrent Quick-Commerce Engine"

# 2. Build application containers and start cluster in detached mode
docker compose -f deployments/docker-compose.yml up --build -d

# 3. Verify container health status
docker compose -f deployments/docker-compose.yml ps

# 4. Check API cluster liveness and readiness via Nginx
curl -i http://localhost:8080/healthz
curl -i http://localhost:8080/readyz

# 5. Access Interactive Swagger Documentation
open http://localhost:8080/docs

# 6. Stream logs from all 3 API instances and Nginx
docker compose -f deployments/docker-compose.yml logs -f quickcart-nginx quickcart-api-1 quickcart-api-2 quickcart-api-3

# 7. Gracefully stop and tear down the cluster
docker compose -f deployments/docker-compose.yml down
```

---

## 3. Option B: Cloud Production Deployment (Fly.io / AWS ECS)

### Cloud Target Topology
* **Managed Database**: PostgreSQL 16 on AWS RDS, Supabase, or Neon.
* **Managed Cache**: Redis 7 on AWS ElastiCache, Upstash, or Redis Cloud.
* **Compute**: Multi-region container instances behind an Application Load Balancer / Anycast routing.

---

### Step-by-Step Deployment on Fly.io (Recommended for Low Latency)

Fly.io provides native Anycast global routing and automatic load balancing across container replicas:

#### Step 1: Install Flyctl & Authenticate
```bash
# Install flyctl CLI
curl -L https://fly.io/install.sh | sh

# Authenticate
fly auth login
```

#### Step 2: Provision Managed PostgreSQL & Redis
```bash
# Create PostgreSQL 16 app
fly postgres create --name quickcart-pg --region bom --initial-cluster-size 2 --vm-size shared-cpu-1x

# Create Redis 7 instance via Upstash
fly redis create --name quickcart-redis --region bom
```

#### Step 3: Configure Production Secrets
```bash
# Set secrets securely (never committed to git)
fly secrets set \
  DATABASE_URL="postgres://quickcart:<secret>@quickcart-pg.internal:5432/quickcart?sslmode=disable" \
  REDIS_URL="redis://default:<secret>@quickcart-redis.internal:6379" \
  JWT_SECRET="<generate-64-char-random-production-secret>" \
  WEBHOOK_SECRET="<generate-32-char-random-production-secret>" \
  METRICS_TOKEN="<generate-secure-metrics-scrape-token>" \
  APP_ENV="production" \
  LOG_LEVEL="info"
```

#### Step 4: Configure `fly.toml`
Create `fly.toml` in the project root:

```toml
app = "quickcart-api"
primary_region = "bom"

[build]
  dockerfile = "deployments/Dockerfile"

[env]
  SERVER_PORT = "8080"
  SERVER_HOST = "0.0.0.0"
  DB_MAX_CONNS = "25"
  DB_MIN_CONNS = "5"
  RATE_LIMIT_REQUESTS = "100"
  WORKER_POOL_SIZE = "4"

[http_service]
  internal_port = 8080
  force_https = true
  auto_stop_machines = false
  auto_start_machines = true
  min_machines_running = 3

  [http_service.concurrency]
    type = "requests"
    soft_limit = 250
    hard_limit = 500

[[http_service.checks]]
  grace_period = "10s"
  interval = "15s"
  method = "GET"
  path = "/readyz"
  timeout = "2s"
```

#### Step 5: Deploy and Scale Replicas
```bash
# Deploy release
fly deploy

# Scale to 3 active replicas across availability zones
fly scale count 3

# Verify cluster status and health checks
fly status
fly logs
```

---

## 4. Production Environment Variables Checklist

| Variable | Required | Description | Production Example |
| :--- | :---: | :--- | :--- |
| `SERVER_PORT` | Yes | HTTP listening port | `8080` |
| `SERVER_HOST` | Yes | Host binding address | `0.0.0.0` |
| `APP_ENV` | Yes | Application environment mode | `production` |
| `LOG_LEVEL` | Yes | Structured logger level (`debug`, `info`, `warn`, `error`) | `info` |
| `DATABASE_URL` | Yes | PostgreSQL connection string | `postgres://user:pass@db-host:5432/quickcart?sslmode=require` |
| `DB_MAX_CONNS` | Yes | Connection pool max limits | `25` |
| `DB_MIN_CONNS` | Yes | Pre-warmed idle connections | `5` |
| `REDIS_URL` | Yes | Redis connection URI | `redis://user:pass@redis-host:6379/0` |
| `JWT_SECRET` | Yes | Cryptographic HMAC-SHA256 signature key (min 32 bytes) | *Secure Vault Secret* |
| `JWT_ACCESS_TTL` | No | Access token expiration duration | `15m` |
| `JWT_REFRESH_TTL`| No | Refresh token expiration duration | `168h` (7 days) |
| `METRICS_TOKEN` | Yes | Bearer token protecting `GET /metrics` | *Secure Scrape Token* |
| `RATE_LIMIT_REQUESTS`| No | Sliding-window request cap per window | `100` |
| `RATE_LIMIT_WINDOW`  | No | Sliding-window duration | `1m` |

---

## 5. Zero-Downtime Migration & Rollout Strategy

1. **Schema Migrations**: Schema migrations (`000001_init_schema` and `000002_auth_refresh_tokens`) are backward-compatible. Run migrations prior to deploying new container versions:
   ```bash
   migrate -path migrations/ -database "$DATABASE_URL" up
   ```
2. **Rolling Deployments**:
   * Kubernetes / ECS / Fly.io updates one pod/task at a time (`maxSurge: 1, maxUnavailable: 0`).
   * The new pod passes `/readyz` dependency checks (DB ping & Redis ping) before receiving traffic.
3. **Graceful Shutdown**:
   * Upon receiving `SIGTERM`, QuickCart halts HTTP listeners, waits up to `SHUTDOWN_TIMEOUT` (10s) to drain active requests, terminates the reservation expiry sweep worker, and gracefully drains background job queues without dropping in-flight orders.
