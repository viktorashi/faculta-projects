# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy the pre-compiled binary from the host context
COPY web-server .

# Expose server port
EXPOSE 8000

# Run the binary
CMD ["./web-server"]
