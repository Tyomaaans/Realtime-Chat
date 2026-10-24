# Testing

## Local checks

Run `make check` at the repository root. It checks formatting, vet, backend behavior, race detection, compilation, frontend lint/typechecking, frontend tests, and the frontend build. `make browser` runs the separate browser scenario after Chromium is installed with `npx playwright install chromium` from `frontend/`. Tests leave tracked screenshots alone. To regenerate the documentation images deliberately, run `CHAT_CAPTURE_SCREENSHOTS=1 npm run test:browser` from `frontend/`.

Go tests use the standard test library, in-memory hub fixtures, `httptest`, and real Gorilla clients. Cases cover FIFO matching, odd counts, duplicate joins, cancellation, queue expiry, Stop/Next, old rooms, forbidden rematches, cross-room isolation, disconnect, buffer overflow, Unicode/byte limits, malformed and duplicate JSON fields, binary/UTF-8/frame limits, origins, failed upgrades, quotas, handshake limits, limiter expiry, forwarded-header spoofing, slow sockets, pong timeout, repeated cleanup, and shutdown with live sockets. Race tests include simultaneous Next and disconnect.

Vitest and React Testing Library cover event parsing, bounded transcripts, room guards, content clearing, StrictMode cleanup, stale callbacks, reconnect exhaustion, manual retry, typing expiry, rate-limit expiry, IME/keyboard behavior, Unicode limits, text rendering, and scroll position. Playwright uses three browser contexts against the real local servers for Start, message relay, Next, a new partner, Stop, partner disconnection, dialog keyboard focus, text containing HTML, and 320px layout overflow. Screenshots in `docs/screenshots/` come from this scenario.

Color calculations for the actual theme give muted text 4.92:1 on white and 4.61:1 on canvas; control borders 3.54:1 on white and 3.32:1 on canvas; white on brand 7.87:1; and danger text on white 6.57:1. Browser screenshots were inspected at desktop and mobile widths. Mobile testing uses a desktop Chromium viewport; real mobile keyboard and screen-reader behavior still depend on the device/browser.

## Reproduce load tests

Use a separate terminal and an explicitly configured backend. These overrides exist only for loopback tests; the normal five-connection IP limit and ten-handshake burst would correctly reject a 500-socket test.

```sh
cd backend
go build -o /tmp/realtime-chat-server .
CHAT_ADDRESS=127.0.0.1:8081 \
CHAT_MAX_CONNECTIONS_PER_IP=500 \
CHAT_HANDSHAKE_BURST=10000 \
CHAT_HANDSHAKES_PER_MINUTE=10000 \
/tmp/realtime-chat-server
```

In another terminal, from `backend/`, replace `<pid>` with that backend process's PID:

```sh
go run ../scripts/loadtest/main.go -sockets 100 -duration 30s -server-pid <pid>
go run ../scripts/loadtest/main.go -sockets 250 -duration 30s -server-pid <pid>
go run ../scripts/loadtest/main.go -sockets 500 -duration 10m -server-pid <pid>
```

The runner opens sockets, spaces queue joins by 10 ms (about 100 joins/second), waits for matching, sends one timestamp message per visitor every five seconds, and measures send-to-peer-receipt latency using the local clock. Sends are staggered across that interval. It drains pending receipts, closes all sockets, and waits six seconds for cleanup measurements. Any protocol/read/write error, missing receipt, or missing latency sample fails the run. These are short synthetic messages; this profile does not establish capacity for continuous maximum-size messages. `-join-interval 0` deliberately tests admission under a simultaneous join burst; it can close clients when the fixed command buffer fills.

Use `-burst` for five messages per visitor before sustained traffic. Repeat short runs for connection churn and memory trends. Slow socket behavior also has a deterministic backend integration test with small TCP buffers and a short write deadline.

The JSON output includes counts, p50/p95 relay latency, CPU, peak RSS, and RSS after teardown. On Linux, `-server-pid` samples `/proc/<pid>/stat` and `status`; CPU assumes `CLK_TCK=100` and is expressed as a percentage of one logical CPU. RSS is sampled once per second and can miss shorter peaks. Without a process PID those process metrics are unavailable. Backend status logs separately record connections, rooms, queue size, heap allocation, and goroutines every five seconds. Inspect those logs after each cycle; RSS need not immediately drop because the Go allocator can retain pages.

