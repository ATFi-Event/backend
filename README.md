# ATFI Backend API

A robust Go-based REST API service for the ATFI (Event Commitment Platform) that handles user profiles, event management, and check-in functionality with blockchain integration.

## 🚀 Features

- **User Profile Management**: Create, read, and update user profiles with wallet addresses
- **Event Management**: Complete event lifecycle with on-chain and off-chain data
- **Blockchain Integration**: Ethereum/Base Sepolia integration for smart contract interactions
- **Check-in System**: QR code-based event check-in and validation
- **PostgreSQL Database**: Reliable data persistence with pgx driver
- **CORS Support**: Cross-origin resource sharing for frontend integration
- **Health Checks**: Database and service health monitoring

## 📋 Prerequisites

- **Go 1.21+** - [Install Go](https://golang.org/doc/install)
- **PostgreSQL** - [Install PostgreSQL](https://www.postgresql.org/download/)
- **Ethereum Node** - Base Sepolia RPC endpoint (default provided)

## 🛠️ Installation

### 1. Clone the Repository
```bash
git clone <repository-url>
cd backend
```

### 2. Install Dependencies
```bash
go mod download
```

### 3. Set Up Environment Variables
Copy the example environment file:
```bash
cp .env.example .env
```

Edit `.env` with your configuration:
```env
# Database Configuration
DATABASE_URL=postgres://user:password@localhost/atfi_db?sslmode=disable

# Server Configuration
PORT=8080

# Ethereum RPC Configuration
RPC_URL=https://base-sepolia-rpc.publicnode.com
```

### 4. Database Setup
Create a PostgreSQL database:
```sql
CREATE DATABASE atfi_db;
```

The application will automatically create the necessary tables on first run.

## 🚀 Running the Application

### Development Mode
```bash
go run main.go
```

### Build and Run
```bash
go build -o atfi-backend main.go
./atfi-backend
```

### Using Air for Live Reload (Optional)
Install [Air](https://github.com/cosmtrek/air) for live development:
```bash
go install github.com/cosmtrek/air@latest
air
```

## 📚 API Documentation

### Base URL
```
http://localhost:8080/api/v1
```

### 🔐 Health Check
```
GET /health
GET /api/v1/test-db
```

### 👤 User Profile Management

#### Create Profile
```http
POST /api/v1/profiles
Content-Type: application/json

{
  "wallet_address": "0x...",
  "name": "John Doe",
  "email": "john@example.com"
}
```

#### Get Profile
```http
GET /api/v1/profiles/{walletAddress}
```

#### Update Profile
```http
PUT /api/v1/profiles/{walletAddress}
Content-Type: application/json

{
  "name": "Updated Name",
  "email": "updated@example.com"
}
```

#### Upsert Profile (Create or Update)
```http
POST /api/v1/profiles/upsert
Content-Type: application/json

{
  "wallet_address": "0x...",
  "name": "John Doe",
  "email": "john@example.com"
}
```

### 🎉 Event Management

#### Create Event
```http
POST /api/v1/events
Content-Type: application/json

{
  "event_id": 1,
  "title": "Amazing Event",
  "description": "Event description",
  "location": "Event location",
  "image_url": "https://example.com/image.jpg",
  "is_public": true,
  "require_approval": false,
  "organizer_address": "0x..."
}
```

#### Get All Events
```http
GET /api/v1/events?page=1&limit=10&status=REGISTRATION_OPEN&organizer=0x...
```

#### Get Single Event
```http
GET /api/v1/events/{eventId}
```

#### Update Event Status
```http
PUT /api/v1/events/{eventId}/status
Content-Type: application/json

{
  "status": "REGISTRATION_OPEN"
}
```

#### Settle Event
```http
POST /api/v1/events/{eventId}/settle
Content-Type: application/json

{
  "attended_participants": ["0x...", "0x..."]
}
```

#### Get Attended Participants
```http
GET /api/v1/events/{eventId}/attended
```

### 📝 Event Registration

#### Register for Event
```http
POST /api/v1/events/{eventId}/register
Content-Type: application/json

{
  "userAddress": "0x...",
  "transactionHash": "0x..." // Optional for smart contract registration
}
```

#### Get User Registration
```http
GET /api/v1/events/{eventId}/registration?user=0x...
```

### ✅ Check-in Management

#### Check In User
```http
POST /api/v1/checkin
Content-Type: application/json

{
  "event_id": 1,
  "user_address": "0x...",
  "validator_address": "0x...",
  "qr_data": "{\"eventId\":\"1\",\"userAddress\":\"0x...\"}"
}
```

#### Validate Check-in
```http
POST /api/v1/checkin/validate
Content-Type: application/json

{
  "qr_data": "{\"eventId\":\"1\",\"userAddress\":\"0x...\"}",
  "validator_address": "0x..."
}
```

#### Get Event Check-ins
```http
GET /api/v1/events/{eventId}/checkins
```

## 🗄️ Database Schema

The ATFI platform uses PostgreSQL as its primary database with the following actual schema:

### Core Tables

#### `profiles`
User profile information linked to wallet addresses.

**Columns:**
- `id` (UUID, Primary Key) - Unique identifier for the profile (managed externally)
- `wallet_address` (Text, Unique, Not Null) - Ethereum wallet address
- `name` (Text, Not Null) - Display name of the user
- `email` (Text, Unique, Nullable) - Email address for notifications

**Constraints:**
- Primary key on `id`
- Unique constraint on `wallet_address`
- Unique constraint on `email` (if provided)

#### `events_onchain`
On-chain event data synchronized from smart contracts.

**Columns:**
- `event_id` (Bigint, Primary Key) - Unique event identifier from smart contract
- `vault_address` (Text, Not Null, Unique) - Address of the vault contract for this event
- `organizer_address` (Text, Not Null) - Event organizer's wallet address
- `stake_amount` (Numeric, Not Null) - Required stake amount (PostgreSQL numeric type for precision)
- `max_participant` (Bigint, Not Null) - Maximum number of participants allowed
- `registration_deadline` (Numeric, Not Null) - Registration close time (timestamp as numeric)
- `event_date` (Numeric, Not Null) - Event start time (timestamp as numeric)

**Constraints:**
- Primary key on `event_id`
- Unique constraint on `vault_address` (ensures one vault per event)

#### `events_metadata`
Off-chain event metadata for display purposes.

**Columns:**
- `event_id` (Bigint, Primary Key, Foreign Key) - References `events_onchain.event_id`
- `title` (Text, Not Null) - Event title
- `description` (Text, Nullable) - Detailed event description
- `image_url` (Text, Nullable) - Event banner/thumbnail URL
- `status` (USER-DEFINED, Not Null) - Event status (custom PostgreSQL enum type)

**Status Values:**
The status uses a PostgreSQL user-defined enum type that includes values like:
- `REGISTRATION_OPEN` - Accepting participant registrations
- `REGISTRATION_CLOSED` - No longer accepting registrations
- `LIVE` - Event is currently active
- `SETTLED` - Event completed and rewards distributed
- `VOIDED` - Event cancelled (emergency case)

**Constraints:**
- Primary key on `event_id`
- Foreign key constraint to `events_onchain.event_id`

#### `participant`
Core participant tracking table (note: singular table name in actual schema).

**Columns:**
- `id` (UUID, Primary Key) - Unique participant record identifier (auto-generated)
- `event_id` (Bigint, Not Null, Unique) - References `events_onchain.event_id`
- `user_id` (UUID, Not Null, Unique) - References `profiles.id`
- `is_attend` (Boolean, Not Null, Default: false) - Whether participant attended the event
- `is_claim` (Boolean, Not Null, Default: false) - Whether participant has claimed their rewards

**Important Constraints:**
- **Unique constraint on `event_id`** - Only ONE participant record per event
- **Unique constraint on `user_id`** - Only ONE event participation per user at a time
- Foreign key constraints to `events_onchain` and `profiles`

### Data Relationships

```
profiles (1) ←→ (1) participant ←→ (1) events_onchain
    ↓                                   ↓
                                   events_metadata
```

**Important Relationship Notes:**
- The `participant` table enforces a **1:1 relationship** between events and users
- Each event can only have one participant record
- Each user can only participate in one event at a time
- This design suggests events are structured for single-participant scenarios or specific business logic

### Actual SQL Schema

```sql
-- Core user profiles table
CREATE TABLE public.profiles (
  id uuid NOT NULL,
  wallet_address text NOT NULL UNIQUE,
  name text NOT NULL,
  email text UNIQUE,
  CONSTRAINT profiles_pkey PRIMARY KEY (id)
);

-- On-chain event data
CREATE TABLE public.events_onchain (
  event_id bigint NOT NULL,
  vault_address text NOT NULL UNIQUE,
  organizer_address text NOT NULL,
  stake_amount numeric NOT NULL,
  max_participant bigint NOT NULL,
  registration_deadline numeric NOT NULL,
  event_date numeric NOT NULL,
  CONSTRAINT events_onchain_pkey PRIMARY KEY (event_id)
);

-- Event metadata for display
CREATE TABLE public.events_metadata (
  event_id bigint NOT NULL,
  title text NOT NULL,
  description text,
  image_url text,
  status USER-DEFINED NOT NULL,
  CONSTRAINT events_metadata_pkey PRIMARY KEY (event_id),
  CONSTRAINT events_metadata_event_id_fkey FOREIGN KEY (event_id) REFERENCES public.events_onchain(event_id)
);

-- Participant tracking (1:1 relationship)
CREATE TABLE public.participant (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  event_id bigint NOT NULL UNIQUE,
  user_id uuid NOT NULL UNIQUE,
  is_attend boolean NOT NULL DEFAULT false,
  is_claim boolean NOT NULL DEFAULT false,
  CONSTRAINT participant_pkey PRIMARY KEY (id),
  CONSTRAINT participant_event_id_fkey FOREIGN KEY (event_id) REFERENCES public.events_onchain(event_id),
  CONSTRAINT participant_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.profiles(id)
);
```

### Schema Design Implications

**Unique Design Patterns:**
1. **Single Participant per Event**: The unique constraint on `event_id` in the `participant` table means each event can only have one participant record
2. **Exclusive Event Participation**: The unique constraint on `user_id` means a user cannot participate in multiple events simultaneously
3. **External ID Management**: Profile IDs are managed externally (not auto-generated)
4. **High Precision Numbers**: Using PostgreSQL `numeric` type for financial amounts ensures exact precision
5. **Custom Status Types**: Event status uses a user-defined enum for type safety

**Business Logic Implications:**
- This schema suggests a different event model than typical multi-participant events
- May be designed for one-on-one events, consultations, or exclusive experiences
- The constraints enforce exclusivity and prevent conflicts in participation

### Database Initialization

The application expects this exact schema structure. The table creation order is:

1. `profiles` - User profiles (no dependencies)
2. `events_onchain` - Event data (no dependencies)
3. `events_metadata` - References `events_onchain`
4. `participant` - References both `events_onchain` and `profiles`

### Performance Considerations

Given the unique constraints:
- The schema naturally limits scale due to 1:1 relationships
- Indexes on foreign keys provide good join performance
- Unique constraints provide fast lookups for event and user validation

## 🔧 Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | `postgres://user:password@localhost/atfi_db?sslmode=disable` |
| `PORT` | Server port | `8080` |
| `RPC_URL` | Ethereum RPC URL | `https://base-sepolia-rpc.publicnode.com` |

### CORS Configuration
By default, the API allows requests from:
- `http://localhost:3000`
- `http://localhost:3001`
- `http://localhost:3002`

## 🧪 Testing

### Running Tests
```bash
go test ./...
```

### Running Specific Tests
```bash
go test ./handlers
go test ./models
```

### Test Coverage
```bash
go test -cover ./...
```

## 🐳 Docker Support

### Dockerfile
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o main .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
EXPOSE 8080
CMD ["./main"]
```

### Docker Compose
```yaml
version: '3.8'
services:
  db:
    image: postgres:15
    environment:
      POSTGRES_DB: atfi_db
      POSTGRES_USER: user
      POSTGRES_PASSWORD: password
    ports:
      - "5432:5432"

  api:
    build: .
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://user:password@db:5432/atfi_db?sslmode=disable
    depends_on:
      - db
```

## 🔍 Monitoring & Logging

### Structured Logging
The application uses Go's standard `log` package with structured messages for:
- Database connections
- Ethereum client connections
- API requests and responses
- Error conditions

### Health Endpoints
- `/health` - Basic service health check
- `/api/v1/test-db` - Database connectivity test

## 🛡️ Security

- Input validation using Gin's binding features
- SQL injection prevention with parameterized queries
- CORS configuration for cross-origin requests
- Environment variable protection (not logged)
- Ethereum transaction validation

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🆘 Troubleshooting

### Common Issues

#### Database Connection Failed
```bash
# Check PostgreSQL is running
pg_isready -h localhost -p 5432

# Verify database exists
psql -h localhost -U postgres -l
```

#### Ethereum Client Connection Failed
- Verify RPC URL is accessible
- Check network connectivity
- Ensure Base Sepolia is the correct network

#### Port Already in Use
```bash
# Find process using port 8080
lsof -i :8080

# Kill process
kill -9 <PID>
```

### Getting Help

- Check the application logs for detailed error messages
- Verify all environment variables are set correctly
- Ensure database schema is up to date
- Test API endpoints using a tool like Postman or curl

## 📊 Performance

- Connection pooling with pgx
- Efficient JSON handling with gin
- Optimized database queries
- Minimal memory footprint

Built with ❤️ using Go, Gin, PostgreSQL, and Ethereum