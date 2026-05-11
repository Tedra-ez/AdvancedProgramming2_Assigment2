# AP2 Assignment 4
## Medical Scheduling Platform — Caching & Background Jobs

### What changed from Assignment 3
- **PostgreSQL replaces in-memory storage** in both services (repository interfaces preserved).
- **Schema is managed only via versioned migrations** (`golang-migrate`) under each service’s `migrations/` folder.
- **Asynchronous domain events** are published after successful write operations.
- **New third service: `notification-service`** subscribes to events and prints **one JSON log line per event**.
- **Redis cache** is used for read operations in Doctor Service and Appointment Service.
- **Redis-backed rate limiting** protects gRPC endpoints through unary interceptors.
- **Notification Service has a worker-pool job queue** for completed appointments.
- **Mock Gateway** simulates an external notification API at `POST /notify`.

### Broker choice
This project uses **NATS (core)**.
- **Why**: simplest local setup and fits the “fire-and-forget notifications” requirement.
- **If switching to RabbitMQ**: publishers would publish to a `fanout` exchange (`ap2.events`) and subscribers would bind exclusive queues; durable delivery would require durable queues + publisher confirms (RabbitMQ) or JetStream (NATS).

### Messaging model: Pub/Sub, not Point-to-Point
This project uses **Publish/Subscribe**.
- Doctor Service and Appointment Service publish events to NATS subjects.
- Notification Service subscribes to those subjects.
- Producers do not send messages to a specific consumer or work queue.
- If another subscriber service is added later, it can subscribe to the same subjects and receive the same events.

It is **not Point-to-Point**, because messages are not targeted to exactly one worker from a queue. Point-to-Point would be more appropriate for background jobs where only one worker should process each task.

### Architecture diagram

```mermaid
flowchart LR
  C[Client / grpcurl] --> A[Appointment Service :50052]
  A -->|GetDoctor (gRPC)| D[Doctor Service :50051]

  D --> DDB[(PostgreSQL: doctor DB)]
  A --> ADB[(PostgreSQL: appointment DB)]
  D -->|cache + rate limit| R[(Redis :6379)]
  A -->|cache + rate limit| R
  S -->|idempotency keys| R

  D -->|publish doctors.created| N[(NATS :4222)]
  A -->|publish appointments.created| N
  A -->|publish appointments.status_updated| N

  N -->|subscribe all subjects| S[Notification Service]
  S -->|background job POST /notify| G[Mock Gateway :8088]
```

### Services
- **`doctor-service`**: owns `Doctor` data + validation.
- **`appointment-service`**: owns `Appointment` data + status transitions; validates doctor existence via gRPC.
- **`notification-service`**: subscribes, logs events, and runs a background worker pool.
- **`mock-gateway`**: simulated external notification API.

## Environment variables

### doctor-service
- **`DB_DSN`** or **`DATABASE_URL`**: PostgreSQL connection string (required)
- **`NATS_URL`**: e.g. `nats://localhost:4222` (optional; events are disabled if missing)
- **`REDIS_URL`**: e.g. `redis://localhost:6379`
- **`CACHE_TTL_SECONDS`**: cache TTL in seconds (default `60`)
- **`RATE_LIMIT_RPM`**: requests per minute per client IP (default `100`)
- **`GRPC_ADDR`**: gRPC listen address (default `:50051`)

### appointment-service
- **`DB_DSN`** or **`DATABASE_URL`**: PostgreSQL connection string (required)
- **`DOCTOR_SERVICE_ADDR`**: default `localhost:50051`
- **`NATS_URL`**: e.g. `nats://localhost:4222` (optional; events are disabled if missing)
- **`REDIS_URL`**: e.g. `redis://localhost:6379`
- **`CACHE_TTL_SECONDS`**: cache TTL in seconds (default `60`)
- **`RATE_LIMIT_RPM`**: requests per minute per client IP (default `100`)
- **`GRPC_ADDR`**: gRPC listen address (default `:50052`)

