# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy dependency manifests
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application source code
COPY . .

# Build the statically compiled binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o web-server .

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/web-server .

# Expose server port
EXPOSE 8000

# Run the binary
CMD ["./web-server"]
