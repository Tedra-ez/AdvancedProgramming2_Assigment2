# AP2 Assignment 4 Infrastructure

## Database URLs

Doctor Service:

```text
postgres://postgres:pass@localhost:5543/doctor_db?sslmode=disable
```

Appointment Service:

```text
postgres://postgres:pass@localhost:5434/appointment_db?sslmode=disable
```

## pgAdmin4

URL:

```text
http://localhost:5050
```

Login:

```text
Email: admin@example.com
Password: admin
```

The two AP2 databases are pre-registered from `infra/pgadmin/servers.json`.
When pgAdmin asks for the database password, use:

```text
pass
```

## Redis

```bash
docker run --name ap2-redis -p 6379:6379 -d redis:7-alpine
```

URL:

```text
redis://localhost:6379
```

## Mock Gateway URL

```text
http://localhost:8088
```

## Run Services With .env Files

Doctor Service:

```bash
cd doctor-service
set -a; source .env; set +a
go run .
```

Appointment Service:

```bash
cd appointment-service
set -a; source .env; set +a
go run .
```

Notification Service:

```bash
cd notification-service
set -a; source .env; set +a
go run .
```

Mock Gateway:

```bash
cd mock-gateway
set -a; source .env; set +a
go run .
```
