# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o aks-health-monitor .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS calls
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/aks-health-monitor .

# Create directory for ConfigMap mount
RUN mkdir -p /etc/config

# Expose port (if needed for health checks or metrics)
EXPOSE 8080

# Run the application
CMD ["./aks-health-monitor"]
