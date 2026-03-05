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

# Docker (full stack with PostgreSQL + Redis)
docker compose -f docker/docker-compose-full.yml up -d

# Docker (production single service)
docker compose -f docker/docker-compose-single.yml up -d
```

## Architecture

This is a Go backend API that serves as an LLM proxy with user management. It uses **Fiber v2** as the HTTP framework with **Huma v2** layered on top for type-safe handlers and automatic OpenAPI 3.1 spec generation.

### Two-Tier Configuration

- **Environment variables** (via Viper): Server settings, database credentials, OAuth2, JWT, storage. Loaded from `env/api.env`. Template at `env/api.env.template`.
- **YAML config** (`config/config.yaml`): LLM proxy model routing and API keys. Defines upstream model endpoints and their credentials. Template at `config/config.yaml.tamplate`.

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
- **JWT** (`Authorization: Bearer <token>`) - For user-facing routes. Tokens issued after OAuth2 login (GitHub/Google).
- **API Key** (`Authorization: Bearer <api-key>`) - For OpenAI-compatible routes. Keys defined in `config.yaml`.

### Environment Handling

The `internal/enum/env.go` defines `production` and `development` environments. The `/docs` (Scalar API documentation UI) route is only registered in non-production environments.

### Key Dependencies

- **Fiber v2** + **Huma v2**: HTTP framework + OpenAPI typed handlers
- **GORM** + PostgreSQL: ORM and database
- **Redis** (go-redis): Caching and rate limiter backend
- **Sonic**: High-performance JSON (replaces encoding/json in Fiber)
- **Cobra/Viper**: CLI and configuration
- **Zap** + Lumberjack: Structured logging with rotation
- **MinIO / Tencent COS**: Object storage (abstracted)
- **golang-jwt**: JWT token generation and validation
