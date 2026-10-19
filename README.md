# Aris

**English** | [简体中文](README.zh-CN.md)

A **MaaS model aggregation gateway + usage-data curation platform + Agent Harness call-trace analysis platform** for personal large-model usage.

Aris unifies access to multiple MaaS platforms, models, and API protocols. Through model aliases, upstream endpoint management, protocol translation, authentication, rate limiting, auditing, and a visual admin console, clients only need to talk to a single gateway address and a model alias — they never need to know the vendor, the real model name, or the upstream API differences.

## Core Capabilities

- **Model aggregation & single entry point**: two-level Endpoint/Model configuration (user-level multi-tenancy — ordinary users manage their own endpoints and models, admins can filter by user and create on their behalf); model aliases hide upstream differences, and one alias can be bound to multiple endpoints. Provides OpenAI Chat Completions, OpenAI Responses, and Anthropic Messages compatible entry points, plus cross-protocol translation between OpenAI and Anthropic (including SSE streaming events).
- **Data curation & governance**: model interactions are aggregated into Sessions; Messages/Tools are stored in a protocol-neutral structure and deduplicated by checksum content addressing. Supports rating, share links, session filtering, and ShareGPT JSONL streaming export — curating data assets for private-model SFT.
- **Audit & runtime monitoring**: every call records four categories of token usage, time to first token, streaming duration, upstream status code, and Trace ID. Provides trend / success-rate / throughput / usage statistics and cron auditing, plus cross-pod runtime monitoring of QPS, P95, SSE connections, goroutines/heap/CPU.
- **Agent Harness Trace analysis**: the separately compiled `aris` CLI collects Codex hook events and rollout records and batch-uploads them from a local spool; the server rebuilds events into a readable TraceConversation to observe the Agent's model requests, reasoning, and tool-call chains.
- **Subscription account OAuth (Claude Code / Codex)**: subscription accounts join as a dedicated endpoint type (`oauth_anthropic` / `oauth_openai`) via official OAuth flows. Access tokens refresh automatically (request-path with a 3-minute skew, plus a maintenance cron) and credentials are stored AES-256-GCM encrypted; the breaker key uses a stable per-endpoint identity, so token rotation never resets circuit state. Model aliases bind them like any endpoint — scheduling, fallback, auditing, pricing, and data curation apply unchanged — and subscription quota windows (Claude 5h/7d, Codex primary/secondary) are probed on a cron and shown in the console.
- **Client config export**: generate OpenCode, Claude Code, Codex, and Pi connection configs or install scripts from the admin console in one click — idempotent patching, `.bak` backups, atomic replacement, credential files at `0600`.
- **Security & governance**: GitHub/Google OAuth2 login + JWT admin auth, Proxy API Key call auth, four-tier `pending`/`demo`/`user`/`admin` permissions with user approval (demo account: module whitelist + session-whitelist / full-masking dual views + one-click login + access auditing), Aho-Corasick trigger-word blocking, and Redis token-bucket request rate limiting plus token-usage limiting.

## Comparison with Similar Projects

