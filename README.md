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
- `GET /healthz` - Service health status
  - **Response (`200 OK`)**:
    ```json
    { "status": "ok" }
    ```

---

### Authentication & Sessions

#### Register Member
`POST /api/v1/auth/register` *(Rate Limit: 3/hr per IP)*
- **Request Body**:
  ```json
  {
    "email": "tunde@example.com",
    "phone": "08012345678",
    "password": "securepassword123",
    "first_name": "Tunde",
    "last_name": "Adeleke",
    "emergency_contact": {
      "name": "Folake Adeleke",
      "phone": "08087654321",
      "relationship": "spouse"
    },
    "role": "member"
  }
  ```
  > `relationship` must be one of: `spouse`, `parent`, `sibling`, `child`, `friend`, `relative`, `colleague`, `other`.
- **Response (`201 Created`)**:
  ```json
  {
    "user": {
      "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "member_code": "EMT-104928",
      "email": "tunde@example.com",
      "phone": "08012345678",
      "first_name": "Tunde",
      "last_name": "Adeleke",
      "role": "member",
      "status": "active",
      "emergency_contact": {
        "name": "Folake Adeleke",
        "phone": "08087654321",
        "relationship": "spouse"
      },
      "created_at": "2026-11-27T20:00:00Z"
    },
    "tokens": {
      "access_token": "eyJhbGciOiJIUzI1NiIsIn...",
      "refresh_token": "7a3f81e09c...",
      "expires_in": 900
    }
  }
  ```

#### Login
`POST /api/v1/auth/login` *(Rate Limit: 5/15m Account, 20/15m IP)*
- **Request Body**:
  ```json
  {
    "email": "tunde@example.com",
    "password": "securepassword123"
  }
  ```
- **Response (`200 OK`)**:
  ```json
  {
    "user": {
      "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "member_code": "EMT-104928",
      "email": "tunde@example.com",
      "role": "member"
    },
    "tokens": {
      "access_token": "eyJhbGciOiJIUzI1NiIsIn...",
      "refresh_token": "9b1c23d45e...",
      "expires_in": 900
    }
  }
  ```

#### Refresh Token
`POST /api/v1/auth/refresh`
- **Request Body**:
  ```json
  {
    "refresh_token": "9b1c23d45e..."
  }
  ```
- **Response (`200 OK`)**:
  ```json
  {
    "tokens": {
      "access_token": "eyJhbGciOiJIUzI1NiIsIn...",
      "refresh_token": "1a2b3c4d5e...",
      "expires_in": 900
    }
  }
  ```

#### Logout
`POST /api/v1/auth/logout`
- **Request Body**:
  ```json
  {
    "refresh_token": "1a2b3c4d5e..."
  }
  ```
- **Response (`200 OK`)**:
  ```json
  {
    "message": "logged out successfully"
  }
  ```

---

### Members & Profiles

#### Get Profile
`GET /api/v1/users/me` *(Authenticated)*
- **Response (`200 OK`)**: User profile object with `member_code`, `status`, and `emergency_contact`.

#### Upload Personal Avatar
`POST /api/v1/users/me/avatar` *(Authenticated)*
- **Request Content-Type**: `multipart/form-data` (form field: `avatar` or `file`, max 10MB)
- **Response (`200 OK`)**:
  ```json
  {
    "message": "profile picture updated successfully",
    "avatar_url": "https://res.cloudinary.com/your-cloud/image/upload/v1/baki/profiles/...",
    "user": { "id": "...", "avatar_url": "https://res.cloudinary.com/..." }
  }
  ```

#### Search & List Members
`GET /api/v1/members?page=1&page_size=20&search=Adeleke` *(`admin`, `staff`)*
- **Response (`200 OK`)**:
  ```json
  {
    "members": [ ... ],
    "total": 45,
    "page": 1,
    "page_size": 20
  }
  ```

#### List Expired Members
`GET /api/v1/members/expired?page=1&page_size=20&search=EMT` *(`admin`, `staff`)*
- **Response (`200 OK`)**:
  ```json
  {
    "expired_members": [
      {
        "user_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
        "member_code": "EMT-104928",
        "first_name": "Tunde",
        "last_name": "Adeleke",
        "email": "tunde@example.com",
        "phone": "08012345678",
        "avatar_url": "https://res.cloudinary.com/...",
        "plan_name": "Monthly",
        "subscription_id": "c1f7a0b3-...",
        "end_date": "2026-11-20T10:00:00Z",
        "days_expired": 7
      }
    ],
    "total": 12,
    "page": 1,
    "page_size": 20
  }
  ```

