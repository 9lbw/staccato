# Build stage
FROM golang:1.22.4-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -ldflags="-s -w" -o spotigo ./cmd/spotigo

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates sqlite tzdata

# Create non-root user
RUN addgroup -g 1001 -S spotigo && \
    adduser -u 1001 -S spotigo -G spotigo

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/spotigo .

# Copy static files
COPY static/ static/

# Create a basic config file (the app will generate one if missing)
RUN echo "# Spotigo will generate a proper config file on first run" > config.toml

# Create directories
RUN mkdir -p music data && \
    chown -R spotigo:spotigo /app

# Switch to non-root user
USER spotigo

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ || exit 1

# Run the application
CMD ["./spotigo"]
