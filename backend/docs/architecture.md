# Architecture

## Backend

`main.go` loads configuration, wires constructors, starts the hub and HTTP listener, and handles shutdown. Gin routes use the request middleware for public errors and operational logging. Connections, rooms, waiting sessions, and rate-limit entries stay in memory.

The chat module has two packages:

- `domain` contains client state and room entities.
- `session` contains the handler, service, DTOs, request decoder, response mapper, WebSocket client, and hub.

`ChatHandler` checks the browser Origin and reserves connection capacity before upgrading. Failed upgrades and registrations release the reservation. Successful connections retain capacity until both socket pumps exit.

`ChatService` validates message length before submitting commands. Request decoding rejects unknown fields, duplicate JSON keys, invalid types, and unsupported commands. The response mapper converts application errors into public error codes.

`client.go` owns socket I/O, heartbeat deadlines, and per-connection limits. Each connection has one reader and one writer. Control frames are routed through the writer, and the hub never writes directly to a socket.

## Queue and rooms

`hub.go` owns the event loop, client registry, bounded command channel, and connection cleanup. `queue.go` handles FIFO matching, cancellation, and expiry. `room.go` checks command state, room ID, and membership before relaying messages or ending rooms.

One goroutine mutates the client map, room map, waiting list, and queue index. Each waiting client has one entry and expiry time. Rooms contain two distinct clients. Membership is removed before departure notifications are sent.

Each client remembers its last partner. Matching skips a candidate if either client still remembers the other. Disconnect removes references to the departing client. One hub ticker checks queue expiry and closed connections.

Outbound buffers and the command channel are bounded. A full outbound buffer removes the slow client. A full command channel closes the submitting connection, which the hub removes on its next command or maintenance tick. Cleanup is idempotent.

## Admission and shared helpers

`middleware/rate_limit.go` protects global and per-IP reservations with a mutex because upgrades arrive concurrently. Idle limiter entries expire; active entries remain until their connections close.

`pkg/rate_limit.go` provides the token bucket used by admission and connection limits. `pkg/id.go` creates random IDs for clients, rooms, and messages. These helpers do not depend on HTTP or WebSocket transport.

The backend binds to loopback and accepts exact configured origins. Gin trusts no proxies. Admission uses the TCP peer address and ignores forwarded headers. Vite proxies local requests, so browser tabs share its loopback IP quota.

## Frontend

`pages/chat.tsx` connects the chat components and actions. `features/chat/hooks/use-chat.ts` owns the socket lifecycle, reconnects, timers, and command sending. `state.ts` holds session transitions and command availability; `protocol.ts` validates server events.

Tests live beside their components, hook, state, and protocol. Shared buttons stay in `components/ui`. Chat-specific components and confirmation copy stay in the chat feature.

The reducer ignores stale room events and clears messages when a conversation ends. The composer remounts when private content is cleared, removing its draft. Text renders as React text nodes. The transcript retains the latest 200 messages without browser storage.

## Shutdown and deployment scope

SIGINT or SIGTERM closes admission and marks readiness unavailable. The hub clears membership, signals writers to close connections, and closes outbound channels. HTTP shutdown and socket pump waiting share a deadline of at most 10 seconds.

This application runs in one process. Multiple processes would require shared matchmaking and routing between connection owners. Queue capacity and delivery limits are documented in the [WebSocket contract](websocket.md); measurements are in [testing](testing.md).
