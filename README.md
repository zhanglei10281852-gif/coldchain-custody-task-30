# Coldchain Custody

Coldchain Custody is a pure Go backend for tracking regulated biological sample shipments from site registration through dispatch, custody handoff, temperature monitoring, quality review and final release or destruction. It is designed as a realistic operational baseline for future Go Coding Agent tasks; the baseline itself contains no intentionally seeded defect.

## Structure

```text
cmd/server                 HTTP server and graceful shutdown
cmd/seed-user              first operations account bootstrap command
internal/domain            state machines, value objects and business errors
internal/service           authentication, planning, custody, telemetry, review and queries
internal/repository        persistence contracts, filters and pages
internal/storage/sqlite   SQLite implementation, migrations and transactions
internal/httpapi           JSON HTTP handlers and API error mapping
internal/middleware        request IDs, structured logs and panic recovery
internal/worker            expiry and outbox retry processing
internal/audit             immutable audit event creation
migrations                 operator-visible SQL migration history
testdata                   stable integration fixtures
```

## Data model

The database contains users and sessions, studies, sites, sample batches, reusable containers, shipments, shipment items, custody handoffs, temperature readings, excursions, review decisions, audit events, idempotency records and outbox jobs. Foreign keys and unique indexes enforce ownership, one active handoff per shipment, one active excursion per shipment, unique readings and idempotent planning requests.

Shipment planning reserves every sample and its container in one transaction. State transitions use optimistic versions. Temperature excursions quarantine shipment samples and enqueue review work. A reviewer can clear samples for release or reject them for destruction. The worker expires unattended handoffs and retries outbox jobs with bounded backoff.

## Run locally

```bash
cp .env.example .env
go run ./cmd/server
```

Create the first operations account in a separate terminal:

```bash
DATABASE_PATH=./data/coldchain.db BOOTSTRAP_PASSWORD='change-this-local-password' go run ./cmd/seed-user
```

The server listens on `HTTP_ADDR` (default `:8080`). `GET /healthz` is a liveness check and `GET /readyz` verifies the database connection.

## Configuration

See `.env.example` for `HTTP_ADDR`, `DATABASE_PATH`, `BUSINESS_TIMEZONE`, `HANDOFF_TTL`, `SESSION_TTL`, `WORKER_INTERVAL`, `WORKER_BATCH_SIZE`, `SHUTDOWN_TIMEOUT` and `LOG_LEVEL`. No credentials are committed.

## API overview

1. `POST /api/v1/auth/login` with `{ "email": "...", "password": "..." }` returns an opaque bearer token.
2. Use `Authorization: Bearer <token>` for protected endpoints.
3. Operations users create studies, sites, containers and sample batches, then mark samples ready.
4. Plan a shipment with an `Idempotency-Key`, pack it, dispatch it and complete custody handoffs.
5. Couriers submit temperature readings. Out-of-range readings create an excursion and quarantine the shipment samples.
6. Reviewers start and decide excursions. Auditors can query immutable audit events.

Example login:

```bash
curl -s http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"operator@example.test","password":"change-this-local-password"}'
```

## Tests and checks

```bash
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build ./...
```

Integration tests use temporary SQLite databases and verify migrations, transactions, restart recovery, HTTP contracts, state transitions, idempotency, context cancellation and worker behavior.

## Future task areas

The separated domain, transaction, repository, HTTP, audit and worker boundaries support future defects involving state transitions, optimistic concurrency, transaction rollback, time-zone business days, context cancellation, error classification, idempotency, slice isolation, retry handling and audit consistency. This README intentionally does not describe any particular defect or expected fix.
