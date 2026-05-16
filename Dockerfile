FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make bash fuse fuse-dev libseccomp-dev gcc musl-dev

# Build thrive for Linux
WORKDIR /workspace
COPY go.mod go.sum ./
RUN GOTOOLCHAIN=auto go mod download
COPY . .
RUN GOTOOLCHAIN=auto GOOS=linux CGO_ENABLED=1 go build -o bin/thrive ./cmd/thrive

# Runtime image
FROM alpine:3.19

RUN apk add --no-cache \
    fuse \
    libseccomp \
    ca-certificates \
    tzdata \
    && rm -f /etc/apk/repositories

# Create required directories
RUN mkdir -p /var/lib/thrive/images /var/lib/thrive/chunks /var/lib/thrive/secrets /run/thrive/containers /sys/fs/cgroup

# Copy binary from builder
COPY --from=builder /workspace/bin/thrive /usr/local/bin/thrive

ENTRYPOINT ["thrive"]