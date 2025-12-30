# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies (auto-download newer Go if needed)
ENV GOTOOLCHAIN=auto
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-linkmode external -extldflags "-static"' -o /bot ./cmd/bot

# Runtime stage
FROM alpine:3.19

# Install ca-certificates for HTTPS
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /bot /app/bot

# Copy config example
COPY configs/config.example.yaml /app/configs/config.example.yaml

# Create data directory
RUN mkdir -p /app/data

# Expose webhook port
EXPOSE 8080

# Run the bot
CMD ["/app/bot", "-config", "/app/configs/config.yaml"]
