FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates bash

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -a -installsuffix cgo -o api-uploadv2 .

# Final stage
FROM alpine:latest

# Install ca-certificates
RUN apk --no-cache add ca-certificates tzdata bash

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/api-uploadv2 .

# Create .env file with default values
ENV MONGOURI=mongodb://mongodb:27017/EyeCDB
ENV DB_HOST=postgresql
ENV DB_USER=protecht_user
ENV DB_PASSWORD=your_secure_password
ENV DB_NAME=protecht
ENV DB_PORT=5432
ENV REDISURL=redis://redis:6379
ENV MEDIADIR=/app/media/
ENV STREAMDIR=/app/streams/

# Expose port
EXPOSE 30970

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:30970/healthz || exit 1

# Run the application
CMD ["./api-uploadv2"]