#### Verify Member Access at Desk
`GET /api/v1/members/verify?query=EMT-104928` *(`admin`, `staff`)*
- Query parameter `query` can be a Member Code (`EMT-XXXXXX`), phone number, or email address.
- **Response (`200 OK`)**:
  ```json
  {
    "has_access": true,
    "member": {
      "id": "3fa85f64-...",
      "member_code": "EMT-104928",
      "first_name": "Tunde",
      "last_name": "Adeleke",
      "avatar_url": "https://res.cloudinary.com/...",
      "status": "active"
    },
    "subscription": {
      "id": "c1f7a0b3-...",
      "status": "active",
      "start_date": "2026-11-01T10:00:00Z",
      "end_date": "2026-12-01T10:00:00Z"
    },
    "days_remaining": 4
  }
  ```

#### Scan QR Code Access
`GET /api/v1/members/qr/{code}` *(`admin`, `staff`)*
- Verifies member code embedded in QR badge. Same response payload as `/verify`.

#### Get Member Details
`GET /api/v1/members/{id}` *(`admin`, `staff`)*
- **Response (`200 OK`)**: Full member profile, current subscription, and past payment logs.

#### Update Member Details
`PUT /api/v1/members/{id}` *(`admin`, `staff`)*
- **Request Body**:
  ```json
  {
    "first_name": "Tunde",
    "last_name": "Adeleke-Jones",
    "phone": "08012345679",
    "email": "tunde.new@example.com",
    "emergency_contact": {
      "name": "Folake Adeleke-Jones",
      "phone": "08087654321",
      "relationship": "spouse"
    },
    "status": "active"
  }
  ```
- **Response (`200 OK`)**: Updated User object.

#### Desk Upload Member Avatar
`POST /api/v1/members/{id}/avatar` *(`admin`, `staff`)*
- **Request Content-Type**: `multipart/form-data` (form field: `avatar` or `file`, max 10MB)
- **Response (`200 OK`)**:
  ```json
  {
    "message": "member profile picture updated successfully",
    "avatar_url": "https://res.cloudinary.com/...",
    "user": { ... }
  }
  ```

---

### Membership Plans

#### List Plans
`GET /api/v1/plans` *(Public / Authenticated)*
- **Response (`200 OK`)**: Array of active membership plans.

#### Get Plan
`GET /api/v1/plans/{id}`
- **Response (`200 OK`)**: Single MembershipPlan object.

#### Create Plan
`POST /api/v1/plans` *(`admin`)*
- **Request Body**:
  ```json
  {
    "name": "VIP Unlimited",
    "description": "Full gym & spa access for 1 month",
    "duration_days": 30,
    "price_cents": 3500000,
    "currency": "NGN"
  }
  ```
- **Response (`201 Created`)**: Created MembershipPlan object.

#### Update Plan
`PUT /api/v1/plans/{id}` *(`admin`)*
- **Request Body**:
  ```json
  {
    "name": "VIP Unlimited Premium",
    "description": "Full gym, personal trainer & spa access",
    "duration_days": 30,
    "price_cents": 4000000,
    "currency": "NGN",
    "is_active": true
  }
  ```
- **Response (`200 OK`)**: Updated MembershipPlan object.

---

### Subscriptions

#### Get Active Subscription
`GET /api/v1/subscriptions/me` *(Authenticated Member)*
- **Response (`200 OK`)**:
  ```json
  {
    "subscription": {
      "id": "c1f7a0b3-...",
      "user_id": "3fa85f64-...",
      "plan_id": "e2a1b0c9-...",
      "start_date": "2026-11-20T10:00:00Z",
      "end_date": "2026-12-20T10:00:00Z",
      "status": "active",
      "auto_renew": false
    },
    "days_remaining": 23
  }
  ```

#### Get Subscription History
`GET /api/v1/subscriptions/history` *(Authenticated Member)*
- **Response (`200 OK`)**: Array of all past and present subscriptions for the authenticated member.

