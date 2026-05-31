# computer-docker

A small, dependency-free **REST API for a headless dev sandbox** — filesystem access, command execution, and background process management — packaged to run in a Docker container. Built for driving a "computer" from an agent or remote client.

> ⚠️ This API runs arbitrary commands and reads/writes disk inside its container. **Run it on localhost or a trusted network only.** See [Security](#security).

---

## Features

- **Filesystem API** — list, stat, read, write, mkdir, delete, move, copy, chmod, **search** (glob + grep), **patch** (apply_patch-style edits), binary **upload/download**. All paths jailed to a root directory.
- **Exec API** — run a command synchronously, capture stdout/stderr/exit code, timeout-bounded.
- **Process API** — start long-running background processes, stream their logs, stop/remove them.
- **API-key auth** — single shared key via `?key=` or `X-API-Key`, configured through `.env`.
- **Swagger UI** — interactive docs at `/docs`.
- **Zero third-party Go deps** — stdlib `net/http` mux + `log/slog`.
- **Ubuntu runtime** — real userland so exec/process commands have the tools they expect.

---

## Quick start

### Docker

```bash
docker build -t computer-docker .
docker run --rm -p 8080:8080 -e API_KEY=changeme computer-docker
```

### Local

```bash
cp .env.example .env      # set API_KEY
go run ./cmd/api
```

Open **http://localhost:8080/docs** for Swagger UI.

---

## Configuration (`.env`)

| Var | Default | Meaning |
|-----|---------|---------|
| `ADDR` | `:8080` | Listen address |
| `API_KEY` | _(empty)_ | Auth key. Empty disables auth |
| `FS_ROOT` | `./data` | Filesystem jail root |
| `EXEC_TIMEOUT_SEC` | `30` | Default exec timeout |
| `EXEC_MAX_TIMEOUT_SEC` | `300` | Hard cap on exec timeout |

`.env` is loaded at startup; real environment variables take precedence.

---

## Auth

When `API_KEY` is set, every `/api/v1/*` route requires the key:

- query param: `?key=YOURKEY`
- header: `X-API-Key: YOURKEY`

Public routes: `/healthz`, `/docs`, `/openapi.json`.

---

## API reference

Base path: `/api/v1`. All `fs` paths are relative to `FS_ROOT`; traversal outside it is clamped/rejected.

### Filesystem

| Method | Path | Body / Query | Notes |
|--------|------|--------------|-------|
| GET | `/fs/list` | `?path=` | directory entries |
| GET | `/fs/stat` | `?path=` | file/dir metadata |
| GET | `/fs/read` | `?path=` | text contents (JSON) |
| POST | `/fs/write` | `{path, content}` | create/overwrite text |
| POST | `/fs/mkdir` | `{path}` | mkdir -p |
| DELETE | `/fs/delete` | `?path=` | recursive |
| POST | `/fs/move` | `{from, to}` | move/rename |
| POST | `/fs/copy` | `{from, to}` | copy file |
| POST | `/fs/chmod` | `{path, mode}` | mode is octal, e.g. `"0755"` |
| POST | `/fs/patch` | `{path, old, new}` | replace unique block (apply_patch-style) |
| GET | `/fs/search` | `?path=&glob=&content=&limit=` | name glob + content grep |
| POST | `/fs/upload` | multipart, field `file`, `?path=` dir | binary-safe upload |
| GET | `/fs/download` | `?path=` | raw byte stream |

### Exec (synchronous)

```
POST /api/v1/exec
{"command":"ls","args":["-la"],"dir":"","stdin":"","timeout_sec":30}
→ {"stdout","stderr","exit_code","timed_out","duration_ms"}
```

### Processes (background)

| Method | Path | Notes |
|--------|------|-------|
| POST | `/procs` | `{command, args, dir, env}` → `201` with process snapshot |
| GET | `/procs` | list all |
| GET | `/procs/{id}` | one process |
| GET | `/procs/{id}/logs` | last 1000 lines (stdout+stderr, tagged) |
| POST | `/procs/{id}/stop` | SIGTERM the process group |
| DELETE | `/procs/{id}` | stop + remove |

---

## Examples

```bash
KEY=changeme
BASE=http://localhost:8080/api/v1

# write + read
curl -X POST "$BASE/fs/write?key=$KEY" -d '{"path":"hello.txt","content":"hi"}'
curl "$BASE/fs/read?path=hello.txt&key=$KEY"

# apply_patch-style edit
curl -X POST "$BASE/fs/patch?key=$KEY" \
  -d '{"path":"hello.txt","old":"hi","new":"hello world"}'

# search
curl "$BASE/fs/search?path=/&glob=*.txt&content=hello&key=$KEY"

# upload / download
curl -X POST "$BASE/fs/upload?path=/&key=$KEY" -F "file=@./local.bin"
curl "$BASE/fs/download?path=local.bin&key=$KEY" -o got.bin

# synchronous exec
curl -X POST "$BASE/exec?key=$KEY" -d '{"command":"echo","args":["hello"]}'

# background process + logs
PID=$(curl -s -X POST "$BASE/procs?key=$KEY" \
  -d '{"command":"sh","args":["-c","while true; do date; sleep 1; done"]}' | jq -r .id)
curl "$BASE/procs/$PID/logs?key=$KEY"
curl -X POST "$BASE/procs/$PID/stop?key=$KEY"
```

---

## Development

```bash
make build        # build binary to bin/api
make run          # go run
make test         # go test ./...
make test-race    # race detector
make cover        # coverage summary
make docker       # build image
```

### Run tests in Docker (isolated)

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.25-bookworm \
  sh -c "go vet ./... && go test -race ./..."
```

---

## Project layout

```
cmd/api              entrypoint, wiring, graceful shutdown
internal/config      .env + env loader
internal/fsapi       path-jailed filesystem service (+ ops, patch, search)
internal/execapi     synchronous command runner with timeout
internal/procapi     background process manager + log ring buffer
internal/handler     HTTP handlers + route table
internal/middleware  logging, panic recovery, API-key auth
internal/docs        embedded OpenAPI spec + Swagger UI
```

---

## Security

- **`exec` and `procs` run arbitrary commands; `fs` touches disk under `FS_ROOT`.** Treat the API as equivalent to shell access to the container.
- Bind to **localhost or a trusted network only**. Do not expose publicly without a real authz layer + network isolation.
- The single shared API key is a coarse gate, not a substitute for isolation. Prefer one container per tenant.
- The container runs as a non-root user (`appuser`) and the filesystem is jailed to `FS_ROOT`, but a determined command can still consume CPU/disk. Add resource limits (`docker run --cpus --memory --pids-limit`) in production.

---

## License

MIT