## Recorded environment

Local Linux x86_64, kernel 7.0.0-34-generic, Intel Core i5-7200U at 2.50 GHz (2 cores, 4 logical CPUs), 7,292 MiB RAM and 8,191 MiB swap. Toolchain: Go 1.26.4, Node 22.22.1, npm 9.2.0, Playwright 1.64.1, Chromium 156.0.8078.4. Tests ran on the same machine as the backend and load clients, alongside other desktop processes. This is not an isolated benchmark host.

The backend retained its normal 500 global connections, 500 queue entries, 256 hub commands, 32 outbound events per connection, 8 KiB frame limit, command/message limits, and heartbeat/write deadlines. Only the address and the three loopback admission settings shown above were overridden.

## Recorded results

Recorded on 2026-10-24. All backend formatting, vet, behavior, race, and build checks passed. Frontend installation from the lockfile, lint, typecheck, 29 Vitest tests, build, and the three-context Playwright scenario passed. The browser scenario reported no uncaught page errors, and its default run left tracked screenshots unchanged.

The final backend binary produced these sustained-traffic results. All queue joins used the documented 10 ms spacing, and the traffic window started after matching. CPU and RSS refer to the backend process; the load client ran separately on the same host.

| Sockets / rooms | Duration | Sent / received | Errors | p50 / p95 (ms) | Mean CPU | Sampled peak / final RSS (KiB) | Active goroutines |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 100 / 50 | 30 s | 500 / 500 | 0 | 0.374 / 0.998 | 0.77% | 19,140 / 19,160 | 206 |
| 250 / 125 | 30 s | 1,250 / 1,250 | 0 | 0.438 / 1.080 | 1.70% | 24,820 / 24,196 | 506 |
| 500 / 250 | 10 min | 59,490 / 59,490 | 0 | 0.482 / 2.098 | 5.61% | 34,644 / 34,648 | 1,006 |

The 500-socket profile met the local p95 target of 300 ms. After every run, connections, queue entries, and rooms returned to zero, and goroutines returned to the baseline of six. Sampled heap allocation during the 500-socket run peaked at 10,790,352 bytes. This supports the documented local traffic profile; maximum-size message traffic and public-network capacity were not measured.

A 100-socket burst run sent five messages per visitor immediately, followed by the normal five-second interval for 20 seconds. All 800 writes reached the peer, with no errors or policy closes; p50/p95 was 10.080/19.900 ms. Mean backend CPU was 0.75%, sampled peak RSS 21,772 KiB, and final RSS 21,264 KiB.

The same synchronized burst at 500 sockets exceeded the command buffer. That stress run failed: 3,281 successful socket writes produced 1,347 peer receipts, 342 policy closes (1008), and 2,311 error events. Its received-message p95 of 71.586 ms does not describe successful delivery for the missing messages. Error totals include read/write failures, server errors, and departure notices; they are not a count of distinct visitors. The backend cleaned up to zero sessions and six goroutines afterward. The default 256-command buffer was retained, so 500 simultaneous visitors must not be advertised as a guarantee for synchronized bursts.

One earlier unpaced 500-join attempt also timed out during matching; status logs showed 471 connections, 235 rooms, and one waiting visitor. A repeat with no join spacing completed, with 501/501 peer receipts over 10 seconds and no policy closes. Admission under a simultaneous burst depends on timing and available CPU. The reproducible steady profile therefore uses an explicit ramp rather than silently changing server limits.

Three additional 100-socket cycles used a one-second message interval for two seconds, with the runner's six-second teardown period:

| Cycle | Sent / received | Errors | p95 (ms) | RSS after teardown (KiB) |
| --- | --- | --- | --- | --- |
| 1 | 100 / 100 | 0 | 1.364 | 20,616 |
| 2 | 101 / 101 | 0 | 0.501 | 19,608 |
| 3 | 100 / 100 | 0 | 1.179 | 19,804 |

Every cycle returned to zero connections, rooms, and waiting visitors, with six goroutines. RSS stayed within this range across the three cycles. Slow-consumer failure is separately covered by the bounded-buffer unit test and a real TCP integration test that constrains socket buffers and write time.
