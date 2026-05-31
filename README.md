<div align="center">

# 🖥️ computer-docker

### **A computer for your agent — inside a Docker, remotely.**

*Give your AI a real machine. Files, shell, processes, browsers — behind one clean REST API, jailed in a container, reachable from anywhere.*

<br>

[![CI](https://github.com/renrindonesia/computer-docker/actions/workflows/ci.yml/badge.svg)](https://github.com/renrindonesia/computer-docker/actions/workflows/ci.yml)
[![Docker](https://img.shields.io/badge/Docker-ubuntu%2024.04-2496ED?logo=docker&logoColor=white)](./Dockerfile)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Deps](https://img.shields.io/badge/3rd--party%20deps-0-success)](./go.mod)
[![Tests](https://img.shields.io/badge/tests-39%20passing%20%7C%20race--clean-brightgreen)](#-development)
[![API](https://img.shields.io/badge/API-REST%20%2B%20Swagger-orange)](#-api)
[![License](https://img.shields.io/badge/license-MIT-black)](#-license)

</div>

---

```
        ┌──────────────────────────────────────────────────────┐
        │   your agent                                          │
        │      │  HTTP + ?key=                                  │
        │      ▼                                                │
        │  ┌────────────────────────────────────────────────┐  │
        │  │   computer-docker   (one Go binary, 0 deps)      │  │
        │  │                                                  │  │
        │  │   📁 fs      📜 exec     ⚙️  procs     🧩 ext     │  │
        │  │   read       run         start         browser-  │  │
        │  │   write      stdout      logs          use ...   │  │
        │  │   patch      stderr      stop          install   │  │
        │  │   search     exit        stream        on demand │  │
        │  └────────────────────────────────────────────────┘  │
        │           jailed to  /opt/data   ·  non-root          │
        │                  Ubuntu 24.04 userland                │
        └──────────────────────────────────────────────────────┘
```

---

## ✨ Why

Your agent is brilliant but **homeless** — no disk to keep, no shell to run, no process to babysit, no browser to drive. `computer-docker` hands it a **whole computer** through plain HTTP:

- 🧠 **Stateful** — a real filesystem under `/opt/data`, survives across calls
- 🔌 **One API** — files, shell, long-running processes, installable extensions
- 📦 **Disposable** — it's a container; nuke and respawn in seconds
- 🌍 **Remote-first** — runs on your laptop, a VM, or a cloud box; reach it from anywhere
- 🪶 **Featherweight** — single static Go binary, **zero** third-party Go deps

> Think of it as the **hands and feet** for a brain that only had a mouth.

---

## ⚡ 60-second start

```bash
docker run --rm -p 8080:8080 -e API_KEY=changeme renrindonesia/computer-use:latest
```

```bash
# build it yourself
docker build -t computer-docker . && \
docker run --rm -p 8080:8080 -e API_KEY=changeme computer-docker
```

🎛️ **Swagger UI** → http://localhost:8080/docs

```bash
curl -X POST "localhost:8080/api/v1/exec?key=changeme" \
     -d '{"command":"echo","args":["hello, agent"]}'
# → {"stdout":"hello, agent\n","exit_code":0,"duration_ms":2}
```

---

## 🧭 `/info` — one call, whole machine

```bash
curl "localhost:8080/api/v1/info?key=changeme"
```

```jsonc
{
  "system":     { "hostname": "…", "os": "linux", "arch": "amd64", "num_cpu": 8 },
  "fs_root":    "/opt/data",
  "processes":  [ /* every background proc + state */ ],
  "extensions": [ { "name": "browser-use", "installed": false, … } ],
  "root_files": [ /* top-level listing of /opt/data */ ]
}
```

> One endpoint to answer *"what is this machine, and what's it doing right now?"*

---

## 🧰 API

Base: `/api/v1` · Auth: `?key=` or `X-API-Key` · FS jailed to **`/opt/data`**

<details open>
<summary><b>📁 Filesystem</b></summary>

| Method | Path | What |
|---|---|---|
| `GET` | `/fs/list?path=` | list directory |
| `GET` | `/fs/stat?path=` | metadata |
| `GET` | `/fs/read?path=` | read text |
| `POST` | `/fs/write` | `{path, content}` |
| `POST` | `/fs/mkdir` | `{path}` |
| `DELETE` | `/fs/delete?path=` | recursive remove |
| `POST` | `/fs/move` | `{from, to}` |
| `POST` | `/fs/copy` | `{from, to}` |
| `POST` | `/fs/chmod` | `{path, mode:"0755"}` |
| `POST` | `/fs/patch` | `{path, old, new}` — apply_patch-style unique-block edit |
| `GET` | `/fs/search?path=&glob=&content=` | name glob + content grep |
| `POST` | `/fs/upload?path=` | multipart, binary-safe |
| `GET` | `/fs/download?path=` | raw byte stream |

</details>

<details>
<summary><b>📜 Exec — synchronous shell</b></summary>

```bash
POST /api/v1/exec
{ "command":"ls", "args":["-la"], "dir":"", "stdin":"", "timeout_sec":30 }
→ { "stdout", "stderr", "exit_code", "timed_out", "duration_ms" }
```
Timeout-bounded. For anything long-lived, use **procs** ↓
</details>

<details>
<summary><b>⚙️ Procs — background processes</b></summary>

| Method | Path | What |
|---|---|---|
| `POST` | `/procs` | `{command, args, dir, env}` → starts in background |
| `GET` | `/procs` | list all + state |
| `GET` | `/procs/{id}` | one process |
| `GET` | `/procs/{id}/logs` | last 1000 lines (stdout+stderr, tagged) |
| `POST` | `/procs/{id}/stop` | SIGTERM the process group |
| `DELETE` | `/procs/{id}` | stop + remove |

```bash
# run a dev server, watch its logs, kill it later
ID=$(curl -s -X POST ".../procs?key=$K" \
  -d '{"command":"python3","args":["-m","http.server","9000"]}' | jq -r .id)
curl ".../procs/$ID/logs?key=$K"
curl -X POST ".../procs/$ID/stop?key=$K"
```
</details>

<details>
<summary><b>🧩 Extensions — installable superpowers</b></summary>

Drop-in capabilities, installed at runtime into the container. First-class entry: **[browser-use](https://github.com/browser-use/browser-use)** 🌐 — browser automation for agents.

| Method | Path | What |
|---|---|---|
| `GET` | `/extensions` | catalog + installed status |
| `POST` | `/extensions/{name}/install` | install **in the background** → returns a proc id |

```bash
# kick off install, then tail the pip logs live
P=$(curl -s -X POST ".../extensions/browser-use/install?key=$K" | jq -r .process.id)
curl ".../procs/$P/logs?key=$K"
```
</details>

---

## 🤖 Connect your agent (MCP)

The sandbox speaks **Model Context Protocol** over Streamable HTTP at **`/mcp`** — point any MCP client at it and your agent gets all the tools natively, no hand-written HTTP.

**Endpoint:** `http://HOST:8080/mcp` · **Auth:** `X-API-Key` header (or `?key=`)

```jsonc
// MCP client config
{
  "mcpServers": {
    "computer-docker": {
      "url": "http://localhost:8080/mcp",
      "headers": { "X-API-Key": "changeme" }
    }
  }
}
```

**15 tools** exposed:

| Group | Tools |
|---|---|
| 📁 files | `fs_list` · `fs_read` · `fs_write` · `fs_edit` · `fs_search` · `fs_move` · `fs_remove` |
| 📜 shell | `exec` |
| ⚙️ procs | `proc_start` · `proc_list` · `proc_logs` · `proc_stop` |
| 🧩 system | `ext_list` · `ext_install` · `info` |

> Same services, same API key as the REST API — REST stays available for non-MCP clients.

---

## ⚙️ Config

| Var | Default | Meaning |
|---|---|---|
| `ADDR` | `:8080` | listen address |
| `API_KEY` | _(empty)_ | auth key — empty disables auth |
| `FS_ROOT` | `/opt/data` | filesystem jail root |
| `EXEC_TIMEOUT_SEC` | `30` | default exec timeout |
| `EXEC_MAX_TIMEOUT_SEC` | `300` | hard cap |

Loaded from `.env`, then real env wins. Copy `.env.example` → `.env`.

---

## 🛠️ Development

```bash
make test         # go test ./...
make test-race    # race detector
make cover        # coverage summary
make docker       # build image
```

Run the suite **isolated in Docker**:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.25-bookworm \
  sh -c "go vet ./... && go test -race ./..."
```

```
cmd/api              entrypoint · wiring · graceful shutdown
internal/config      .env + env loader
internal/fsapi       path-jailed filesystem (+ ops, patch, search)
internal/execapi     synchronous command runner
internal/procapi     background process manager + log ring buffer
internal/extapi      installable extension catalog (browser-use)
internal/handler     HTTP handlers + route table + /info
internal/middleware  logging · panic recovery · API-key auth
internal/docs        embedded OpenAPI + Swagger UI
```

---

## 🚀 CI / CD

| Workflow | Trigger | Does |
|---|---|---|
| **CI** (`.github/workflows/ci.yml`) | push / PR to `main` | gofmt check · `go vet` · `go test -race` + coverage · build · Docker build + health-check smoke |
| **Release** (`.github/workflows/release.yml`) | push tag `v*` | multi-arch (`amd64`+`arm64`) build, push to **Docker Hub** + **GHCR** |

**Cut a release:**

```bash
git tag v0.1.0 && git push origin v0.1.0
```

**Required repo secrets** (Settings → Secrets → Actions):

| Secret | For |
|---|---|
| `DOCKERHUB_USERNAME` | Docker Hub login |
| `DOCKERHUB_TOKEN` | Docker Hub access token ([create here](https://hub.docker.com/settings/security)) |

> GHCR uses the built-in `GITHUB_TOKEN` — no setup needed.

---

## 🔐 Security — read this

> `exec` + `procs` run **arbitrary commands**. `fs` touches disk under `/opt/data`.
> **This API is equivalent to shell access to the container.**

- 🚧 Bind to **localhost / trusted network only**. Don't expose publicly without real authz + isolation.
- 🔑 The shared API key is a coarse gate, **not** isolation. One container per tenant.
- 👤 Runs as non-root (`appuser`); fs jailed to `/opt/data` — but a command can still burn CPU/disk.
- 🧱 In production add limits: `docker run --cpus 2 --memory 2g --pids-limit 512 …`

---

## 📄 License

MIT — go build something that thinks **and** acts.

<div align="center">
<sub>🖥️ a computer for your agent · inside a Docker · remotely</sub>
</div>
