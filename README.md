# Aris

**English** | [简体中文](README.zh-CN.md)

A **MaaS model aggregation gateway + usage-data curation platform + Agent Harness call-trace analysis platform** for personal large-model usage.

Aris unifies access to multiple MaaS platforms, models, and API protocols. Through model aliases, upstream endpoint management, protocol translation, authentication, rate limiting, auditing, and a visual admin console, clients only need to talk to a single gateway address and a model alias — they never need to know the vendor, the real model name, or the upstream API differences.

## Core Capabilities

- **Model aggregation & single entry point**: two-level Endpoint/Model configuration (user-level multi-tenancy — ordinary users manage their own endpoints and models, admins can filter by user and create on their behalf); model aliases hide upstream differences, and one alias can be bound to multiple endpoints. Provides OpenAI Chat Completions, OpenAI Responses, and Anthropic Messages compatible entry points, plus cross-protocol translation between OpenAI and Anthropic (including SSE streaming events).
- **Data curation & governance**: model interactions are aggregated into Sessions; Messages/Tools are stored in a protocol-neutral structure and deduplicated by checksum content addressing. Supports rating, share links, session filtering, and ShareGPT JSONL streaming export — curating data assets for private-model SFT.
- **Audit & runtime monitoring**: every call records four categories of token usage, time to first token, streaming duration, upstream status code, and Trace ID. Provides trend / success-rate / throughput / usage statistics and cron auditing, plus cross-pod runtime monitoring of QPS, P95, SSE connections, goroutines/heap/CPU.
- **Agent Harness Trace analysis**: the separately compiled `aris` CLI collects Codex hook events and rollout records and batch-uploads them from a local spool; the server rebuilds events into a readable TraceConversation to observe the Agent's model requests, reasoning, and tool-call chains.
- **Client config export**: generate OpenCode, Claude Code, Codex, and Pi connection configs or install scripts from the admin console in one click — idempotent patching, `.bak` backups, atomic replacement, credential files at `0600`.
- **Security & governance**: GitHub/Google OAuth2 login + JWT admin auth, Proxy API Key call auth, four-tier `pending`/`demo`/`user`/`admin` permissions with user approval (demo account: module whitelist + session-whitelist / full-masking dual views + one-click login + access auditing), Aho-Corasick trigger-word blocking, and Redis token-bucket request rate limiting plus token-usage limiting.

## System Architecture

The backend is organized by DDD layering: interface layer (router / ten-level middleware chain / handler / dto) → application layer (17 use-case modules with `command`/`query`/`port`) → domain layer (aggregates, value objects, repository interfaces); the infrastructure layer implements repository interfaces in reverse to achieve dependency inversion, with Cron scheduled governance and `bootstrap`/Fx wiring as cross-cutting concerns. There are three classes of callers: the Web admin console (JWT), the `aris` Trace CLI, and agentic clients (`X-API-Key`).

![System overview · DDD layering](docs/diagrams/archify/aris-arch-01.gif)

Interactive version (node search / impact tracing / light-dark themes / guided tour, nodes link to source anchors): [docs/diagrams/archify/aris-arch-01.html](docs/diagrams/archify/aris-arch-01.html)

**LLM proxy path**: auth / rate limiting / trigger words → endpoint resolution (user-level isolation) → cross-protocol translation → upstream; the response is returned synchronously, while messages and audit records are persisted asynchronously through a Pond goroutine pool so the request is never blocked.

![LLM proxy path](docs/diagrams/archify/aris-arch-02.gif)

**Upstream fault tolerance (circuit breaking / isolation / degradation)**: the proxy forwarding path plugs a fault-tolerance guard `Guard` outside the retry loop — a three-state circuit breaker keyed on `BaseURL|APIKey` (sliding window of 60s × 6 buckets; opens at ≥10 requests with an error rate ≥50%; after 30s goes half-open allowing 1 probe to recover) combined with per-key semaphore isolation (concurrency ≤32, 1s wait timeout). When the breaker is open or the semaphore is saturated it fast-fails with 503 + `Retry-After` + an error body formatted for the ingress protocol. 429 remains retryable and does not count toward tripping the breaker (rate limiting is not a fault), and already-established SSE connections are unaffected; breaker state and rejection counts are exported as Prometheus metrics (label = endpoint key).

![Upstream fault tolerance: circuit breaking / isolation / degradation](docs/diagrams/archify/aris-arch-14.gif)

**Session data model & lifecycle**: LLM forwarding and Trace ingestion both write into the Session aggregate root; Messages/Tools are deduplicated by checksum content addressing, and audit, cache, export, and background consumption revolve around it.