#### Assign Subscription Manually
`POST /api/v1/subscriptions` *(`admin`, `staff`)*
- **Request Body**:
  ```json
  {
    "user_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
    "plan_id": "e2a1b0c9-4826-4447-b892-71092a98f123",
    "auto_renew": false
  }
  ```
- **Response (`201 Created`)**: Activated/Extended Subscription object with stacked dates.

---

### Payments

#### Get Payment Quote
`GET /api/v1/payments/quote?plan_id={plan_id}` *(Authenticated)*
- Optional query parameter `user_id={id}` for staff calculating quotes for members at the desk.
- **Response (`200 OK`)**:
  ```json
  {
    "plan_id": "e2a1b0c9-4826-4447-b892-71092a98f123",
    "plan_name": "Monthly",
    "plan_price_cents": 2000000,
    "first_timer_fee_cents": 500000,
    "is_first_timer": true,
    "total_amount_cents": 2500000,
    "currency": "NGN"
  }
  ```

#### Record Front-Desk Manual Payment
`POST /api/v1/payments/manual` *(`admin`, `staff`)*
- **Request Body**:
  ```json
  {
    "user_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
    "plan_id": "e2a1b0c9-4826-4447-b892-71092a98f123",
    "channel": "pos",
    "amount_cents": 2500000,
    "notes": "Paid via Zenith POS Terminal at front desk"
  }
  ```
  > `channel` must be one of: `cash`, `pos`, `bank_transfer`.
- **Response (`201 Created`)**:
  ```json
  {
    "payment": {
      "id": "8a7c2b1e-...",
      "user_id": "3fa85f64-...",
      "plan_id": "e2a1b0c9-...",
      "amount_cents": 2500000,
      "currency": "NGN",
      "channel": "pos",
      "status": "successful",
      "reference": "MANUAL-1764284400-...",
      "notes": "Paid via Zenith POS Terminal at front desk",
      "created_at": "2026-11-27T20:00:00Z"
    },
    "subscription": {
      "id": "c1f7a0b3-...",
      "user_id": "3fa85f64-...",
      "plan_id": "e2a1b0c9-...",
      "start_date": "2026-11-27T20:00:00Z",
      "end_date": "2026-12-27T20:00:00Z",
      "status": "active"
    }
  }
  ```

#### Initialize Paystack Online Checkout
`POST /api/v1/payments/initialize` *(Authenticated Member)*
- **Request Body**:
  ```json
  {
    "plan_id": "e2a1b0c9-4826-4447-b892-71092a98f123",
    "callback_url": "http://localhost:8080/api/v1/payments/verify"
  }
  ```
- **Response (`200 OK`)**:
  ```json
  {
    "authorization_url": "https://checkout.paystack.com/0123456789abcdef",
    "access_code": "0123456789abcdef",
    "reference": "baki_pay_1764284400_abc"
  }
  ```

#### Verify Online Payment
`POST /api/v1/payments/verify` *(Authenticated)*
- **Request Body**:
  ```json
  {
    "reference": "baki_pay_1764284400_abc",
    "plan_id": "e2a1b0c9-4826-4447-b892-71092a98f123"
  }
  ```
- **Response (`200 OK`)**:
  ```json
  {
    "payment": {
      "id": "8a7c2b1e-...",
      "reference": "baki_pay_1764284400_abc",
      "status": "successful",
      "channel": "paystack",
      "amount_cents": 2000000
    },
    "subscription": {
      "id": "c1f7a0b3-...",
      "status": "active",
      "start_date": "2026-11-27T20:00:00Z",
      "end_date": "2026-12-27T20:00:00Z"
    }
  }
  ```

#### List Payments
`GET /api/v1/payments?page=1&page_size=20` *(Members see own history; Admin/Staff see all)*
- **Response (`200 OK`)**:
  ```json
  {
    "payments": [ ... ],
    "total": 18,
    "page": 1,
    "page_size": 20
  }
  ```

#### Paystack Webhook Listener
`POST /api/v1/payments/paystack/webhook`
- **Request Header**: `x-paystack-signature: <hmac-sha512-hex-hash>`
- **Request Body**: Paystack transaction event JSON
- **Response (`200 OK`)**:
  ```json
  { "status": "ok" }
  ```

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
