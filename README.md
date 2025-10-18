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

### Tables

#### `profiles`
- `id` (UUID, Primary Key)
- `wallet_address` (String, Unique)
- `name` (String)
- `email` (String)
- `balance` (String)
- `created_at` (Timestamp)
- `updated_at` (Timestamp)

#### `events_onchain`
- `event_id` (Integer, Primary Key)
- `vault_address` (String)
- `organizer_address` (String)
- `stake_amount` (String)
- `max_participant` (Integer)
- `registration_deadline` (Integer, Unix Timestamp)
- `event_date` (Integer, Unix Timestamp)

#### `events_metadata`
- `event_id` (Integer, Primary Key)
- `title` (String)
- `description` (String)
- `image_url` (String)
- `status` (String)

#### `checkins`
- `id` (UUID, Primary Key)
- `event_id` (Integer)
- `user_address` (String)
- `qr_data` (String)
- `checked_in_at` (Timestamp)
- `is_validated` (Boolean)
- `validated_at` (Timestamp)
- `validated_by` (String)

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