# store service

The stateless read service for store. Devices call it to `Pull` the
committed log of their scope by cursor and to hold a `Pokes` stream that
hints when to pull again. It only reads the product's Postgres (the
`store_*` tables from [`modules/sql`](../sql)); writes go through each
product server's Push. The contract is [`docs/protocol.md`](../../docs/protocol.md).

## Run

```bash
docker compose up -d --wait          # Postgres 17 on localhost:55433 with modules/sql applied
STORE_DATABASE_URL=postgres://store:store@localhost:55433/store \
STORE_TOKEN_ISSUER=time \
STORE_TOKEN_PUBLIC_KEY_PEM="$(cat time-sync-public.pem)" \
go run ./cmd/store
```

The compose volume keeps data between runs; `docker compose down -v` starts
over (migrations run only on a fresh volume).

```bash
docker build -t skkil-store .        # distroless, non-root, ~30 MB
```

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `STORE_DATABASE_URL` | required | The product's Postgres (pgx connection string) |
| `STORE_LISTEN_ADDR` | `:8080` | HTTP/1.1 and cleartext HTTP/2 (h2c) on one port |
| `STORE_TOKEN_ISSUER` | required | The only accepted `iss`, e.g. `time` |
| `STORE_TOKEN_PUBLIC_KEY_PEM` | | The product's P-256 public key (PKIX PEM) |
| `STORE_TOKEN_JWKS_URL` | | Or a JWKS URL, refreshed in the background; set exactly one |
| `STORE_ALLOWED_ORIGINS` | none | Comma-separated browser origins for CORS |
| `STORE_POLL_INTERVAL` | `5s` | How often the polling fallback reads `store_scope.updated_at` |
| `STORE_POKE_KEEPALIVE` | `25s` | Re-send the current version on idle streams (below proxy idle timeouts) |
| `STORE_MIN_VERSION_WAIT` | `2s` | How long a Pull waits for `min_version` before `unavailable` |
| `STORE_SHUTDOWN_TIMEOUT` | `20s` | Grace period for in-flight requests on SIGTERM |
| `STORE_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`; logs are JSON on stdout |

## Endpoints

| Path | What |
| --- | --- |
| `/skkil.store.v1.StoreService/Pull` | Connect (proto or JSON), gRPC and gRPC-Web |
| `/skkil.store.v1.StoreService/Pokes` | Server stream; the head first, then new versions and keep-alives |
| `/livez` | 200 while the process serves HTTP |
| `/readyz` | 200 when Postgres answers; 503 while draining on shutdown |
| `/metrics` | Prometheus: `store_pull_duration_seconds`, `store_pull_versions_served_total`, `store_pull_resets_total`, `store_poke_streams_active`, `store_pokes_sent_total`, `store_poke_fanout_total`, `store_poke_bus_notifications_total` |

Behaviour the protocol leaves to the service:

- `Version.committed_at` is Unix milliseconds.
- A fresh cursor (epoch 0, version 0, no hash) never resets; epoch 0 with a
  version does.
- `client_id` in a request must equal the token's `cid` (`permission_denied`).
- Errors: bad or missing token `unauthenticated`, `protocol_version` other than
  1 `failed_precondition`, `min_version` not reached `unavailable`.
- Pokes come from Postgres `LISTEN store_poke` plus polling of
  `store_scope.updated_at`, so a lost `store_poke` call delays a poke by at
  most `STORE_POLL_INTERVAL`.

## Develop

```bash
go test -race ./...                  # needs Docker (Testcontainers, postgres:17.11-alpine)
go vet ./... && go tool staticcheck ./...
gofmt -l .
buf generate                         # regenerate gen/ from ../protocol/proto
```

`protoc-gen-go`, `protoc-gen-connect-go` and `staticcheck` are pinned as Go
tools in `go.mod`, so `buf generate` and lint need nothing else installed.

## Layout

```text
cmd/store/            main: config, JSON logging, signals
gen/storev1/          generated protobuf and Connect code (committed)
internal/
  app/                wires every component; Serve and graceful shutdown
  auth/               ES256 sync-token verification, PEM and JWKS keys
  config/             environment variables
  health/             /livez and /readyz
  metrics/            Prometheus collectors and /metrics
  poke/               PokeBus (LISTEN and polling) and the per-scope Hub
  pull/               cursor checks, log reads, Pull
  server/             StoreService handler, HTTP mux, CORS, h2c server
  storetest/          test-only: Postgres container, write-path client, token issuer
test/
  e2e/                whole instances over HTTP, two instances on one database
  writepath/          the SQL write path in modules/sql, tested directly
```
