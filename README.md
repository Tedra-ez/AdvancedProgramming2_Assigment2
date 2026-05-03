# AP2 Assignment 3
## Medical Scheduling Platform — Message Queue & Database Migrations

### What changed from Assignment 2
- **PostgreSQL replaces in-memory storage** in both services (repository interfaces preserved).
- **Schema is managed only via versioned migrations** (`golang-migrate`) under each service’s `migrations/` folder.
- **Asynchronous domain events** are published after successful write operations.
- **New third service: `notification-service`** subscribes to events and prints **one JSON log line per event**.

### Broker choice
This project uses **NATS (core)**.
- **Why**: simplest local setup and fits the “fire-and-forget notifications” requirement.
- **If switching to RabbitMQ**: publishers would publish to a `fanout` exchange (`ap2.events`) and subscribers would bind exclusive queues; durable delivery would require durable queues + publisher confirms (RabbitMQ) or JetStream (NATS).

### Architecture diagram

```mermaid
flowchart LR
  C[Client / grpcurl] --> A[Appointment Service :50052]
  A -->|GetDoctor (gRPC)| D[Doctor Service :50051]

  D --> DDB[(PostgreSQL: doctor DB)]
  A --> ADB[(PostgreSQL: appointment DB)]

  D -->|publish doctors.created| N[(NATS :4222)]
  A -->|publish appointments.created| N
  A -->|publish appointments.status_updated| N

  N -->|subscribe all subjects| S[Notification Service]
```

### Services
- **`doctor-service`**: owns `Doctor` data + validation.
- **`appointment-service`**: owns `Appointment` data + status transitions; validates doctor existence via gRPC.
- **`notification-service`**: no gRPC, no DB, no ports — only subscribes and logs events.

## Environment variables

### doctor-service
- **`DB_DSN`** or **`DATABASE_URL`**: PostgreSQL connection string (required)
- **`NATS_URL`**: e.g. `nats://localhost:4222` (optional; events are disabled if missing)
- **`GRPC_ADDR`**: gRPC listen address (default `:50051`)

### appointment-service
- **`DB_DSN`** or **`DATABASE_URL`**: PostgreSQL connection string (required)
- **`DOCTOR_SERVICE_ADDR`**: default `localhost:50051`
- **`NATS_URL`**: e.g. `nats://localhost:4222` (optional; events are disabled if missing)
- **`GRPC_ADDR`**: gRPC listen address (default `:50052`)

### notification-service
- **`NATS_URL`**: e.g. `nats://localhost:4222` (required)

## Infrastructure setup (Docker)

### NATS
```bash
docker run --name ap2-nats -p 4222:4222 -d nats:2
```

### PostgreSQL (two isolated databases)
Doctor DB:
```bash
docker run --name ap2-doctor-db -e POSTGRES_PASSWORD=pass -e POSTGRES_DB=doctor_db -p 5433:5432 -d postgres:16
```

Appointment DB:
```bash
docker run --name ap2-appointment-db -e POSTGRES_PASSWORD=pass -e POSTGRES_DB=appointment_db -p 5434:5432 -d postgres:16
```

Example DSNs:
- doctor-service: `postgres://postgres:pass@localhost:5433/doctor_db?sslmode=disable`
- appointment-service: `postgres://postgres:pass@localhost:5434/appointment_db?sslmode=disable`

## Migrations
- Location:
  - `doctor-service/migrations/`
  - `appointment-service/migrations/`
- **Run automatically on startup** (before gRPC starts accepting requests).
- **Down migrations are included** and undo the corresponding up migration.

## Startup order
1. Start PostgreSQL containers
2. Start NATS
3. Start `doctor-service`
4. Start `appointment-service`
5. Start `notification-service` (so you can see logs immediately)

Commands:
```bash
cd doctor-service && go run .
```
```bash
cd appointment-service && go run .
```
```bash
cd notification-service && go run .
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
- **Fields**: `event_type`, `occurred_at`, `id`, `old_status`, `new_status`

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

## a
// test