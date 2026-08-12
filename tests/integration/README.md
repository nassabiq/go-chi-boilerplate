# Authentication Integration / E2E Tests

HTTP-level integration tests for the authentication lifecycle. They exercise the
**real** Chi router, middleware, handlers, usecases, repositories, JWT
implementation, and Redis `SessionManager` against a **real PostgreSQL** and
**real Redis** — no mocks of the auth core.

## Layout

- `setup_test.go` — builds the application stack exactly like
  `cmd/http/wire_gen.go` (config → Postgres/GORM → Redis → JWT → SessionManager
  → repositories → usecases → handlers → router), runs the real Goose
  migrations from `../../migrations`, and isolates each test (truncate users +
  `FlushDB`).
- `helpers_test.go` — HTTP client over the real router, user fixtures (real
  bcrypt hashing), token/session helpers.
- `auth_test.go` — the scenarios (login, /auth/me, refresh rotation & reuse,
  session listing, device revocation, logout, logout-all, token confusion,
  tampering, and input-validation edge cases).

## Running

The suite is guarded by the `integration` build tag so `go test ./...` never
touches the database.

### With Docker (recommended)

```bash
docker compose up -d            # postgres:5432, redis:6379
make test-integration
# or
go test -tags=integration -count=1 -v ./tests/integration/...
```

### Against existing services

Point the suite at your instances via environment variables:

| Variable                | Default            | Purpose                |
|-------------------------|--------------------|------------------------|
| `TEST_DB_HOST`          | `localhost`        | PostgreSQL host        |
| `TEST_DB_PORT`          | `5432`             | PostgreSQL port        |
| `TEST_DB_USER`          | `postgres`         | PostgreSQL user        |
| `TEST_DB_PASSWORD`      | `postgres`         | PostgreSQL password    |
| `TEST_DB_NAME`          | `postgres`         | database name          |
| `TEST_REDIS_ADDR`       | `127.0.0.1:6379`   | Redis `host:port`      |
| `TEST_REDIS_DB`         | `1`                | Redis logical DB       |
| `TEST_REDIS_PASSWORD`   | ``                 | Redis password         |
| `TEST_JWT_ACCESS_SECRET`| test secret (opaque)| JWT access secret    |
| `TEST_JWT_REFRESH_SECRET`| test secret (opaque)| JWT refresh secret  |

Then:

```bash
go test -tags=integration -count=1 -v ./tests/integration/...
```

## Isolation

Each test calls `env.reset(t)` which truncates the `users` table (identity
restart) and flushes the configured Redis DB, so tests are order-independent.

## No infrastructure?

If PostgreSQL/Redis cannot be reached, every test calls `t.Skip` with the reason
— they are **not** reported as passing. See the test output / CI log for the
skip message rather than treating `ok` as a green run.