![Session data model & lifecycle](docs/diagrams/archify/aris-arch-03.gif)

**Agent Harness Trace ingestion**: Codex / Claude Code hook and rollout events are collected by the `aris` CLI (fail-open) and batch-uploaded from a local spool; after authentication and dedup they are persisted, and the web side projects them into a readable call trace.

![Agent Harness Trace ingestion](docs/diagrams/archify/aris-arch-04.gif)

**Web admin console frontend architecture**: the browser loads the statically exported assets → a root-layout Provider chain (I18n → Theme → Auth) → route groups (14 admin pages / 3 public pages, admin pages guarded by `PermissionGuard`) → components and 6 shared hooks → `src/lib` as the sole backend gateway (Bearer injection, 401 single-flight refresh, error-code toasts, dataset SSE export).

![Web admin console frontend architecture](docs/diagrams/archify/aris-arch-05.gif)

**Frontend build & embedded delivery**: `next build` static export (`basePath: /web`) → `make web-build` copies and per-file `gzip -9` pre-compresses (10MB → 3.5MB) → `embed.FS` ships it with the server binary → `/web/*` explicitly resolves `Accept-Encoding` and serves pre-compressed content directly, while non-static paths fall back to `index.html`.

![Frontend build & embedded delivery](docs/diagrams/archify/aris-arch-06.gif)

**Runtime metrics collection**: in-process `HTTPCollector` (request-latency histogram + success/failure counters), `SSEGauge` (concurrent SSE connections), `TokenUsageCounter` (input/output token throughput) and a Go runtime collector (`go_goroutines` / `go_memstats_alloc_bytes` / `process_cpu_seconds_total`) register into a Prometheus Registry; one `MetricsFlusher` per pod periodically (5s) runs `BuildSnapshot` and writes the snapshot to Redis (ZSET `metrics:runtime:data:{pod}` + an instance registry, with 24h retention cleanup). The runtime dashboard aggregation layer derives, from adjacent snapshots, an in-bucket mean for gauges / a positive counter delta ÷ bucket width for counters / a histogram bucket merge for P95, producing QPS / P95 / success rate / CPU% / tokens·s; the `/metrics` endpoint reads this pod's Registry directly via `promhttp.Handler`.

![Runtime metrics collection](docs/diagrams/archify/aris-arch-09.gif)

**Graceful shutdown**: `SIGINT` / `SIGTERM` triggers `fx.Lifecycle.OnStop` to run in reverse order (registration order Redis → DB → TriggerService → Logger → HTTP → Flusher → Inflight → Pool → Cron, so OnStop runs the reverse): stop cron (`CronManager.StopAll`, 3min) → stop the `pond` pool (`StopWithContext`, 3min) → drain `inflight` and wait for in-flight work (5min) → shut down HTTP (`ShutdownWithContext` 30s, letting SSE long connections drain, stopping the Flusher) → flush logs + stop TriggerService pub/sub → close DB → close Redis. Kubernetes pairs this with `preStop sleep 10` + `terminationGracePeriodSeconds 660` for lossless rollouts.

![Graceful shutdown sequence](docs/diagrams/archify/aris-arch-10.gif)

**Trigger-word hot reload**: after the admin console writes to the DB via `/api/web/v1/trigger`, `NotifyChanged` sends an immediate signal via Redis `Publish(trigger:changed)` plus a version broadcast `INCR(trigger:version)`; each pod's `TriggerService` combines pub/sub subscription, version polling (2s), and a low-frequency fallback (5min) to trigger a full `Rebuild` (rebuilding the in-memory Aho-Corasick matcher from `ListAll`). LLM requests run `Check`, and on a hit are blocked (deny) or masked (omit) by action; hit counts are batch-written back to the DB via the `TriggerHitSync` cron (every 5min `PopAll` → `BatchIncrementHitCount`).

![Trigger-word hot reload](docs/diagrams/archify/aris-arch-11.gif)

**Scheduled task trigger & execution**: the admin console updates spec / enabled via `/api/web/v1/cron` (`update_cron_job` validates the spec, and core tasks cannot be disabled) or manually triggers `CronManager.Trigger`; `CronManager` hot-reloads (`Restart` / `Enable` / `Disable`) and broadcasts across pods via the `cron:reload` Redis pub/sub (other pods sync in `handleMessage`). A Redis distributed lock (`cron:lock:{name}`, TTL 5min + ticker renewal) guarantees single-instance execution; `wrapCronFunc` injects traceID / an enabled check / panic recovery. The 4 tasks (terminal-cleanup scans the last 24h of terminal sessions hourly / purge on Sundays at 04:00 / think daily at 00:00 / hit-sync every 5min) each record a `CronCallAudit` (status + duration + trigger source). Session prefix dedup no longer runs via cron — after the insert transaction commits, a separate short transaction with a `FOR UPDATE` row lock merges same-prefix sessions in real time.

