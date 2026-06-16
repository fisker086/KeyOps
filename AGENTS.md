# AGENTS.md

## KeyOps — Infrastructure Management Platform

Go backend (`github.com/fisker086/keyops`) + React frontend. Module path **not** the repo name.

## Project structure

- `cmd/api-server/main.go` — single binary entrypoint
- `internal/` — app modules (api, auth, bastion, k8s, ai, cloud, alert, approval, ...)
- `pkg/` — reusable libs (config, crypto, database, sshclient, casbin, ...)
- `ui/web/` — React 19 + MUI 7 + Vite + TypeScript frontend (**separate** package.json)
- `config/config.yaml` — runtime config, overridable via env vars (DB_HOST, REDIS_*, etc.)
- `docs/` — Swagger docs **auto-generated**, do not edit manually
- `sql/init.sql` — DB schema init

## Backend commands (from root Makefile)

| Command | Action |
|---|---|
| `make build-api` | Build Go binary to `bin/api-server` |
| `make dev-api` | `go run cmd/api-server/main.go` |
| `make test` | `go test -v ./...` |
| `make test-cover` | Test with coverage report |
| `make check` | `fmt -> vet -> test` (run this before pushing) |
| `make lint` | `golangci-lint run` |
| `make fmt` | `go fmt ./...` |
| `make vet` | `go vet ./...` |
| `make swagger` | Regenerate Swagger docs (requires `swag` CLI) |
| `make swagger-install` | `go install github.com/swaggo/swag/cmd/swag@latest` |

## Frontend commands (from `ui/web/`)

| Command | Action |
|---|---|
| `npm run dev` | Vite dev server, proxies `/api` and `/ws` to `localhost:8080` |
| `npm run build` | `tsc -b && vite build` (output to `dist/`) |
| `npm run lint` | `eslint .` |

## Build quirks

- Frontend is deployed separately via Nginx (split architecture in `docker-compose.yml`). The Dockerfile builds the API-only binary without embedded frontend.
- For local backend-only dev, run `make dev-api`; for frontend dev, run `npm run dev` in `ui/web/` (which proxies to the backend).

## Testing

- `make test` runs all Go tests (`./...`). No special test tags or exclude patterns found, but some tests may require DB/Redis/Mongo connectivity.
- No frontend test framework is configured.

## Deployment

- **All-in-one**: `Dockerfile` builds Go binary with embedded frontend, serves everything on `:8080` + `:2222`
- **Split**: `docker-compose.yml` runs MySQL + Redis + MongoDB + guacd + backend + frontend (Nginx reverse-proxy)
- Default login: `admin` / `admin123`
- Ports: `8080` (HTTP/API), `2222` (SSH Gateway), `4822` (Guacamole RDP)
- GHCR images: `ghcr.nju.edu.cn/fisker086/keyops-backend`, `ghcr.nju.edu.cn/fisker086/keyops-frontend`

## Config notes

- `config/config.yaml` can be overridden by env vars (DB_HOST, REDIS_ENABLED, AUTH_METHOD, etc.). See `config/config.yaml` for full list.
- Redis is **optional** (set `redis.enabled: false` for single-node). MongoDB is required when bastion uses `bastion_storage.engine: mongodb` (session recording). Bill/CUR data is stored in MySQL only.
- Bastion storage engine: MySQL or MongoDB (configurable via `bastion_storage.engine`).
