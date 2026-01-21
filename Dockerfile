# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY main.go ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o nginx-log-viewer .

# Final stage
FROM alpine:latest

WORKDIR /app

# Install basic certificates for HTTPS if needed
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /app/nginx-log-viewer .
# Copy static files (index.html)
COPY index.html .

# Expose port
EXPOSE 58080

# Run
CMD ["./nginx-log-viewer"]
