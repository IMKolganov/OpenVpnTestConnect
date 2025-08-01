# Use official Golang base image
FROM golang:1.24.5 as builder

# Set working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files first (for caching dependencies)
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the application
COPY . .

# Build the application
RUN go build -o vpnchecker

# Use a minimal image for running the application
FROM debian:bookworm-slim

# Install OpenVPN
RUN apt-get update && \
    apt-get install -y openvpn ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Create app directory
WORKDIR /app

# Copy built binary and config files
COPY --from=builder /app/vpnchecker /app/vpnchecker
COPY ./ovpn /app/ovpn

# Set environment variables (you can override these at runtime)
ENV VPN_CONFIG_DIR=/app/ovpn
ENV CHECK_INTERVAL=30m

# Run the binary
CMD ["/app/vpnchecker"]