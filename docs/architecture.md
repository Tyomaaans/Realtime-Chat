# Architecture

## Ownership

`main.go` loads and validates configuration, creates a zerolog logger, wires dependencies, starts the hub and HTTP listener, and handles termination. The backend uses manual constructors. `ChatService` is an interface backed by the private `chatService`; `Hub` is concrete because no substitute matchmaking implementation is needed.

`ChatHandler` checks Origin, reserves global/IP capacity atomically, creates a cryptographic connection ID, upgrades the connection, and registers it with the hub. Failed upgrades and failed registration release their reservation. Successful connections keep their quota until both pumps exit, including sockets that are still closing.

`client.go` owns socket I/O and per-connection rate buckets. The read pump sets frame/read limits, processes pong responses, parses commands, and submits validated commands. The write pump sends events and ping/pong/close control frames with deadlines. Gorilla can also send a protocol close itself when its read limit is exceeded. The hub never writes to a socket.

Socket ownership follows the [Gorilla WebSocket concurrency contract](https://pkg.go.dev/github.com/gorilla/websocket#hdr-Concurrency). Proxy trust is configured explicitly using [Gin's trusted-proxy API](https://gin-gonic.com/en/docs/server-config/trusted-proxies/).

`ChatService` validates message rules before submission. The hub checks state, room ID, and membership before every room mutation or relay. Public error mapping stays in the chat transport package; internal details are not returned to browsers.

## Hub state

One goroutine owns the client map, room map, FIFO linked list, and queue-removal index. Each queued visitor has one list entry and one expiry time. A room has two distinct clients. The hub removes membership before sending departure notices, so a full peer buffer cannot leave a half-ended room.

Commands and outbound events use bounded channels. The hub is the only sender and closer of client outbound channels. If an outbound channel fills, the hub removes that slow client. When the command channel fills, the submitter is marked closed without spawning a worker; its writer closes the transport, and the hub removes it on its next event or one-second tick. Cleanup is idempotent.

Each client stores one last-partner ID. Candidates are skipped if either still remembers the other. Disconnect removes references to the departing connection. This avoids immediate rematches without a growing block list or a busy retry loop. Queue timeouts are checked by one hub ticker; there is no timer per join.

The admission limiter has a separate mutex because HTTP upgrades occur concurrently before hub registration. It bounds active/reserved connections, per-IP quotas, token buckets, and its identity map. One maintenance ticker removes idle limiter entries and records aggregate hub/runtime metrics. Active quota records are never evicted.

## Local origin and IP handling

The backend accepts an exact Origin allowlist. It binds only to a configured loopback IP. Vite connects to `127.0.0.1:8080`; browsers connect to `localhost:5173`. No CORS middleware is needed for the same-origin browser flow.

Gin uses `SetTrustedProxies(nil)`. The quota uses the TCP peer address, and ignores `X-Forwarded-For` and other forwarded headers. All local Vite connections therefore share the loopback quota. This is deliberate for a local-only application: direct requests to the backend cannot forge a different quota identity with a header. A future public proxy arrangement would need an explicit trust and client-IP design.

## Browser lifecycle

`useChat` keeps the socket in a ref and rejects callbacks from old socket generations. Its reducer separates connection state from server-owned chat state. Transition commands have request correlation and a bounded response timeout. Outgoing messages appear only after the server echo. Socket `send()` failures do not create an offline queue.

The hook retries at most five consecutive connection attempts, using exponential delay and jitter capped at 15 seconds. A new connection returns to idle. Unmount and page departure remove listeners, close sockets, and clear timers. Returning from the browser's back/forward cache starts a new session.

Messages and drafts never enter persistent browser storage. Stop, Next, partner departure, and disconnect release the current transcript and draft. The reducer keeps the newest 200 messages and ignores old-room events. `MessageComposer` owns its local draft and is remounted when the hook changes the content version. The message log renders React text nodes; it interprets no HTML or Markdown.

## Delivery and shutdown

A server echo confirms validation and attempted relay to bounded buffers. It cannot prove that a peer received or read a message. Connections have no resume token, transcript replay, or automatic retry. Losing either transport ends the room.

On SIGINT/SIGTERM, admission closes and readiness becomes unavailable. The hub removes queue and room state, signals writers to send 1001, and closes outbound channels. Writers explicitly close upgraded sockets, which also unblocks readers. HTTP shutdown and pump waiting share a deadline of at most 10 seconds. A slow or broken network can prevent delivery of the close frame; it cannot keep a writer past its write deadline.

## Tradeoffs

One hub makes state changes easy to reason about, but also places all matching work in one event loop. The default capacity is an operational bound, not a public-service guarantee. The load report describes the actual hardware, traffic, and measurements.

Running multiple processes would require shared matchmaking, connection-owner routing, transient event relay, node failure cleanup, and bounded inter-process queues. These relays must not become durable transcripts. No distributed infrastructure is implemented here.
