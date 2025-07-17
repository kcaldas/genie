# GoReleaser provides pre-built binaries, so we only need the runtime stage
FROM alpine:3.20

# Install ca-certificates for HTTPS requests and git for potential operations
RUN apk --no-cache add ca-certificates git

# Create non-root user for security
RUN addgroup -g 1001 -S genie && \
    adduser -u 1001 -S genie -G genie

# Set working directory
WORKDIR /home/genie

# Copy binary (GoReleaser will place the pre-built binary here)
COPY genie /usr/local/bin/genie

# Create directories for genie configuration and history
RUN mkdir -p /home/genie/.genie && \
    chown -R genie:genie /home/genie

# Switch to non-root user
USER genie

# Create a volume for persistent data (optional)
VOLUME ["/home/genie/.genie"]

# Expose no ports (CLI tool)
# No EXPOSE directive needed

# Set default command
ENTRYPOINT ["genie"]
CMD ["--help"]

# Labels for better maintainability
LABEL org.opencontainers.image.title="Genie" \
      org.opencontainers.image.description="AI-powered coding assistant with CLI and TUI interfaces" \
      org.opencontainers.image.vendor="kcaldas" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.source="https://github.com/kcaldas/genie"