### notification-service
- **`NATS_URL`**: e.g. `nats://localhost:4222` (required)
- **`REDIS_URL`**: e.g. `redis://localhost:6379`
- **`GATEWAY_URL`**: e.g. `http://localhost:8088`
- **`WORKER_POOL_SIZE`**: background job workers (default `3`)

### mock-gateway
- **`GATEWAY_PORT`**: HTTP listen port (default `8088` in this local setup)

## Infrastructure setup (Docker)

### NATS
```bash
docker run --name ap2-nats -p 4222:4222 -d nats:2
```

### Redis
```bash
docker run --name ap2-redis -p 6379:6379 -d redis:7-alpine
```

### PostgreSQL (two isolated databases)
Doctor DB:
```bash
docker run --name ap2-doctor-db-defense -e POSTGRES_PASSWORD=pass -e POSTGRES_DB=doctor_db -p 5543:5432 -d postgres:16
```

Appointment DB:
```bash
docker run --name ap2-appointment-db -e POSTGRES_PASSWORD=pass -e POSTGRES_DB=appointment_db -p 5434:5432 -d postgres:16
```

Example DSNs:
- doctor-service: `postgres://postgres:pass@localhost:5543/doctor_db?sslmode=disable`
- appointment-service: `postgres://postgres:pass@localhost:5434/appointment_db?sslmode=disable`

Note: this local setup uses `5543` for the Doctor DB because `5433` may already be occupied by another PostgreSQL container.

### pgAdmin4
pgAdmin4 is available at:

```text
http://localhost:5050
```

Login:

```text
Email: admin@example.com
Password: admin
```

Registered servers:
- `AP2 Doctor DB`: `host.docker.internal:5543`, database `doctor_db`, user `postgres`, password `pass`
- `AP2 Appointment DB`: `host.docker.internal:5434`, database `appointment_db`, user `postgres`, password `pass`

### Postman gRPC
Use Postman's **gRPC** request type, not a normal HTTP request. See `postman_grpc_guide.md` for exact methods and payloads.

## Migrations
- Location:
  - `doctor-service/migrations/`
  - `appointment-service/migrations/`
- **Run automatically on startup** (before gRPC starts accepting requests).
- **Down migrations are included** and undo the corresponding up migration.

Automatic startup examples:
```bash
cd doctor-service
DB_DSN='postgres://postgres:pass@localhost:5543/doctor_db?sslmode=disable' \
NATS_URL='nats://localhost:4222' \
go run .
```

```bash
cd appointment-service
DB_DSN='postgres://postgres:pass@localhost:5434/appointment_db?sslmode=disable' \
DOCTOR_SERVICE_ADDR='localhost:50051' \
NATS_URL='nats://localhost:4222' \
go run .
```

Manual migration commands if the `migrate` CLI is installed:
```bash
migrate -path doctor-service/migrations \
  -database 'postgres://postgres:pass@localhost:5543/doctor_db?sslmode=disable' up

migrate -path appointment-service/migrations \
  -database 'postgres://postgres:pass@localhost:5434/appointment_db?sslmode=disable' up
```

Rollback example:
```bash
migrate -path doctor-service/migrations \
  -database 'postgres://postgres:pass@localhost:5543/doctor_db?sslmode=disable' down 1
```

Dockerized `migrate` example if the CLI is not installed locally:
```bash
docker run --rm -v "$PWD/doctor-service/migrations:/migrations" migrate/migrate \
  -path=/migrations \
  -database 'postgres://postgres:pass@host.docker.internal:5543/doctor_db?sslmode=disable' up
```

## Assignment 4 Design

