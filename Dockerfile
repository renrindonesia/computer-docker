# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.25-bookworm AS build
WORKDIR /src

# Cache deps first.
COPY go.mod ./
# COPY go.sum ./   # add when third-party deps exist
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# ---- runtime stage ----
FROM ubuntu:24.04

# exec API needs a real userland; add common tools + CA certs.
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        git \
        coreutils \
    && rm -rf /var/lib/apt/lists/*

# Non-root user owns the fs jail.
RUN useradd --create-home --uid 10001 appuser
WORKDIR /app
RUN mkdir -p /app/data && chown -R appuser:appuser /app

COPY --from=build /out/api /usr/local/bin/api

ENV ADDR=:8080 \
    FS_ROOT=/app/data \
    EXEC_TIMEOUT_SEC=30 \
    EXEC_MAX_TIMEOUT_SEC=300

USER appuser
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -fsS http://localhost:8080/healthz || exit 1

ENTRYPOINT ["api"]
