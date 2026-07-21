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

# Base tools + apt-provided languages (Python 3.12, Java 21, PHP 8.3, Ruby 3.x)
# + CLI power tools agents reach for (ripgrep, fd, fzf, jq, bat, tree).
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
        ripgrep \
        fd-find \
        fzf \
        jq \
        bat \
        tree \
        xvfb \
        x11vnc \
        fluxbox \
        novnc \
        websockify \
        x11-xserver-utils \
        xterm \
    && rm -rf /var/lib/apt/lists/* \
    && ln -s "$(command -v fdfind)" /usr/local/bin/fd \
    && ln -s "$(command -v batcat)" /usr/local/bin/bat

# gh (GitHub CLI) from the official apt repo, plus yq as a static binary.
RUN install -m 0755 -d /etc/apt/keyrings \
    && curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
        -o /etc/apt/keyrings/githubcli-archive-keyring.gpg \
    && chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
        > /etc/apt/sources.list.d/github-cli.list \
    && apt-get update && apt-get install -y --no-install-recommends gh \
    && rm -rf /var/lib/apt/lists/* \
    && curl -fsSL "https://github.com/mikefarah/yq/releases/latest/download/yq_linux_$(dpkg --print-architecture)" \
        -o /usr/local/bin/yq \
    && chmod +x /usr/local/bin/yq

# Node.js (NodeSource) — latest LTS major.
RUN curl -fsSL "https://deb.nodesource.com/setup_${NODE_MAJOR}.x" | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

# AI coding agents (Node CLIs): Claude Code + OpenAI Codex.
RUN npm install -g @anthropic-ai/claude-code @openai/codex \
    && npm cache clean --force

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
    && chown -R appuser:appuser /opt/data /opt/venv /opt/go \
    && chmod -R u+rwX /opt/data

# browser-use + Playwright Chromium. Installed as root because the browser needs
# system libs (libX11, libgbm, libnss3, ...) pulled in via `playwright
# install-deps`, which runs apt. Bakes the browser-use extension in so
# /api/v1/extensions reports it installed and the exec/procs APIs can drive a
# real headless Chromium out of the box. Browsers live in a shared path owned by
# appuser. Adds ~1GB; drop this block if you don't need browser automation.
ENV PLAYWRIGHT_BROWSERS_PATH=/opt/ms-playwright
RUN pip install --no-cache-dir browser-use playwright \
    && python3 -m playwright install-deps chromium \
    && python3 -m playwright install chromium \
    && rm -rf /var/lib/apt/lists/* \
    && chown -R appuser:appuser /opt/venv /opt/ms-playwright

WORKDIR /app
COPY --from=build /out/api /usr/local/bin/api
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

ENV ADDR=:8080 \
    FS_ROOT=/opt/data \
    EXEC_TIMEOUT_SEC=30 \
    EXEC_MAX_TIMEOUT_SEC=300 \
    DISPLAY=:99

# Runs as root so bind-mounted volumes at /opt/data are accessible regardless of
# host ownership (appuser hit permission errors on mounted volumes).

# Default git identity + initialize a repo in the data jail so agents (claude
# code, codex) can commit/diff out of the box. NOTE: if /opt/data is replaced by
# a volume mount at runtime, re-run `git init` inside it.
RUN git config --global user.name "computer-docker" \
    && git config --global user.email "agent@computer-docker.local" \
    && git config --global init.defaultBranch main \
    && git config --global --add safe.directory /opt/data \
    && git init -q /opt/data

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -fsS http://localhost:8080/healthz || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
