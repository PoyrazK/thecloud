# Build stage
FROM --platform=$BUILDPLATFORM golang:1.25.0-alpine AS builder

ARG TARGETARCH
ARG TARGETOS

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build binary
# CGO_ENABLED=0 for static binary
RUN CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" \
    go build -o /bin/api ./cmd/api

# Production stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/api /app/api
COPY --from=builder /app/internal/repositories/postgres/migrations /app/migrations
# Copy docs for swagger if needed
COPY --from=builder /app/docs /app/docs
# Copy K8s templates for cluster provisioning
COPY --from=builder /app/internal/repositories/k8s/templates /app/internal/repositories/k8s/templates

# Create data directory
RUN mkdir -p /app/thecloud-data

# Expose API port
EXPOSE 8080

CMD ["/app/api"]
