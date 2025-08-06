# Build stage
FROM golang:1.24.5-bookworm AS builder

WORKDIR /app

# Copy source code
COPY . .
RUN go mod tidy


# Build the Go binary
RUN go build -o server .

# Runtime stage
FROM debian:bookworm-slim

WORKDIR /app

# Copy built binary from builder
COPY --from=builder /app/server .

# Expose port if your app uses one
EXPOSE 8080

# Run the binary
CMD ["./server"]
