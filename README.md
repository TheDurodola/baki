# Baki

A robust, modular monolith backend server for gym customer management, subscriptions, and payments, written in idiomatic **Go** using the **Go Standard Library (`net/http`)** and **MongoDB**.

---

## Background & Origin

This project is a proposed software solution designed to address operational challenges observed first-hand at **Emmatos Gym in Ijaiye**. 

As a regular gym member employing the **"Fly on the Wall" observation technique**, I identified critical friction points across daily gym operations:
- **Subscription Tracking & Revenue Leakage**: Lack of an automated system to verify who has paid, leading to awkward front-desk interactions and manual guesswork.
- **Lapsed Members & Retention**: Front-desk staff having no centralized view of expired members to follow up with renewal reminders.
- **Renewal Stacking Issues**: Members renewing early fearing they would lose their remaining active days.
- **First-Timer Onboarding**: Inconsistent enforcement and auditing of the mandatory ₦5,000 first-timer registration fee.
- **Gym Safety & Emergency Readiness**: Inability to quickly contact family or emergency guardians during medical or physical incidents.
- **Identity Fraud**: Difficulty visually confirming whether a member scanning in matches the registered account.

**Baki** directly resolves these problems with instant QR/code verification, automated stacking renewals, strict timestamp expiration cut-offs, mandatory typed emergency contacts, Cloudinary profile photo verification, and an expired subscription tracker for staff.

---

## Features

- **Customer & Member Management**: Full lifecycle management of gym members, contact info, member codes (`EMT-XXXXXX`), and status tracking (`active`, `suspended`, `inactive`).
- **Profile Picture Uploads via Cloudinary**: Direct profile photo capture for members and front-desk staff to eliminate card sharing and identity fraud.
- **Mandatory Emergency Contacts**: Strict requirement at sign-up with typed relationships (`spouse`, `parent`, `sibling`, `child`, `friend`, `relative`, `colleague`, `other`).
- **Official Fee Structure & Plans**:
  - **First-Timer Registration**: Strict ₦5,000 onboarding fee for members without previous subscriptions.
  - **Monthly**: ₦20,000 (30 days)
  - **3 Months**: ₦50,000 (90 days)
  - **6 Months**: ₦100,000 (180 days)
  - **1 Year**: ₦180,000 (365 days)
  - Transparent pricing quote endpoint via `/api/v1/payments/quote` and automated database seeding on boot.
- **Stacking Renewals & Strict Cut-Off**: Early renewals automatically extend from the existing `end_date`, while access strictly terminates the exact second `NOW() >= end_date`.
- **Expired Subscriptions Tracker**: Dedicated endpoint for staff/admin to view and search lapsed members for follow-ups.
- **Dual Payment Channels**:
  - **Online via Paystack**: Initialize checkout, verify transactions, and handle webhooks with HMAC-SHA512 verification.
  - **Front-Desk Offline Ledger**: Record Cash, POS terminal, and Bank Transfer payments with staff accountability.
- **Access & Refresh Token Authentication**:
  - 15-minute JWT Access Tokens for stateless authorization.
  - 7-day cryptographically secure Refresh Tokens with automatic token rotation and explicit logout revocation.
- **Advanced In-Memory Rate Limiting**:
  - **Login Protection (Sliding Window Log)**: Dual-key defense enforcing 5 attempts / 15 mins per account and 20 attempts / 15 mins per IP.
  - **Sign-Up Protection (Sliding Window Log)**: 3 registrations / hour per IP to block bot storms.
  - **General API Traffic (Token Bucket)**: 60 requests/minute with a burst capacity of 10.
- **Zero Framework Bloat**: Built cleanly on the Go standard library (`http.ServeMux` with Go 1.22+ method and pattern routing).
- **Database**: MongoDB 7.0+ (Atlas or self-hosted) with official `go.mongodb.org/mongo-driver/v2` and automated index management.

---

## Tech Stack

- **Language**: Go 1.22+ (tested on Go 1.25)
- **HTTP Routing**: Go Standard Library `net/http` (`http.ServeMux` pattern matching)
- **Database**: MongoDB 7.0+
- **Database Driver**: Official `go.mongodb.org/mongo-driver/v2`
- **Authentication**: JWT (`golang-jwt/jwt/v5`), Bcrypt hashing, Access & Refresh Token rotation
- **File & Media Storage**: Cloudinary REST API
- **Payments**: Paystack API & HMAC-SHA512 Webhooks + Front-Desk Cash/POS ledger

---

## Project Structure

```
baki/
├── cmd/
│   └── server/
│       └── main.go                  # Application entry point & router setup
├── internal/
│   ├── config/                      # Environment variable configuration
│   ├── database/                    # MongoDB connection, automated index setup & plan seeding
│   ├── domain/                      # Domain entities (User, Plan, Subscription, Payment, Auth)
│   ├── gateway/                     # External services (Paystack API, Cloudinary Image Upload)
│   ├── handler/                     # HTTP handlers & JSON helpers (net/http)
│   ├── middleware/                  # Logger, CORS, Panic Recovery, JWT Auth, RBAC & Rate Limiters
│   ├── repository/                  # Database access layer (MongoDB collections)
│   └── service/                     # Business logic (Members, Subscriptions, Payments, Auth)
├── docker-compose.yml               # Local MongoDB 7.0 instance for development
├── Makefile                         # Common build, run, and test targets
├── go.mod
└── go.sum
```

