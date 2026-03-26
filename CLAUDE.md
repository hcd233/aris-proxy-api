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
- Create domain-specific constant files (e.g., `llm.go` for LLM-related constants)
- Never use magic numbers like `3` directly in function calls

### 5. Error Handling
- Do not wrap errors unnecessarily - let them propagate naturally
- Use structured logging with Zap for errors (`logger.Error("[Component] Description", zap.Error(err), zap.Fields...)`)
- Callback patterns should be avoided in favor of direct execution within the pool task

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
- [ ] Following existing code patterns for similar features
- [ ] Proper package placement (agent/, dto/, cron/)
