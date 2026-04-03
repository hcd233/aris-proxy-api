# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

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

# Run tests
go test ./...

# Run a single test
go test -v -run TestFunctionName ./path/to/package

# Docker (full stack with PostgreSQL + Redis)
docker volume create postgresql-data
docker volume create redis-data
docker compose -f docker/docker-compose-full.yml up -d

# Docker (production single service)
docker compose -f docker/docker-compose-single.yml up -d

# Docker (dev single service - builds from local Dockerfile)
docker compose -f docker/docker-compose-dev-single.yml up -d --build
```

## Architecture

This is a Go backend API that serves as an LLM proxy with user management. It uses **Fiber v2** as the HTTP framework with **Huma v2** layered on top for type-safe handlers and automatic OpenAPI 3.1 spec generation.

### Two-Tier Configuration

- **Environment variables** (via Viper): Server settings, database credentials, OAuth2, JWT, storage. Loaded from `env/api.env`. Template at `env/api.env.template`.
- **YAML config** (`config/config.yaml`): LLM proxy model routing and API keys. Defines upstream model endpoints and their credentials. Template at `config/config.yaml.tamplate`. Uses `::` as the key delimiter to support model names containing dots (e.g., `gpt-4.1`).

### Request Flow

```
Fiber (HTTP) -> Middleware Stack -> Huma (typed handlers) -> Service -> DAO -> PostgreSQL/Redis
```

Middleware executes in order: Recover -> fgprof -> CORS -> Compress -> Trace -> Log -> (per-route: JWT / APIKey / RateLimiter / Permission)

### Layer Responsibilities

- **`cmd/`** - Cobra CLI commands (`server start`, `database migrate`, `object bucket create`)
- **`internal/api/`** - Fiber app and Huma API singletons. Fiber handles HTTP transport; Huma provides OpenAPI schema generation and typed request/response handling
- **`internal/router/`** - Route registration, groups routes by domain (health, token, oauth2, user, openai). Each router file wires middleware and handlers for its group
- **`internal/handler/`** - HTTP handlers that parse requests and call services
- **`internal/service/`** - Business logic layer
- **`internal/infrastructure/database/dao/`** - Data access objects (GORM queries)
- **`internal/infrastructure/database/model/`** - GORM database models
- **`internal/dto/`** - Data transfer objects for API request/response shapes
- **`internal/middleware/`** - JWT auth, API key auth, rate limiting, permission validation, CORS, logging, tracing
- **`internal/proxy/`** - LLM proxy logic (scaffolded, implementation in progress)
- **`internal/config/`** - Viper-based config loading for both env vars and YAML proxy config

### Authentication

Two auth mechanisms, applied per-route:
- **JWT** (`Authorization: Bearer <token>`) - For user-facing routes under `/api/v1/`. Tokens issued after OAuth2 login (GitHub/Google).
- **API Key** (`Authorization: Bearer <api-key>`) - For LLM proxy routes (`/openai/v1/*`, `/anthropic/v1/*`). Keys defined in `config.yaml`.

### Route Structure

- `/api/v1/*` - User-facing API (JWT auth): token refresh, OAuth2, user management
- `/openai/v1/*` - OpenAI-compatible endpoints (API key auth): `/models`, `/chat/completions`
- `/anthropic/v1/*` - Anthropic-compatible endpoints (API key auth): `/models`, `/messages`
- `/health`, `/ssehealth` - Health checks (no auth)
- `/docs` - Scalar API documentation UI (only in non-production environments)

### Environment Handling

The `internal/enum/env.go` defines `production` and `development` environments. The `/docs` route and OpenAPI schema endpoints are only registered in non-production environments.

### Key Dependencies

- **Fiber v2** + **Huma v2**: HTTP framework + OpenAPI typed handlers
- **GORM** + PostgreSQL: ORM and database
- **Redis** (go-redis): Caching and rate limiter backend
- **Sonic**: High-performance JSON (replaces encoding/json in Fiber)
- **Cobra/Viper**: CLI and configuration
- **Zap** + Lumberjack: Structured logging with rotation
- **MinIO / Tencent COS**: Object storage (abstracted)
- **golang-jwt**: JWT token generation and validation

## Development Guidelines & Lessons Learned

### 1. Always Study Existing Patterns First
Before implementing any feature, thoroughly examine existing similar implementations in the codebase. For example:
- When creating a new cron job, study `session_dedup.go` for structure, interface compliance, and logging patterns
- When adding pool tasks, reference `MessageStoreTask` in `dto/asynctask.go` for field naming and structure
- Follow the established patterns for error handling, logging, and documentation comments

### 2. Use Existing Infrastructure
- **PoolManager**: Always use the global `PoolManager` (`internal/infrastructure/pool/pool.go`) for goroutine pool operations. Add new pool types to the Manager struct, initialize in `InitPoolManager()`, and stop in `Stop()`.
- **DAO Layer**: All database operations must go through DAOs (`internal/infrastructure/database/dao/`). Never use `database.GetDBInstance()` directly in business logic - this should only be done in the DAO layer or infrastructure layer.

### 3. Package Organization
- Place reusable LLM/agent capabilities in `internal/agent/` (not in `internal/cron/` or `internal/llm/`)
- Keep cron jobs focused on scheduling logic only
- Task definitions belong in `internal/dto/asynctask.go`

### 4. Constants Management
- All numeric literals must be extracted to constants in `internal/common/constant/`
- **All string literals in `switch case` statements must be extracted to constants** in `internal/enum/` (e.g., content types, event types, error types)
- Create domain-specific constant files (e.g., `llm.go` for LLM-related constants, `anthropic.go` for Anthropic-specific enums)
- Never use magic numbers like `3` or string literals like `"text"` directly in function calls or case statements

### 5. Error Handling
- All internal Go errors must be created via `internal/common/ierr` package — never use `fmt.Errorf` or `errors.New` directly
- Use `ierr.Wrap(ierr.ErrXxx, err, "context")` to wrap errors with context
- Use `ierr.New(ierr.ErrValidation, "description")` for errors without a cause
- In Service layer, map to business errors via `ierr.ErrXxx.BizError()` and set `rsp.Error`
- In Middleware layer, use `ierr.ErrXxx.BizError()` with `WriteErrorResponse`
- `constant/error.go` (`constant.ErrXxx`) is **deprecated** — use `ierr.ErrXxx.BizError()` instead
- Use structured logging with Zap for errors (`logger.Error("[Component] Description", zap.Error(err), zap.Fields...)`)

### 6. Complete Message Serialization
When serializing messages for LLM processing, include ALL fields:
- Role, Name, Content (Text + Parts with all types: text, image_url, input_audio, file, refusal)
- ReasoningContent
- ToolCalls (ID, Name, Arguments)
- ToolCallID
- Refusal

### 7. Architecture Compliance Checklist
Before submitting changes:
- [ ] No direct database access outside DAO/infrastructure layer
- [ ] Using PoolManager for all concurrent operations
- [ ] Constants extracted to appropriate constant files
- [ ] **All switch case values use constants instead of string literals**
- [ ] Following existing code patterns for similar features
- [ ] Proper package placement (agent/, dto/, cron/)

---

## Development Workflow（开发流程）

**Every code change MUST follow this workflow. No exceptions.**

### Step 1: Self-Check While Coding

#### Error Handling (BLOCKING)

- **NEVER** use `fmt.Errorf` / `errors.New` — use `ierr.Wrap` / `ierr.New`
- **NEVER** use `constant.ErrXxx` — use `ierr.ErrXxx.BizError()`
- DAO/Util layer: `ierr.Wrap(ierr.ErrXxx, err, "context")`
- Service layer: `rsp.Error = ierr.ErrXxx.BizError()` + `return rsp, nil` (Go error always nil)
- Handler layer: one-liner `return util.WrapHTTPResponse(h.svc.Method(ctx, req))`
- Middleware layer: `lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrXxx.BizError()))`
- Choose the most precise sentinel error, not just `ErrInternal`

#### Logging (BLOCKING)

- Format: `"[PascalCaseModule] English message"`, e.g. `"[SessionService] Get session detail"`
- Context: `logger.WithCtx(ctx)` or `logger.WithFCtx(c)`
- Sensitive data (Key/Token/Secret/Password) MUST use `util.MaskSecret()`
- Structured fields: `zap.String()`, `zap.Error()`, `zap.Uint()`, etc.
- Levels: Error=needs human intervention, Warn=self-recoverable, Info=key business nodes, Debug=dev
- NEVER log inside loops or high-frequency paths

#### Naming (BLOCKING)

- Interface: PascalCase, no `I` prefix; Implementation: camelCase private struct
- Factory: `NewXxx()` returns interface type
- Handler methods: `Handle` prefix
- DTO: `XxxReq` / `XxxRsp` / `XxxReqBody`
- NEVER use `data1`, `tmp`, `userList`, `userMap` — use plural forms like `users`, `orders`

#### Code Structure

- Functions: prefer ≤10 lines, max 20 lines
- if nesting ≤2 levels, prefer guard clauses
- Parameters 0-3, use param object beyond that
- Extract logic appearing 2+ times — no copy-paste
- Delete dead code (commented-out code must be removed)
- Keep private unless export is necessary

#### Imports & Dependencies

- Three groups separated by blank lines: stdlib → third-party → internal
- NEVER use `encoding/json` — use `github.com/bytedance/sonic`
- NEVER use `json.RawMessage` or `any`/`interface{}`

#### Comments

- godoc format: Chinese summary on first line + `@receiver`/`@param`/`@return`/`@author`/`@update` tags
- Package comment: `// Package xxx 中文描述`

#### Architecture

- Handler: thin wrapper only, no business logic
- Service: no direct infrastructure dependencies
- All business methods: first param is `context.Context`
- Singletons: access via `GetXxx()`

### Step 2: Run Convention Linter

```bash
make lint-conv
```

Fix all ERRORs, evaluate all WARNs. Script at `script/lint-conventions.sh`.

| Check | Level | Description |
|-------|-------|-------------|
| `fmt.Errorf` / `errors.New` usage | ERROR | Must use ierr package |
| `constant.ErrXxx` usage | ERROR | Deprecated |
| `encoding/json` / `json.RawMessage` | ERROR | Use sonic |
| Test files in internal/ | ERROR | Must be in test/ |
| Scattered test files in test/ root | ERROR | Must be in subdirectory |
| testify usage | ERROR | Use stdlib testing |
| `time.Sleep` in tests | ERROR | Use channel/WaitGroup |
| Handler direct DAO/DB access | ERROR | Business logic in Service |
| Log missing `[Module]` prefix | WARN | Should fix |
| Sensitive data without MaskSecret | WARN | Should fix |
| Possible dead code | WARN | Manual review |
| Implementation-exposing names | WARN | Use plural forms |
| Service returning non-nil error | WARN | Verify correctness |
| `interface{}` in core business | WARN | Prefer concrete types/generics |

### Step 3: Write/Update Tests

See Testing section in CODEBUDDY.md.

### Step 4: Run All Tests

```bash
make test
```

All tests MUST pass before committing.