---

## Getting Started

### Prerequisites

- [Go 1.22+](https://go.dev/dl/) installed
- [Docker](https://docs.docker.com/get-docker/) & Docker Compose (for local MongoDB) or MongoDB Atlas connection string
- A [Paystack](https://paystack.com/) account (Test secret and public keys)
- A [Cloudinary](https://cloudinary.com/) account (optional for profile image uploads)

### 1. Clone the repository

```bash
git clone https://github.com/TheDurodola/baki.git
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

# Database (MongoDB)
MONGODB_URI=mongodb://localhost:27017
MONGODB_DATABASE=baki

# Authentication (Access & Refresh Tokens)
JWT_SECRET=super-secret-jwt-key-change-in-production
ACCESS_TOKEN_MINUTES=15
REFRESH_TOKEN_DAYS=7

# Paystack API & Webhooks
PAYSTACK_SECRET_KEY=sk_test_xxxxxxxxxxxxxxxxxxxxxxxx
PAYSTACK_PUBLIC_KEY=pk_test_xxxxxxxxxxxxxxxxxxxxxxxx
PAYSTACK_WEBHOOK_SECRET=your_paystack_webhook_secret
PAYSTACK_CALLBACK_URL=http://localhost:8080/api/v1/payments/verify

# Cloudinary (Profile Pictures)
CLOUDINARY_CLOUD_NAME=your_cloud_name
CLOUDINARY_API_KEY=your_cloudinary_api_key
CLOUDINARY_API_SECRET=your_cloudinary_api_secret
CLOUDINARY_UPLOAD_FOLDER=baki/profiles
```

### 3. Start Local Database (Optional)

If you are not using a remote MongoDB Atlas instance, start the local MongoDB container:

```bash
docker compose up -d
```

### 4. Start the Server

```bash
go run cmd/server/main.go
```

The server will be available at `http://localhost:8080`.

---

## API Reference Overview

### Health
- `GET  /healthz` - Service health status

### Authentication & Sessions
- `POST /api/v1/auth/register` - Register a new member (strictly requires `emergency_contact`) *(Rate Limit: 3/hr per IP)*
- `POST /api/v1/auth/login` - Authenticate credentials and receive token pair (`access_token`, `refresh_token`) *(Rate Limit: 5/15m Account, 20/15m IP)*
- `POST /api/v1/auth/refresh` - Rotate refresh token and obtain a fresh token pair
- `POST /api/v1/auth/logout` - Revoke current refresh token

### Members (`Admin` / `Staff` / `Member`)
- `GET  /api/v1/users/me` - Get profile of the current authenticated user
- `POST /api/v1/users/me/avatar` - Upload / update personal profile picture via Cloudinary (`multipart/form-data`)
- `GET  /api/v1/members` - Search & paginate members (`admin`, `staff`)
- `GET  /api/v1/members/expired` - List all members with expired subscriptions with pagination & search (`admin`, `staff`)
- `GET  /api/v1/members/verify?query=...` - Front-desk verify access by phone, email, or member code (`admin`, `staff`)
- `GET  /api/v1/members/qr/{code}` - Check member access via QR code (`admin`, `staff`)
- `GET  /api/v1/members/{id}` - Get full member details, subscription & payment history
- `PUT  /api/v1/members/{id}` - Update member profile details
- `POST /api/v1/members/{id}/avatar` - Front-desk staff upload member photo via Cloudinary (`admin`, `staff`)

### Membership Plans
- `GET  /api/v1/plans` - List active membership plans (public / authenticated)
- `GET  /api/v1/plans/{id}` - Get plan details by ID
- `POST /api/v1/plans` - Create a new membership plan (`admin`)
- `PUT  /api/v1/plans/{id}` - Update a membership plan (`admin`)

### Subscriptions
- `GET  /api/v1/subscriptions/me` - Get active subscription of current logged-in member
- `GET  /api/v1/subscriptions/history` - View all past subscriptions for the logged-in member
- `POST /api/v1/subscriptions` - Assign / activate subscription manually (`admin`, `staff`)

### Payments
- `GET  /api/v1/payments/quote?plan_id=...` - Calculate pricing quote with automatic ₦5,000 first-timer fee detection
- `POST /api/v1/payments/manual` - Record front-desk payment (Cash, POS, Bank Transfer) (`admin`, `staff`)
- `POST /api/v1/payments/initialize` - Initialize Paystack checkout for a membership plan
- `POST /api/v1/payments/verify` - Verify an online Paystack transaction reference
- `GET  /api/v1/payments` - List payment history (filtered to user for members, all for staff)
- `POST /api/v1/payments/paystack/webhook` - Paystack webhook listener (HMAC-SHA512 verified)

---

## Testing

Run all unit, integration, and security tests:

```bash
# Run tests
go test -v ./...

# Run tests with race detector
go test -v -race ./...
```

---

## Author

- **Durodola Abolaji** - [@TheDurodola](https://github.com/TheDurodola)

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
