# Production Dockerfile for confd
# Used by GoReleaser - receives pre-built binary

FROM alpine:3.21

# Create non-root user
RUN addgroup -g 1000 confd && \
    adduser -u 1000 -G confd -s /bin/sh -D confd

# Install runtime dependencies
# - ca-certificates: TLS backends (Vault, Consul, etcd)
# - tzdata: timezone support for datetime template function
RUN apk add --no-cache ca-certificates tzdata

# Copy pre-built binary from GoReleaser
COPY confd /usr/local/bin/confd

# Create configuration directories
RUN mkdir -p /etc/confd/conf.d /etc/confd/templates && \
    chown -R confd:confd /etc/confd

# Run as non-root user
USER confd:confd

# Set working directory
WORKDIR /etc/confd

# Default entrypoint
ENTRYPOINT ["/usr/local/bin/confd"]
