# MinZ Compiler Docker Image
# Multi-stage build for minimal final image

# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git nodejs npm python3 make g++

# Install tree-sitter CLI
RUN npm install -g tree-sitter-cli

# Copy source
WORKDIR /build
COPY . .

# Generate parser
RUN tree-sitter generate

# Build compiler
WORKDIR /build/minzc
RUN go mod download
RUN go build -o minzc cmd/minzc/main.go

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache libstdc++

# Create minz user
RUN adduser -D -s /bin/sh minz

# Copy compiler and libraries
COPY --from=builder /build/minzc/minzc /usr/local/bin/
COPY --from=builder /build/stdlib /usr/local/lib/minz/stdlib
COPY --from=builder /build/src /usr/local/share/minz/tree-sitter-minz/src
COPY --from=builder /build/grammar.js /usr/local/share/minz/tree-sitter-minz/

# Set up environment
ENV MINZ_STDLIB_PATH=/usr/local/lib/minz/stdlib
ENV PATH=/usr/local/bin:$PATH

# Create working directory
WORKDIR /workspace
RUN chown minz:minz /workspace

# Switch to non-root user
USER minz

# Default command
CMD ["minzc", "--help"]

# Example usage:
# docker build -t minz:latest -f build/Dockerfile .
# docker run -v $(pwd):/workspace minz:latest minzc program.minz -O --enable-smc