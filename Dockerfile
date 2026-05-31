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
# Polyglot sandbox: ships Python, Node.js, Go, Java, Kotlin, PHP and Ruby so the
# exec/procs APIs can run code in any of them out of the box. This makes the
# image large (~2GB); trim the languages you don't need to slim it down.
FROM ubuntu:24.04

ARG NODE_MAJOR=22
ARG KOTLIN_VERSION=2.1.10
ARG DEBIAN_FRONTEND=noninteractive

# Base tools + apt-provided languages (Python 3.12, Java 21, PHP 8.3, Ruby 3.x).
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        wget \
        git \
        unzip \
        coreutils \
        build-essential \
        python3 \
        python3-venv \
        python3-pip \
        openjdk-21-jdk-headless \
        php-cli \
        ruby-full \
    && rm -rf /var/lib/apt/lists/*

# Node.js (NodeSource) — latest LTS major.
RUN curl -fsSL "https://deb.nodesource.com/setup_${NODE_MAJOR}.x" | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

# Go — copied from the build stage (exact toolchain used to build the binary).
COPY --from=build /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}" \
    GOPATH=/opt/go
ENV PATH="${GOPATH}/bin:${PATH}"

# Kotlin compiler (needs the JDK above).
RUN curl -fsSL -o /tmp/kotlin.zip \
        "https://github.com/JetBrains/kotlin/releases/download/v${KOTLIN_VERSION}/kotlin-compiler-${KOTLIN_VERSION}.zip" \
    && unzip -q /tmp/kotlin.zip -d /opt \
    && rm /tmp/kotlin.zip
ENV PATH="/opt/kotlinc/bin:${PATH}"

# Shared virtualenv on PATH so `pip install` works without --break-system-packages.
ENV VIRTUAL_ENV=/opt/venv
RUN python3 -m venv "$VIRTUAL_ENV"
ENV PATH="$VIRTUAL_ENV/bin:$PATH"

# Non-root user; owns the venv, GOPATH and the fs jail at /opt/data.
RUN useradd --create-home --uid 10001 appuser \
    && mkdir -p /opt/data /opt/go \
    && chown -R appuser:appuser /opt/data /opt/venv /opt/go

WORKDIR /app
COPY --from=build /out/api /usr/local/bin/api

ENV ADDR=:8080 \
    FS_ROOT=/opt/data \
    EXEC_TIMEOUT_SEC=30 \
    EXEC_MAX_TIMEOUT_SEC=300

USER appuser
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -fsS http://localhost:8080/healthz || exit 1

ENTRYPOINT ["api"]