![Scheduled task trigger & execution](docs/diagrams/archify/aris-arch-12.gif)

**Demo account architecture**: four-tier permissions `pending` < `demo` < `user` < `admin`, with a global singleton demo account offering a one-click read-only experience. The login page's "Continue as Demo" hits an unauthenticated, IP-rate-limited endpoint (token bucket 5s/8) calling `POST /api/web/v1/demo/login`; the DemoLogin command checks `login_enabled`, locates the demo user by permission, and issues a JWT. Login and module-access events (path/IP/UA) land in `demo_access_audits` for admin auditing. The frontend drives a three-state PermissionGuard and grays/locks Nav & delete buttons via `isDemo()` + `demoModules`. Demo read-only requests first pass an access line: the permission middleware whitelists by module (fail-closed), and `TokenBucketRateLimiterMiddleware`, via `WithPermissionFilter(demo)`, enables the `demoAccess` IP token bucket only for the demo permission (5s/30, 8 endpoint groups share one bucket globally, over-limit 429 + `Retry-After`; non-demo users pass at zero cost). Data is presented in two views: **session whitelist** — an admin ticks sessions in a dedicated `/demo` tab to batch add/remove `demo_sessions` (`session_id` unique index); the demo list uses `ListSessionsByIDs` and details are checked for membership via `IsAllowed`, with anything not whitelisted returning "not found" (to prevent enumeration), and whitelist contents are stored in plaintext (admin selection is the authorization). **Full masking** — audit/models/endpoints show full data but mask key fields (identity fields via `MaskIdentity` → `***`, connection fields via `MaskSecret` → keep first 4 / last 4), while statistics and aliases are deliberately kept. All write endpoints and LLM forwarding with existing API keys are naturally rejected because demo permission is below user — zero extra changes.

![Demo account architecture](docs/diagrams/archify/aris-arch-13.gif)

## Tech Stack

| Layer | Technology |
| --- | --- |
| Backend | Go 1.25 · Fiber v3 · Huma v2 · Cobra · Viper · Uber Fx |
| Data | PostgreSQL + GORM · Redis (cache / token bucket / Cron locks) · SQLite (unit tests) · Pond goroutine pool |
| Observability | Zap + Lumberjack · Tencent Cloud CLS · Prometheus · fgprof · `X-Trace-Id` request tracing |
| Frontend | Next.js 16 (App Router) · React 19 · TypeScript · Tailwind CSS 4 · Base UI/shadcn · Recharts · Mermaid |
| Delivery | Frontend static assets gzip pre-compressed + `embed.FS` embedding · Docker multi-stage build (Distroless nonroot) · Docker Compose · Kubernetes · GitHub Actions |

## Quick Start

### Requirements

- Go 1.25+, Node.js 22+ (only needed for frontend development/building)
- PostgreSQL, Redis (can be started via Docker Compose)

### Start dependencies & configure

```bash
git clone https://github.com/hcd233/aris-proxy-api.git
cd aris-proxy-api
go mod download

# Prepare config (adjust DB, Redis, JWT Secret, OAuth2, etc. as needed)
cp env/api.env.template env/api.env
cp env/postgresql.env.template env/postgresql.env
cp env/redis.env.template env/redis.env

# Start PostgreSQL and Redis
docker volume create postgresql-data && docker volume create redis-data
docker compose -f docker/docker-compose-full.yml up -d
```

### Migrate & start the service

```bash
# Database migration
go run ./cmd/server database migrate

# Build and embed the frontend (required for production; skippable for backend-only dev)
make web-build

# Start the service
go run ./cmd/server server start --host localhost --port 8080
```

After startup:

- Admin console `http://localhost:8080/`
- Health `/health` · Readiness `/ready` · SSE health `/ssehealth`
- Dev-only API docs `/docs`, OpenAPI `/openapi.json` (auto-disabled in production)

First use: log in to the admin console via OAuth2 → admin approves the user → create a Proxy API Key → configure an Endpoint and a Model → then call the OpenAI/Anthropic compatible APIs.

### Frontend standalone development

```bash
cd web && npm ci && npm run dev   # http://localhost:3000
```

## Common Commands

