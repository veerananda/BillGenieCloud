.PHONY: build run dev test docker-up docker-down clean help

# Help command
help:
	@echo "Available commands:"
	@echo "  make build          - Build Go binary"
	@echo "  make run            - Run the application"
	@echo "  make dev            - Run with hot reload (requires air)"
	@echo "  make test           - Run tests"
	@echo "  make docker-up      - Start Docker containers"
	@echo "  make docker-down    - Stop Docker containers"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make install-tools  - Install development tools"

# Build binary
build:
	@echo "Building restaurant-api..."
	go build -o restaurant-api cmd/server/main.go
	@echo "✅ Build complete: restaurant-api"

# Run application
run: build
	@echo "Starting server..."
	./restaurant-api

# Development with hot reload
dev:
	@echo "Checking if air is installed..."
	@which air > /dev/null || (echo "Installing air..." && go install github.com/cosmtrek/air@latest)
	@echo "Starting development server with hot reload..."
	air

# Run tests
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Test specific package
test-pkg:
	@read -p "Enter package path: " pkg; \
	go test -v -race $$pkg

# Docker commands
docker-up:
	@echo "Starting Docker containers..."
	docker-compose up -d
	@echo "✅ Containers started"
	@echo "PostgreSQL: localhost:5432"
	@echo "pgAdmin: localhost:5050"

docker-down:
	@echo "Stopping Docker containers..."
	docker-compose down
	@echo "✅ Containers stopped"

docker-logs:
	docker-compose logs -f postgres

# Database migrations
db-migrate:
	@echo "Running database migrations..."
	GORM should auto-migrate on startup

db-seed:
	@echo "This will be implemented in services"

# Code quality
lint:
	@echo "Running golangci-lint..."
	golangci-lint run ./...

fmt:
	@echo "Formatting code..."
	go fmt ./...

vet:
	@echo "Running go vet..."
	go vet ./...

# Install development tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/cosmtrek/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "✅ Tools installed"

# Dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "✅ Dependencies updated"

# Clean build artifacts
clean:
	@echo "Cleaning up..."
	rm -f restaurant-api
	rm -f coverage.out
	go clean -cache -testcache
	@echo "✅ Clean complete"

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t restaurant-api:latest .
	@echo "✅ Docker image built"

# Deploy to Heroku
deploy-heroku:
	git push heroku main

# Version information
version:
	@go version

.DEFAULT_GOAL := help
