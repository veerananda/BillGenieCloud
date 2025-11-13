# Restaurant POS Backend

Production-ready Go backend for multi-device restaurant POS system.

## Features

- ✅ Real-time WebSocket sync (<100ms latency)
- ✅ REST API for orders, inventory, menu management
- ✅ Multi-device coordination
- ✅ JWT authentication with refresh tokens
- ✅ Razorpay payment integration
- ✅ PostgreSQL database with GORM ORM
- ✅ Docker support for local development

## Quick Start

### Prerequisites

- Go 1.21+
- PostgreSQL 14+
- Docker (optional)

### Setup

1. Clone repository and install dependencies:
```bash
cd restaurant-api
go mod download
```

2. Create `.env` file:
```bash
cp .env.example .env
```

3. Start PostgreSQL:
```bash
docker-compose up -d
```

4. Run development server:
```bash
go run cmd/server/main.go
```

Server will start on `http://localhost:3000`

## Project Structure

```
restaurant-api/
├── cmd/
│   └── server/
│       └── main.go              # Entry point
├── internal/
│   ├── config/
│   │   ├── config.go            # Configuration loading
│   │   ├── database.go          # Database setup
│   │   └── server.go            # Server setup
│   ├── models/
│   │   ├── user.go
│   │   ├── restaurant.go
│   │   ├── order.go
│   │   └── ...                  # Other models
│   ├── handlers/
│   │   ├── auth.go
│   │   ├── orders.go
│   │   ├── websocket.go
│   │   └── ...                  # Other handlers
│   ├── services/
│   │   ├── order_service.go
│   │   ├── inventory_service.go
│   │   └── ...                  # Other services
│   ├── middleware/
│   │   ├── auth.go
│   │   └── logger.go
│   └── websocket/
│       ├── hub.go               # WebSocket hub
│       └── client.go            # WebSocket client
├── .env                         # Environment variables
├── .env.example                 # Template
├── docker-compose.yml           # Docker setup
├── Dockerfile                   # Container image
├── Makefile                     # Build commands
└── README.md
```

## API Endpoints

### Authentication
- `POST /api/auth/register` - Register new user
- `POST /api/auth/login` - Login user
- `POST /api/auth/refresh` - Refresh token

### Orders
- `POST /api/orders` - Create order
- `GET /api/orders` - List orders
- `GET /api/orders/:id` - Get order details
- `PUT /api/orders/:id` - Update order

### Inventory
- `GET /api/inventory` - List inventory
- `PUT /api/inventory/:id` - Update inventory

### Menu
- `GET /api/menu` - List menu items
- `POST /api/menu` - Create menu item

## WebSocket Events

### Connection
- `join-restaurant` - Join restaurant room

### Orders
- `order:created` - New order created
- `order:updated` - Order status changed
- `order:completed` - Order completed

### Inventory
- `inventory:updated` - Inventory changed

## Development

### Run with auto-reload
```bash
go install github.com/cosmtrek/air@latest
air
```

### Run tests
```bash
go test ./...
```

### Build binary
```bash
go build -o restaurant-api cmd/server/main.go
```

## Deployment

### Heroku
```bash
git push heroku main
```

### DigitalOcean
```bash
scp restaurant-api user@droplet:/opt/
ssh user@droplet "chmod +x /opt/restaurant-api && /opt/restaurant-api"
```

## Environment Variables

```
DATABASE_URL=postgresql://user:password@localhost:5432/restaurant_db
SERVER_PORT=3000
JWT_SECRET=your-secret-key
RAZORPAY_KEY_ID=your-key-id
RAZORPAY_KEY_SECRET=your-secret
```

## License

MIT
