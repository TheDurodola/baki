# Baki 🏋️‍♂️

A robust, modular monolith backend server for gym customer management and payments, written in idiomatic **Go** using the **Go Standard Library (`net/http`)**.

---

## ⚡ Features

- **Customer & Member Management**: Full lifecycle management of gym members, profiles, and attendance/status tracking (`active`, `suspended`, `inactive`).
- **Flexible Membership Plans**: Configurable plans (daily, monthly, quarterly, annual) with automatic subscription validity and renewal calculation.
- **Multi-Channel Payments**:
  - **Online Payments via Paystack**: Initialize checkout, verify transactions, and handle asynchronous webhook notifications with HMAC-SHA512 verification.
  - **Front-Desk Manual Payments**: Record cash, POS terminal payments, and direct bank transfers with staff audit notes.
- **Role-Based Access Control (RBAC)**:
  - `admin`: Full system control (staff accounts, membership plans, analytics, financial audit logs).
  - `staff`: Front-desk operations (register members, issue subscriptions, record cash/POS payments).
  - `member`: Customer portal access (view active subscription status, initiate online renewals).
- **Zero Framework Bloat**: Built cleanly on the Go standard library (`http.ServeMux` with Go 1.22+ method and pattern routing).
- **Database**: PostgreSQL (native support for **Supabase** or self-hosted) with connection pooling via `pgx/v5` and versioned SQL migrations.

---

## 🛠️ Tech Stack

- **Language**: Go 1.22+ (tested on Go 1.25)
- **HTTP Routing**: Go Standard Library `net/http` (`http.ServeMux`)
- **Database**: PostgreSQL / Supabase
- **Database Driver**: `jackc/pgx/v5` (connection pool `pgxpool`)
- **Authentication**: JWT (`golang-jwt/jwt/v5`) & Bcrypt password hashing
- **Payments**: Paystack API & Webhooks + Front-desk Cash/POS ledger

---

## 📁 Project Structure

```
baki/
├── cmd/
│   └── server/
│       └── main.go                  # Application entry point & router setup
├── internal/
│   ├── config/                      # Environment variable configuration
│   ├── database/                    # DB connection pool & migration runner
│   │   └── migrations/              # Versioned SQL migrations (.up.sql / .down.sql)
│   ├── domain/                      # Domain entities (User, Plan, Subscription, Payment)
│   ├── gateway/                     # External services (Paystack API & Webhook HMAC)
│   ├── handler/                     # HTTP handlers & JSON helpers (net/http)
│   ├── middleware/                  # Logging, CORS, Panic Recovery, JWT Auth & RBAC
│   ├── repository/                  # Database access layer (PostgreSQL / pgx)
│   └── service/                     # Business logic (Members, Subscriptions, Payments)
├── docker-compose.yml               # Local PostgreSQL instance for development
├── Makefile                         # Common build, run, and migration targets
├── go.mod
└── go.sum
```

---

## 🚀 Getting Started

### Prerequisites

- [Go 1.22+](https://go.dev/dl/) installed
- [Docker](https://docs.docker.com/get-docker/) & Docker Compose (optional for local PostgreSQL)
- A [Supabase](https://supabase.com/) project (or local PostgreSQL instance)
- A [Paystack](https://paystack.com/) account (Test secret key and public key)

### 1. Clone the repository

```bash
git clone https://github.com/bolaji/baki.git
cd baki
```

### 2. Configure Environment Variables

Create a `.env` file in the project root:

```bash
cp .env.example .env
```

Example `.env` configuration:

```env
# Server
PORT=8080
ENV=development

# Database (Supabase or Local Postgres)
DATABASE_URL=postgres://postgres:postgres@localhost:5432/baki?sslmode=disable

# Authentication
JWT_SECRET=super-secret-jwt-key-change-in-production
JWT_EXPIRATION_HOURS=72

# Paystack
PAYSTACK_SECRET_KEY=sk_test_xxxxxxxxxxxxxxxxxxxxxxxx
PAYSTACK_WEBHOOK_SECRET=your_paystack_webhook_secret
PAYSTACK_CALLBACK_URL=http://localhost:8080/api/v1/payments/verify
```

### 3. Start Local Database (Optional)

If you are not using a remote Supabase instance, spin up a local PostgreSQL database:

```bash
docker compose up -d
```

### 4. Run Migrations & Start the Server

```bash
# Run database migrations
make migrate-up

# Start the server
go run cmd/server/main.go
```

The server will be available at `http://localhost:8080`.

---

## 📡 API Reference Overview

### Health
- `GET /healthz` - Service health status

### Authentication
- `POST /api/v1/auth/register` - Register a new member account
- `POST /api/v1/auth/login` - Authenticate and receive a JWT token

### Members (`Admin` / `Staff` / `Member`)
- `GET  /api/v1/users/me` - Get profile of the current authenticated user
- `GET  /api/v1/members` - Search & paginate members (`admin`, `staff`)
- `GET  /api/v1/members/{id}` - Get full member details, subscription & payment history
- `PUT  /api/v1/members/{id}` - Update member profile details

### Membership Plans
- `GET  /api/v1/plans` - List active membership plans (public / authenticated)
- `POST /api/v1/plans` - Create a new membership plan (`admin`)
- `PUT  /api/v1/plans/{id}` - Update a membership plan (`admin`)

### Subscriptions
- `GET  /api/v1/subscriptions/me` - Get active subscription of current logged-in member
- `POST /api/v1/subscriptions` - Assign / activate subscription manually (`admin`, `staff`)

### Payments
- `POST /api/v1/payments/initialize` - Initialize Paystack checkout for a membership plan
- `POST /api/v1/payments/manual` - Record front-desk payment (Cash, POS, Bank Transfer) (`admin`, `staff`)
- `GET  /api/v1/payments` - List payment history (filtered to user for members, all for staff)
- `POST /api/v1/payments/paystack/webhook` - Paystack webhook listener (HMAC-SHA512 verified)

---

## 🧪 Testing

Run all unit and integration tests:

```bash
# Run tests
go test -v ./...

# Run tests with race detector
go test -v -race ./...
```

---

## 📜 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