| Command | Description |
| --- | --- |
| `make build` | Production build of server + four-platform Trace client |
| `make build-server` / `make build-client[-all]` | Build the server / the `aris` client respectively |
| `make web-build` | Build the frontend into `internal/web/dist` and gzip pre-compress |
| `make test` / `make test-cover` | Full test suite / coverage report |
| `make lint` | Custom convention lint (`lint conv`) + static checks (vet/staticcheck/golangci-lint) |
| `make web-lint` / `make web-format` | Frontend ESLint / Prettier |
| `make fgprof` | Pull a remote fgprof profile and open the flame graph |

Server CLI: `server start`, `database migrate`, `lint conv/static` (`cmd/server`); Trace client CLI: `aris init`, `aris model export`, `aris trace ingest`, `aris trace install`, `aris status`, `aris update` (`cmd/client`).

## API Overview

All APIs are registered with Huma and generate OpenAPI; in development they can be explored interactively at `/docs`.

| Prefix | Auth | Description |
| --- | --- | --- |
| `/api/openai/v1` · `/api/anthropic/v1` | `X-API-Key` | LLM-compatible entry points: chat/completions, responses, messages, count_tokens, models |
| `/api/cli/v1/model/list` · `/trace/event` · `/aris/client/check` | `X-API-Key` | `aris` client model distribution, Trace event upload, and client key check |
| `/api/web/v1/oauth2` · `/token` | Public (rate-limited) | OAuth2 login/callback, token refresh |
| `/api/web/v1/user` | JWT | Profile; admins can list, approve/demote, and delete users |
| `/api/web/v1/apikey` · `/session` · `/dataset` · `/trace` | JWT + owner isolation | API keys, sessions/sharing, dataset export, Trace queries |
| `/api/web/v1/endpoint` · `/model` · `/upstream` | JWT (user-level multi-tenancy) | Endpoints, models (user-isolated; admins can filter by user and create on their behalf), aggregated upstream view |
| `/api/web/v1/trigger` · `/cron` · `/audit/cron` · `/metrics` | JWT + admin | Trigger words, Cron management/triggering, Cron audit, runtime metrics |
| `/api/web/v1/audit` | JWT | Model-call audit & statistics (admins see global) |
| `/health` `/ready` `/ssehealth` `/metrics` `/install.sh` | Public | Probes, Prometheus metrics, Trace client install script |

## Deployment

- **Single-host Compose**: `docker compose -f docker/docker-compose-single.yml up -d` (requires `env/api.env`, `IMAGE_TAG`, and the external network `1panel-network`; the API starts only after the migration container succeeds; host `7070` → container `8080`). Use `docker-compose-dev-single.yml` for development (port `7060`).
- **Kubernetes**: Deployment + Service + ConfigMap/Secret, rolling updates; `preStop` + draining + `terminationGracePeriodSeconds` support lossless shutdown of long-lived SSE connections.
- **CI**: GitHub Actions builds GHCR images and publishes the `aris` client Release (tar.gz + sha256); the server's `/install.sh` provides a self-contained install script.

![Deployment & release topology](docs/diagrams/archify/aris-arch-07.gif)

## Project Structure

```text
cmd/server        Server entry (start / database migrate / lint)
cmd/client        aris Trace client entry
internal/
  router/         Route registration (Huma Operation + middleware binding)
  handler/        HTTP adaptation layer
  application/    Use-case orchestration (identity/session/llmproxy/trace/...)
  domain/         Domain models and rules
  infrastructure/ GORM Repository, Redis, JWT, LLM Transport, goroutine pool
  cron/           Scheduled tasks (terminal cleanup, soft-deleted purge, Think extraction, trigger-hit sync)
  bootstrap/      Fx dependency injection & lifecycle
  web/dist        Frontend build output (embed.FS)
web/              Next.js admin console source
  src/app/        App Router route groups ((dashboard) 14 pages · login / callback / share)
  src/components/ Base & business components (ui / charts / chat / session-detail / trace-detail)
  src/lib/        api-client · auth-context · i18n · theme · types (backend DTO mirrors)
docker/           Dockerfile and Compose configs
env/              Environment variable templates
docs/             Design docs and architecture diagrams
test/             Unit tests and E2E tests
```

## Documentation

- Architecture diagram set: [docs/diagrams/](docs/diagrams/)
- Design & troubleshooting docs: [docs/](docs/)
- Development standards (for contributors and AI agents): [AGENTS.md](AGENTS.md) and [docs/agents/](docs/agents/)

## License

[Apache License 2.0](LICENSE)