Four mainstream open-source MaaS gateways are compared below (snapshot **2026-10-19**): [new-api](https://github.com/QuantumNous/new-api) (Go/Gin + React; AGPLv3 + Section 7 terms), [LiteLLM](https://github.com/BerriAI/litellm) (Python + React; MIT + commercial enterprise licence), [OmniRoute](https://github.com/diegosouzapw/OmniRoute) (Node/TypeScript + SQLite; MIT) and [sub2api](https://github.com/Wei-Shaw/sub2api) (Go/Gin + Vue; LGPL-3.0). Only engineering capability is compared — product and go-to-market features are out of scope.

Three structural differences stand out:

- **Conversation bodies are a data-model choice, not a roadmap item.** Aris persists them, so checksum deduplication, scoring and ShareGPT/SFT export are structural capabilities. The other four have no conversation aggregate — LiteLLM can opt in, but its payload stays spend-log-shaped and its dataset export targets evaluation.
- **Quota enforcement and cost estimation are different designs.** new-api and sub2api deduct from quotas and balances (pre-deduct → settle, window soft caps); LiteLLM's budget family requires a database and **fails open without one**; Aris only estimates cost, with no budget gate.
- **Agent observability is three answers to one question.** Aris: client hooks + local spool + server-side rebuild, with the agent runtime untouched. LiteLLM: SDK harness + CLI wrappers + server-side agentic loop, with trace analysis in a separate Lens service. OmniRoute: capture at the traffic layer, visualising routing and compression rather than agent steps.

Legend: ✅ yes · ◐ partial · — no. Ratings follow each project's own README/official docs at the snapshot date; numbered cells are explained under [Reference notes](#reference-notes).

| Dimension | Aris | new-api | LiteLLM | OmniRoute | sub2api |
| --- | --- | --- | --- | --- | --- |
| **Ingress protocols** | ◐[1] | ✅[1] | ✅[1] | ✅ | ◐[1] |
| **Cross-protocol translation** | ✅ | ✅[2] | ✅ | ✅ | ◐[2] |
| **Upstream channel types (API key / subscription OAuth)** | ✅[3] | ◐[3] | ✅[3] | ✅[3] | ✅[3] |
| **Model alias & multi-endpoint binding** | ✅ | ✅ | ✅ | ✅ | ◐[4] |
| **Routing & scheduling** | ✅[5] | ✅[5] | ✅[5] | ✅[5] | ✅[5] |
| **Circuit breaking / isolation / degradation** | ✅[6] | ◐[6] | ◐[6] | ✅[6] | ✅[6] |
| **Multi-tenancy & permissions** | ✅ | ✅ | ✅ | —[7] | ✅ |
| **Authentication & login** | ✅[8] | ✅ | ✅[8] | ◐[8] | ✅ |
| **Rate limiting & concurrency** | ✅[9] | ✅ | ✅[9] | ✅ | ✅ |
| **Usage metering & quota enforcement** | ◐[10] | ✅[10] | ✅[10] | ◐[10] | ✅[10] |
| **Call audit & usage stats** | ✅[11] | ✅[11] | ✅ | ◐ | ◐[11] |
| **Conversation body persisted** | ✅[12] | —[12] | ◐[12] | — | — |
| **Conversation dataset export (ShareGPT/SFT)** | ✅[13] | — | ◐[13] | — | — |
| **Agent Harness trace ingestion** | ✅[14] | — | ✅[14] | ◐[14] | — |
| **Trace rebuild & visualisation** | ✅[15] | — | ✅[15] | ◐[15] | — |
| **Content safety & masking** | ✅[16] | ◐[16] | ✅[16] | ✅ | ◐ |
| **Observability & runtime monitoring** | ✅ | ◐[17] | ✅[17] | ◐[17] | ◐[17] |
| **Token compression** | ✅[18] | — | ◐[18] | ✅[18] | — |
| **MCP / A2A** | — | ◐[19] | ✅[19] | ✅[19] | ◐[19] |
| **Deployment & horizontal scaling** | ✅[20] | ✅[20] | ✅[20] | ◐[20] | ✅ |
| **Extensibility** | ◐[21] | ✅[21] | ✅[21] | ✅[21] | ✅[21] |

### Notes

**1 · Ingress protocols** — Aris ◐: text only (Chat/Responses/Messages + native Decision passthrough). new-api: four text protocols + Realtime/Responses WebSocket + media + task-plugin routes, ~60 channel types. LiteLLM: widest — batches, files, fine-tuning, evals, realtime, videos, ocr, search, memory, native Bedrock/Vertex, MCP + A2A. sub2api ◐: subscription-client shaped (Responses incl. `/backend-api/codex/responses`, Messages, Chat, `/v1/systemone`, Antigravity).

**2 · Cross-protocol translation** — Aris: 7 OpenAI ↔ Anthropic paths incl. SSE events. new-api: RelayKit converts the four text protocols, warning that protocol-specific tools/fields may not map exactly. LiteLLM: `/v1/messages` for all supported providers, but its Rust core covers only a few routes. sub2api ◐: provider-targeted conversions (Anthropic → xAI Responses, Codex Responses ← Chat, Antigravity).

**3 · Upstream channel types** — Aris ✅: `api_key` plus subscription OAuth endpoints (`oauth_anthropic` / `oauth_openai`, AES-256-GCM, refreshed with a 3-minute skew) sharing one forwarding path. new-api ◐: ~60 API-key types plus `ChannelTypeCodex = 57 "ChatGPT Subscription (Codex)"` (OAuth key JSON, `/v1/responses` only); the Claude Code OAuth channel (former type 53) was **reverted**. LiteLLM ✅: API keys plus ChatGPT-subscription device flow and per-user Copilot OAuth — the former has no cost-map price (spend stays $0); Anthropic Pro/Max undocumented. OmniRoute ✅: four-tier fallback (Subscription → API Key → Cheap → Free) with PKCE logins. sub2api ✅: pooled OAuth subscription accounts (Claude/Codex/Grok/Antigravity) with sticky sessions.

**4 · Model alias & multi-endpoint binding** — sub2api ◐: Composite Groups resolve a model to concrete providers; the others have first-class alias → N-endpoint binding with priority and weight.

**5 · Routing & scheduling** — Aris: priority tiers + in-tier A-Res weighted shuffle + session affinity (TTL 5 min, fail-open). new-api: weighted random, draggable priority, channel affinity, `auto` cross-group retry. LiteLLM: 5 strategies (default `simple-shuffle`) + four specialised routers + Auto Router. OmniRoute: 19 combo strategies + 16-factor scoring. sub2api: sticky sessions with per-group account selection.

**6 · Circuit breaking / isolation / degradation** — Aris ✅: `Guard` outside the retry loop — `BaseURL\|APIKey` three-state breaker (60s × 6 buckets, opens at ≥10 req and ≥50% errors, 30s → half-open with 1 probe) + per-key semaphore (≤32, 1s) → 503 + `Retry-After`; 429 never trips it. new-api ◐: scheduled health probes with auto-disable/recover, no threshold breaker. LiteLLM ◐: router cooldowns only. OmniRoute ✅: provider breaker (408/5xx only, 8×/12×/2× → half-open 60/30/15s) + connection cooldown (×2 backoff, honours `Retry-After`) + per-model lockout + admission queueing. sub2api ✅: account error policy, temporary-unschedulable rules and failover.

**7 · Multi-tenancy & permissions** — OmniRoute —: single-operator, only API-key scoping, IP filtering and 33 MCP scopes. Aris: user-level multi-tenancy. new-api: users/groups/tokens with fine-grained admin permissions. LiteLLM: Organizations → Teams → Users → Virtual Keys (enterprise org/team admins). sub2api: users/groups with per-user and per-account concurrency.

**8 · Authentication & login** — Aris ✅: OAuth2 + JWT + API key, **no 2FA or passkeys**. LiteLLM ✅: SSO (OIDC/SAML, free ≤5 users) + JWT-OIDC lazy key registration + SCIM (enterprise) + failed-login rate limiting. OmniRoute ◐: password login plus an optional OIDC gate.

**9 · Rate limiting & concurrency** — Aris ✅: 100 req/min per IP or API key; the token-dimension limiter exists but **TPM is deliberately effectively unlimited** (`LimitCallProxyLLMToken = 1e8`, documented as a design decision). LiteLLM ✅: four-level RPM/TPM made hard by `enforce_model_rate_limits` (429 + `retry-after: 60`); multi-instance must share Redis or limits are counted per process.

**10 · Usage metering & quota enforcement** — Aris ◐: per-model pricing (micro-units, time windows + context tiers + 5m/1h cache-creation split, snapshotted at call time) but **estimation only, no interception**; quota is a per-key request cap. new-api ✅: quota points (1 USD = 500,000) with pre-deduct → settle → adjust, plus group/token quotas and subscription quota with reset periods. LiteLLM ✅: budget family (key/user/team/team-member/tag/global) that **requires a database — without one `max_budget` fails open**. OmniRoute ◐: per-key USD spend quotas and cost analytics (subscription providers read $0). sub2api ✅: token-level cost with balance/subscription debit and 5h/1d/7d windows (a **post-request soft cap**, so in-flight requests can overshoot).

**11 · Call audit & usage stats** — Aris ✅: per-call `ModelCallAudit` (four token dimensions, first-token vs stream latency, upstream status, user agent, Trace ID, estimated cost) written asynchronously. new-api ✅: usage/drawing/task logs with tokens, cost, duration and first-byte time, request and upstream request IDs; admins also see the retry chain, request conversion and billing path. sub2api ◐: dashboard-level request/token/cost and account state only.

**12 · Conversation body persisted** — Aris ✅: every call's history lands in the Session aggregate, Messages/Tools deduplicated by checksum and stored protocol-neutrally, same-prefix snapshots merged in real time (exceptions: trigger `omit` and the Playground). new-api —: log fields are tokens/cost/latency/IDs only; the issues requesting prompt logging (#924, #2721, #2982, #4852) are all closed. LiteLLM ◐: off by default, opt in with `store_prompts_in_spend_logs`, with `turn_off_message_logging` / `no-log` for redaction.

**13 · Conversation dataset export** — Aris ✅: ShareGPT JSONL streaming export (SSE with progress) preserving multi-turn context, `<think>` reasoning and tool calls, with score/model/time filters and pre-export statistics. LiteLLM ◐: Lens datasets (Export JSONL) are **evaluation**-oriented, need the separate Lens service + ClickHouse and have no ShareGPT/SFT format.

**14 · Agent Harness trace ingestion** — Aris ✅: a separately built `aris` CLI registers Codex and Claude Code hooks, is fail-open, spools locally (`0600`, 256 MiB cap) and batch-uploads. LiteLLM ✅: three coexisting mechanisms — `litellm.agent()` SDK harness (8 typed events), `lite claude\|codex\|opencode` wrappers tagging `harness,<name>`, and a server-side `agentic_loop_hook` (default 3 re-runs, not triggered by streaming Chat); all beta, sandbox needs the CLI preinstalled. OmniRoute ◐: one-command CLI onboarding and MITM/TPROXY capture — traffic-level, not hook-event level.

**15 · Trace rebuild & visualisation** — Aris ✅: the server rebuilds events into a readable `TraceConversation` rendered as a message/tool/`session_meta` timeline with expandable raw payloads. LiteLLM ✅: Lens (separate service + ClickHouse) with conversation view, subagent branches, Investigations and OTLP APIs — early access, `/lens` is proxy-admin only, Claude Code relies on its own OTel exporter (documented gaps) and Codex on a third-party integration (macOS-only auto-install). OmniRoute ◐: Studios visualise routing cascades and compression waterfalls, not agent call chains.

**16 · Content safety & masking** — Aris ✅: Aho-Corasick trigger words with `deny` / `omit` / `capture` actions, hot-reloaded via Redis pub/sub and version polling. LiteLLM ✅: ~50 guardrail integrations in four modes, but masking does **not** apply to streamed output and `llmguard` / `llamaguard` / `hide_secrets` / `lakera` need the enterprise licence. new-api ◐: per-channel extra settings and upstream header overrides only.

**17 · Observability & runtime monitoring** — LiteLLM ✅: the fullest metric families (TTFT, overhead, budget remaining, admission/rejection, DB queues), two official Grafana dashboards and a dedicated metrics port that bypasses virtual-key auth (do not expose publicly). new-api ◐: dashboards, health probes with webhook alerts, pprof/Pyroscope, and a custom `/api/perf-metrics` route — but **no Prometheus dependency**. sub2api ◐: dashboard and channel monitor, no Prometheus dependency in `go.mod`. OmniRoute ◐: dashboards, `X-OmniRoute-*` headers and a SQLite audit trail, but no `/metrics`.

**18 · Token compression** — Aris ✅: deterministic tool-output compression applied in place on typed DTOs before forwarding, with `smart_crusher` / `log_compressor` / `search_compressor` / `passthrough` strategies and before/after records. OmniRoute ✅: a 12-engine pipeline (RTK, Caveman, LLMLingua-2, GCF v3.2, OmniGlyph) with a default-on inflation guard. LiteLLM ◐: `compress()` (beta).

**19 · MCP / A2A** — LiteLLM ✅: MCP Gateway (per-key server/tool permissions, MCP cost tracking, `/mcp` usable from Cursor) + A2A Agent Gateway. OmniRoute ✅: built-in MCP server with 110 tools (stdio/HTTP/SSE, 33 scopes) + A2A v0.3. new-api ◐: JS task plugins and a marketplace only. sub2api ◐: a `hashicorp/go-plugin` system only.

**20 · Deployment & horizontal scaling** — Aris ✅: Compose/K8s with `preStop` + `terminationGracePeriodSeconds: 660` and an ordered eight-step graceful shutdown that drains SSE; cross-pod state in Redis. new-api ✅: multi-node needs a shared main DB and identical `SESSION_SECRET`/`CRYPTO_SECRET`, plus Redis for shared rate limits. LiteLLM ✅: Helm (HPA/KEDA), official AWS/GCP Terraform (no Azure), cosign-signed images, Prisma migrations, Redis required for multi-instance. OmniRoute ◐: single-node SQLite, with a remote mode driven by scoped tokens.

**21 · Extensibility** — Aris ◐: no plugin system — extension points are the separately built `aris` client (one adapter file + two registry lines per agent), models.dev import and Fx wiring. new-api ✅: JS task plugins with a documented API and marketplace, plus custom OAuth providers. LiteLLM ✅: eight `CustomLogger` hooks, guardrail APIs in any language, custom auth/SSO/secret managers, provider registration by editing one JSON, callbacks from `s3://`/`gcs://`. OmniRoute ✅: plugin framework, marketplace and skills frameworks. sub2api ✅: `hashicorp/go-plugin` and module skills.

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
