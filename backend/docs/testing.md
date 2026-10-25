# Testing

## Development checks

Run these commands from the repository root:

```bash
make format
make check
make build
make test-critical
```

`make check` covers gofmt, Go vet, Prettier, ESLint, TypeScript compilation, and the frontend build. It does not run tests. `make build` compiles the backend, load-test runner, and frontend.

`make test-critical` runs selected existing tests. Backend tests use race detection and cover strict command decoding, message validation, FIFO matching, room isolation, stale rooms, full buffers, failed upgrade rollback, concurrent Next/disconnect, shutdown, and connection reservations. Frontend tests cover event parsing, content clearing, pending actions, reconnect limits, timer cleanup, and stale socket callbacks.

Full unit tests are available through `make test`. They also cover configuration, utility routes, quotas, protocol frame limits, slow sockets, input composition, text rendering, and scroll behavior.

## Browser scenario

Install Chromium once from `frontend/`:

```bash
npx playwright install chromium
```

From the repository root:

```bash
make browser
```

Playwright starts the local backend and Vite when needed. The scenario uses three browser contexts for matching, message delivery, Next, Stop, disconnect, dialog focus, plain-text rendering, and mobile viewport overflow. The test server raises handshake limits to allow the repeated connections.

## Load test

Build the backend and start a separate instance with higher loopback admission limits. From `backend/`:

```bash
make build
CHAT_ADDRESS=127.0.0.1:8081 \
CHAT_MAX_CONNECTIONS_PER_IP=500 \
CHAT_HANDSHAKE_BURST=10000 \
CHAT_HANDSHAKES_PER_MINUTE=10000 \
./bin/realtime-chat
```

In another terminal, from the repository root:

```bash
make load LOAD_ARGS="-sockets 100 -duration 30s"
make load LOAD_ARGS="-sockets 250 -duration 30s"
make load LOAD_ARGS="-sockets 500 -duration 10m"
```

The runner spaces queue joins by 10 ms, waits for matching, and sends one timestamp message per visitor every five seconds. It measures time from send to peer receipt, drains pending receipts, closes sockets, and waits six seconds for cleanup. Protocol errors, missing receipts, or missing latency samples fail the run.

Use `-server-pid <pid>` for Linux process CPU and RSS measurements. `-burst` sends five messages per visitor before sustained traffic; `-join-interval 0` submits queue joins without spacing. The runner targets port 8081 by default; use `-url` to change it.

## Recorded load results

These results were recorded on 2026-10-24, before the structural refactor. They describe short synthetic messages on the local machine, with clients and server on the same host.

Environment: Linux x86_64, Intel Core i5-7200U, 2 cores / 4 logical CPUs, 7,292 MiB RAM, Go 1.26.4. Only the backend address, per-IP quota, and handshake limits were overridden. The global connection limit, command buffer, and outbound buffers retained their defaults.

| Sockets / rooms | Duration | Sent / received | Errors | p50 / p95 (ms) | Mean CPU | Peak / final RSS (KiB) |
| --- | --- | --- | --- | --- | --- | --- |
| 100 / 50 | 30s | 500 / 500 | 0 | 0.374 / 0.998 | 0.77% | 19,140 / 19,160 |
| 250 / 125 | 30s | 1,250 / 1,250 | 0 | 0.438 / 1.080 | 1.70% | 24,820 / 24,196 |
| 500 / 250 | 10m | 59,490 / 59,490 | 0 | 0.482 / 2.098 | 5.61% | 34,644 / 34,648 |

Connections, queue entries, and rooms returned to zero after each run. Goroutines returned to the baseline of six. RSS samples were collected once per second; CPU used Linux process counters and assumed `CLK_TCK=100`.

A synchronized five-message burst at 500 sockets exceeded the 256-command buffer: 3,281 successful socket writes produced 1,347 peer receipts, 342 policy closes, and 2,311 error events. The backend cleaned up afterward. These measurements do not establish capacity for sustained maximum-size messages or synchronized bursts.