### Cache strategies
- `GetDoctor`: Cache-Aside using `doctor:<id>`.
- `ListDoctors`: Cache-Aside using `doctors:list`.
- `CreateDoctor`: Write-Through for `doctor:<id>` and immediate eviction of `doctors:list`.
- `GetAppointment`: Cache-Aside using `appointment:<id>`.
- `ListAppointments`: Cache-Aside using `appointments:list`.
- `CreateAppointment`: Write-Around; only evicts `appointments:list`.
- `UpdateAppointmentStatus`: Write-Through for `appointment:<id>` and eviction of `appointments:list`.

Redis failures are logged and treated as cache misses. PostgreSQL remains the source of truth.

### Rate limiting
Both gRPC services use a Redis-backed **fixed-window counter** implemented as a `UnaryServerInterceptor`.
The key format is `rate:<service>:<client-ip>:<minute-window>`. Default limit is `100` requests per minute.
When exceeded, the service returns `codes.ResourceExhausted` with a retry-after message.

### Background jobs
When Notification Service receives `appointments.status_updated` with `new_status = "done"`, it:
1. Logs the event.
2. Derives an idempotency key from `event_type + id + occurred_at`.
3. Reserves `job:<idempotency_key>` in Redis for 24 hours.
4. Enqueues a job into a buffered channel.
5. A worker calls `POST /notify` on the Mock Gateway.
6. On transient failure, it retries with exponential backoff.
7. After max attempts, it writes a structured dead-letter log to stderr.

## Startup order
1. Start PostgreSQL containers.
2. Start NATS.
3. Start Redis.
4. Start `mock-gateway`.
5. Start `doctor-service`.
6. Start `appointment-service`.
7. Start `notification-service`.

Commands:
```bash
cd doctor-service && set -a; source .env; set +a; go run .
```
```bash
cd appointment-service && set -a; source .env; set +a; go run .
```
```bash
cd notification-service && set -a; source .env; set +a; go run .
```
```bash
cd mock-gateway && set -a; source .env; set +a; go run .
```

## Event contract
All events are **JSON** and include at minimum: `event_type`, `occurred_at` (RFC3339), plus required payload fields.

### doctors.created (Doctor Service)
- **Subject**: `doctors.created`
- **Trigger**: `CreateDoctor` succeeds
- **Fields**: `event_type`, `occurred_at`, `id`, `full_name`, `specialization`, `email`

### appointments.created (Appointment Service)
- **Subject**: `appointments.created`
- **Trigger**: `CreateAppointment` succeeds
- **Fields**: `event_type`, `occurred_at`, `id`, `title`, `doctor_id`, `status`

### appointments.status_updated (Appointment Service)
- **Subject**: `appointments.status_updated`
- **Trigger**: `UpdateAppointmentStatus` succeeds
- **Fields**: `event_type`, `occurred_at`, `id`, `doctor_id`, `old_status`, `new_status`

## Notification Service output
The Notification Service prints **one JSON object per received event** to stdout:

- `time`: when the message was processed (RFC3339)
- `subject`: the NATS subject
- `event`: the full deserialized payload

Example:
```json
{"time":"2026-05-01T10:24:01Z","subject":"appointments.created","event":{"event_type":"appointments.created","occurred_at":"2026-05-01T10:24:01Z","id":"appointment-...","title":"Initial cardiac consultation","doctor_id":"doctor-...","status":"new"}}
```

## Consistency trade-offs (best-effort publishing)
- If **NATS is unavailable at startup** for doctor/appointment services: the service still starts and logs a warning.
- If **publish fails during an RPC**: the RPC still succeeds; the error is logged.
- **Events can be lost** if the process crashes between DB commit and publish.

Production reliability options:
- **Outbox pattern** (DB transaction writes event to outbox table; separate worker publishes reliably)
- **Durable broker features**: RabbitMQ publisher confirms + durable queues, or NATS JetStream

## NATS vs RabbitMQ (concrete differences)
- **Durability**: core NATS is fire-and-forget (no persistence); RabbitMQ queues can be durable with acks.
- **Model**: NATS subjects are lightweight Pub/Sub; RabbitMQ typically uses exchanges + queues with routing/binding.
