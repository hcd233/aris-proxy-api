# CODEBUDDY.md This file provides guidance to CodeBuddy when working with code in this repository.

## Build & Run Commands

```bash
# Build binary
go build -o aris-proxy-api main.go

# Run locally
go run main.go server start --host localhost --port 8080

# Database migration
go run main.go database migrate

# Object storage bucket creation
go run main.go object bucket create

# Install dependencies
go mod download

# Run all tests
go test ./...

# Run single test
go test -v -run TestName ./internal/service/

# Docker (full stack with PostgreSQL + Redis + MinIO)
docker compose -f docker/docker-compose-full.yml up -d

# Docker (production single service)
docker compose -f docker/docker-compose-single.yml up -d
```

## Architecture

This is a Go LLM proxy API that unifies upstream LLM providers (OpenAI, Anthropic) behind a single API, with user management via OAuth2/JWT. It uses **Fiber v2** as the HTTP framework with **Huma v2** layered on top for type-safe handlers and automatic OpenAPI 3.1 spec generation.

### Startup Sequence

Entry point is `main.go` → `cmd.Execute()`. The `server start` command in `cmd/server.go` orchestrates initialization:

1. `database.InitDatabase()` — PostgreSQL via GORM
2. `cache.InitCache()` — Redis client
3. `proxy.InitLLMProxyConfig()` — YAML proxy config for model routing
4. `pool.InitPoolManager()` — Pond v2 goroutine pools
5. Register global Fiber middleware (Recover → fgprof → CORS → Compress → Trace → Log)
6. Conditionally register `/docs` route (non-production only, controlled by `internal/enum/env.go`)
7. `router.RegisterAPIRouter()` — All API routes
8. `app.Listen()` — Start serving

### Two-Tier Configuration

- **Environment variables** (Viper `AutomaticEnv()`): Server settings, database credentials, OAuth2, JWT, storage, pool config. Loaded from `env/api.env`. Template at `env/api.env.template`. Keys use `_` separator (e.g., `POSTGRES_HOST`).
- **YAML config** (`config/config.yaml`): LLM proxy model routing and API keys. Uses `::` as key separator to avoid conflicts with model names containing `.` (e.g., `gpt-4.1`). Template at `config/config.yaml.tamplate`.

### Request Flow

```
Fiber (HTTP) → Global Middleware → Huma Router → Route-Group Middleware → Handler → Service → DAO/Proxy → PostgreSQL/Redis/Upstream LLM
```

**Two-level middleware architecture:**
- **Fiber-level** (`fiber.Handler`): Recover, fgprof, CORS, Compress, Trace, Log — applied globally
- **Huma-level** (`func(huma.Context, func(huma.Context))`): JWT, APIKey, RateLimiter, Permission, Lock — applied per route/group

### Route Structure

```
/health, /ssehealth              — Health checks (no auth)
/api/v1/token/refresh            — Token refresh (no auth)
/api/v1/oauth2/{provider}/login  — OAuth2 login (rate limited)
/api/v1/oauth2/{provider}/callback — OAuth2 callback (rate limited)
/api/v1/user/current             — Current user (JWT auth)
/api/v1/user/                    — Update user (JWT + permission)
/api/openai/v1/models            — OpenAI model list (API Key auth)
/api/openai/v1/chat/completions  — OpenAI chat (API Key auth)
/api/anthropic/v1/models         — Anthropic model list (API Key auth)
/api/anthropic/v1/messages       — Anthropic messages (API Key auth)
```

### Layer Responsibilities

