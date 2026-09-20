# goapi

A production-grade REST API built with **only the Go standard library's
`net/http`** (Go 1.22+ method-aware routing) — no web framework. Includes JWT
auth, bcrypt password hashing, pagination/filtering/sorting, SQL migrations,
    structured logging, graceful shutdown, and a full test suite.

## Stack

| Concern            | Choice                                                                 |
|--------------------|-------------------------------------------------------------------------|
| HTTP routing       | `net/http.ServeMux` (stdlib, Go 1.22+ `"METHOD /path/{param}"` patterns) |
| Database           | PostgreSQL via `database/sql` + `github.com/lib/pq`                     |
| Password hashing   | bcrypt (vendored from `golang.org/x/crypto/bcrypt`, zero extra runtime deps) |
| Auth               | JWT (HS256) access + refresh tokens via `golang-jwt/jwt/v5`             |
| Migrations         | Hand-rolled, embedded `.sql` files, no external migration tool          |
| Logging            | `log/slog` structured JSON logs                                        |
| Tests              | stdlib `testing` + `httptest` + `DATA-DOG/go-sqlmock` (no real DB needed) |

## Architecture

```
cmd/api/            entrypoint: config -> db -> migrate -> router -> graceful shutdown
internal/
  config/            env-driven configuration
  database/          connection pool + migration runner
  migrations/        embedded *.up.sql / *.down.sql
  models/            domain types + request/response DTOs
  repository/        parameterized SQL, one file per table
  service/           business logic, validation, ownership rules
  handlers/          HTTP handlers (thin: decode -> call service -> encode)
  middleware/         auth, logging, recovery, CORS, rate limiting, request ID
  router/            wires repos -> services -> handlers -> routes -> middleware
  utils/             JWT, bcrypt wrapper, pagination/sort parsing, JSON helpers
  pkg/bcrypt/blowfish vendored bcrypt implementation (see note below)
tests/               black-box HTTP handler integration tests
```

This is a classic layered architecture: **handlers** only translate HTTP <-> Go types, **services** hold business rules (ownership checks, validation), and **repositories** are the only place that knows SQL. Every layer depends only inward (handlers -> services -> repositories), which is what keeps each layer independently testable.

### Why vendored bcrypt?

`golang.org/x/crypto/bcrypt` has zero external dependencies of its own (`blowfish` lives in the same module), so the sandbox this was built in vendored those two packages verbatim under `internal/pkg/`. In a normal environment with full network access, just run:

```bash
go get golang.org/x/crypto/bcrypt
```

and swap the import in `internal/utils/password.go` back to `golang.org/x/crypto/bcrypt` — the vendored copy is there for zero-friction `go build` in restricted network environments, not because it should be your first choice.

## Getting started

### Option A: Docker Compose (Postgres + API)

```bash
cp .env.example .env      # edit JWT_SECRET etc.
docker compose up --build
```

The API listens on `http://localhost:8080`. Migrations run automatically on startup.

### Option B: Local Go + your own Postgres

```bash
createdb goapi
cp .env.example .env
export $(cat .env | xargs)
go run ./cmd/api
```

### Running tests

```bash
make test          # go test ./... -race
make test-cover     # with coverage report
```

No database is required to run the tests — repository, service, and handler tests all use `sqlmock` / `httptest`.

## Data model

**users**: `id (uuid pk)`, `email (unique)`, `password_hash`, `name`, `role (user|admin)`, timestamps
**posts**: `id (uuid pk)`, `user_id (fk -> users)`, `title`, `content`, `status (draft|published|archived)`, timestamps

## API reference

All responses are JSON. Errors follow `{"error": "...", "message": "...", "code": "..."}`.

### Auth

| Method | Path                     | Auth | Body                                  |
|--------|--------------------------|------|----------------------------------------|
| POST   | `/api/v1/auth/register`  | none | `{email, password, name}`              |
| POST   | `/api/v1/auth/login`     | none | `{email, password}`                    |
| POST   | `/api/v1/auth/refresh`   | none | `{refresh_token}`                      |

All three return `{access_token, refresh_token, token_type, expires_in, user}`. Send `Authorization: Bearer <access_token>` on protected routes.

### Posts

| Method | Path                  | Auth        | Notes                                   |
|--------|-----------------------|-------------|------------------------------------------|
| GET    | `/api/v1/posts`       | none        | list, see query params below             |
| GET    | `/api/v1/posts/{id}`  | none        | fetch one                                |
| POST   | `/api/v1/posts`       | required    | create, owned by caller                  |
| PATCH  | `/api/v1/posts/{id}`  | required    | partial update, owner only               |
| DELETE | `/api/v1/posts/{id}`  | required    | owner only                                |

**List query parameters** (`GET /api/v1/posts`):

| Param       | Default      | Notes                                          |
|-------------|--------------|--------------------------------------------------|
| `page`      | `1`          | 1-based                                          |
| `page_size` | `20`         | clamped to max `100`                             |
| `status`    | (any)        | `draft` \| `published` \| `archived`             |
| `user_id`   | (any)        | filter by author                                 |
| `search`    | (none)       | case-insensitive substring match on title        |
| `sort_by`   | `created_at` | `created_at` \| `updated_at` \| `title` (allow-listed — never raw-interpolated from arbitrary input) |
| `sort_dir`  | `desc`       | `asc` \| `desc`                                  |

Example:

```
GET /api/v1/posts?status=published&search=golang&sort_by=title&sort_dir=asc&page=2&page_size=10
```

Response shape:

```json
{
  "data": [ { "id": "...", "title": "...", "...": "..." } ],
  "page": 2,
  "page_size": 10,
  "total_items": 37,
  "total_pages": 4
}
```

### Health

| Method | Path       | Purpose                              |
|--------|------------|----------------------------------------|
| GET    | `/healthz` | liveness — always 200 once up         |
| GET    | `/readyz`  | readiness — 200 only if DB reachable  |

## Security notes

- Passwords hashed with bcrypt, cost 12 in production (configurable).
- JWTs are HS256, short-lived access tokens (15m default) + longer refresh tokens (7d default); refresh tokens can't be used as access tokens and vice versa (`typ` claim is checked).
- All SQL uses parameterized queries (`$1, $2, ...`); the only place a query string is built dynamically is `ORDER BY`, and there it's built exclusively from a fixed Go allow-list map, never from raw input.
- `JWT_SECRET` must be >= 32 chars in production or the app refuses to start.
- Ownership checks live in the service layer, not just the handler, so they can't accidentally be bypassed by adding a new route.
- Rate limiting is a simple in-process token bucket per client IP — fine for a single instance; back it with Redis for multi-instance deployments.

## Extending this

- Add more tables the same way `posts` was added: migration -> model -> repository -> service -> handler -> route.
- Swap in a real migration tool (e.g. `golang-migrate`) if you outgrow the hand-rolled runner.
- Add OpenAPI/Swagger docs by hand or via a generator once the route surface stabilizes.


psql -h localhost -p 5432 -U xybug -d goprod
