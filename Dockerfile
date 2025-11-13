FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY internal/ internal/

# Build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o restaurant-api cmd/server/main.go

# Final stage
FROM alpine:latest

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/restaurant-api .

# Copy .env file (optional)
COPY .env .

# Expose port
EXPOSE 3000

# Run application
CMD ["./restaurant-api"]