- **`cmd/`** — Cobra CLI commands (`server start`, `database migrate`, `object bucket create`)
- **`internal/api/`** — Fiber app and Huma API singletons, initialized via `init()`. Fiber uses Sonic as JSON codec. Huma registers two security schemes: `jwtAuth` and `apiKeyAuth`.
- **`internal/router/`** — Route registration grouped by domain. Each file wires middleware and handlers. Routes use `huma.Register()` with operation configs specifying per-route middleware.
- **`internal/handler/`** — Each handler is an **interface** with a private struct implementation created via `NewXxxHandler()`. Handlers hold service references and wrap responses with `util.WrapHTTPResponse()`. Streaming responses (SSE/LLM) return `*huma.StreamResponse`.
- **`internal/service/`** — Business logic. LLM proxy services handle upstream request construction, SSE streaming relay, model name replacement, and async message storage.
- **`internal/infrastructure/database/dao/`** — Generic `baseDAO[ModelT]` provides type-safe CRUD, pagination, and batch operations. Concrete DAOs embed it. All DAOs are singletons. Soft delete uses `deleted_at` (int64 timestamp, 0 = not deleted).
- **`internal/infrastructure/database/model/`** — GORM models. `BaseModel` has ID/CreatedAt/UpdatedAt/DeletedAt. Key models: User, Message (stores UnifiedMessage JSON + SHA256 CheckSum), Session (tracks APIKeyName + MessageIDs + ToolIDs as JSON arrays), Tool (stores UnifiedTool JSON + CheckSum).
- **`internal/dto/`** — Request/response DTOs. Includes full OpenAI and Anthropic API type definitions, plus `UnifiedMessage` and `UnifiedTool` cross-provider formats with bidirectional conversion functions. `MessageContent` uses custom JSON marshal/unmarshal for union types (string | array).
- **`internal/middleware/`** — JWT decodes token and injects userID/userName/permission into context. APIKey middleware builds a reverse index from proxy config to validate keys and inject userName. RateLimiter uses Redis via `ulule/limiter`. Lock middleware uses Redis SETNX + Lua script atomic unlock.
- **`internal/proxy/`** — `LLMProxyConfig` singleton loaded from YAML. Core structure: `APIKeys` (map[name]key for proxy's own keys) and `Models` (map[alias]ModelConfig). Each `ModelConfig` maps to endpoints per `ProviderType` (openai/anthropic), containing upstream APIKey and BaseURL.
- **`internal/infrastructure/pool/`** — `PoolManager` manages Pond v2 goroutine pools: `pingPool` and `messageStorePool`. Message storage task deduplicates via SHA256 CheckSum (batch IN query), then creates messages/tools/sessions in a transaction. Uses `util.CopyContextValues()` to safely pass context to async tasks.

### LLM Proxy Flow

```
Client → /api/openai/v1/chat/completions (model=my-alias)
  → APIKeyMiddleware validates key from proxy config
  → Service looks up my-alias → finds openai endpoint config
  → Handles compatibility (e.g., max_tokens → max_completion_tokens)
  → Serializes request, replaces model with upstream actual name
  → Builds HTTP request with upstream Authorization header
  → Forwards to upstream LLM provider
  → Streaming: reads SSE line-by-line → replaces model name → relays to client → collects chunks → merges into complete message
  → Non-streaming: reads response → replaces model name → returns
  → Async: converts to UnifiedMessage → stores via Pool (dedup by CheckSum)
```

Anthropic proxy follows the same pattern but uses `x-api-key` header, `/v1/messages` path, `anthropic-version` header, and different SSE event format (event + data dual-line).

### Authentication

Two auth mechanisms applied per-route:
- **JWT** (`Authorization: Bearer <token>`) — User routes. Dual token: AccessToken (short-lived) + RefreshToken (long-lived), different secrets and expiry. Issued after OAuth2 login (GitHub/Google via strategy pattern in `internal/oauth2/`).
- **API Key** (`Authorization: Bearer <api-key>`) — LLM proxy routes. Keys defined in `config.yaml`.

### Key Patterns

1. **Interface-driven**: Handler, Service, DAO, TokenSigner, Locker, ObjDAO, Platform all define interfaces
2. **Singleton pattern**: Fiber App, Huma API, DB, Redis, DAOs, JWT Signers, LLMProxyConfig, PoolManager — all initialized once
3. **Generic DAO**: `baseDAO[ModelT]` with Go generics for type-safe CRUD
4. **Strategy pattern**: OAuth2 platform switching (`Platform` interface), object storage platform switching (COS priority > MinIO)
5. **Unified message format**: OpenAI/Anthropic DTOs → UnifiedMessage/UnifiedTool for cross-provider storage
6. **Async goroutine pool**: Post-LLM-request message storage via Pond, deduplication by SHA256 CheckSum
7. **Context-aware logging**: `logger.WithCtx(ctx)` and `logger.WithFCtx(fctx)` auto-attach traceID, userID, userName

### Key Dependencies

- **Fiber v2** + **Huma v2**: HTTP framework + OpenAPI typed handlers
- **GORM** + PostgreSQL: ORM and database
- **Redis** (go-redis): Caching, rate limiter backend, distributed locks
- **Sonic**: High-performance JSON (replaces encoding/json in Fiber)
- **Cobra/Viper**: CLI and configuration
- **Zap** + Lumberjack: Structured logging with multi-output tee (console + info/error/panic files with rotation)
- **MinIO / Tencent COS**: Object storage (abstracted via ObjDAO interface)
- **golang-jwt**: JWT token generation and validation
- **Pond v2**: Goroutine pool for async tasks
- **ulule/limiter**: Redis-backed rate limiting
- **samber/lo**: Functional utilities for Go
