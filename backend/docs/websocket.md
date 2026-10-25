# WebSocket contract

Connect through `ws://localhost:5173/v1/chat/ws`. Each connection is a new anonymous session. Compression is disabled. There is no authentication, resume, replay, offline delivery, or automatic resend.

`GET /health` returns `200 {"status":"ok"}` while the process is alive. `GET /ready` returns `200 {"status":"ready"}` while the hub accepts connections, otherwise `503 {"status":"unavailable"}`.

The upgrade requires an exact allowed browser Origin (default `http://localhost:5173`). Missing, `null`, and unknown origins receive 403. Handshake/IP limits receive 429; global capacity and shutdown receive 503. Temporary rejections include `Retry-After` in seconds and a safe `code` and `message`. Browsers cannot inspect a rejected WebSocket handshake's HTTP body; after bounded retries the UI explains the possible capacity/network causes. No extra diagnostic endpoint is required.

## Envelopes

Each text frame contains exactly one JSON object. Only `type`, `request_id`, `room_id`, and `payload` are allowed. All commands require a nonempty string `request_id` with at most 64 Unicode code points. It correlates a response; it is not an identity or a deduplication key. IDs for connections, rooms, and messages come from server cryptographic randomness.

Room commands require the current `room_id`. Queue commands must omit it. Unknown fields, events, malformed/multiple JSON values, null/nonobject payloads, wrong types, invalid UTF-8, and binary frames are rejected. Duplicate JSON keys are rejected. The default frame limit is 8,192 bytes, including envelope and JSON escapes.

```json
{"type":"chat.send","request_id":"c17","room_id":"room-id","payload":{"text":"Hello"}}
```

## Client commands

| Type | Allowed state | Payload | Result for requester |
| --- | --- | --- | --- |
| `queue.join` | idle, waiting | `{}` | `queue.waiting` or `chat.matched`; a duplicate join keeps one entry |
| `queue.cancel` | waiting | `{}` | `queue.cancelled`, reason `cancelled` |
| `chat.send` | matched | `{"text":"Hello"}` | `chat.message`, sender `self` |
| `chat.typing` | matched | `{"typing":true}` | `chat.typing` echo with sender `self`; partner gets sender `peer` |
| `chat.stop` | matched | `{}` | `peer.left`, reason `stopped`, state `idle` |
| `chat.next` | matched | `{}` | `queue.waiting` or `chat.matched` for a new room |

`chat.send` trims surrounding whitespace. Text must contain 1-2,000 Unicode code points and at most 8,000 UTF-8 bytes. The entire JSON frame must also fit the frame limit. Typing sends only a boolean; a client clears the peer indicator after 3 seconds without an update.

## Server events

Every event has `type` and `payload`. `request_id` is included only in the requester's correlated event, never forwarded to the partner. `room_id` is present for every room event and absent for queue/session events. The initial `session.ready` event announces idle; sockets are never queued automatically.

| Type | Payload |
| --- | --- |
| `session.ready` | `{"state":"idle"}` |
| `queue.waiting` | `{"state":"waiting","expires_at":"2026-10-24T10:02:00Z"}` |
| `queue.cancelled` | `{"state":"idle","reason":"cancelled"}`; reason is `cancelled`, `queue_timeout`, or `shutdown` |
| `chat.matched` | `{"state":"matched"}` with the new `room_id` |
| `chat.message` | `{"message_id":"message-id","text":"Hello","sender":"self","sent_at":"2026-10-24T10:00:00Z"}`; sender is `self` or `peer` |
| `chat.typing` | `{"typing":true,"sender":"peer"}`; sender is `self` or `peer` |
| `peer.left` | `{"state":"idle","reason":"stopped"}`; reason is `stopped`, `next`, `disconnected`, or `slow_consumer` |
| `error` | `{"code":"invalid_state","message":"This action is unavailable in the current state."}`; temporary errors include `retry_after_ms` |

Messages are created by the server with a message ID and UTC timestamp, then offered to both bounded outbound buffers. The sender sees the server echo before displaying an outgoing message. An echo means accepted and attempted relay; it does not prove receipt or reading. If either outbound buffer fills, that connection is removed. No exactly-once guarantee is made.

On Next the requester clears its old transcript before the command, and the server ends the old room before queuing it. The former partner receives `peer.left` with the old room ID and stays idle. Browsers ignore room events whose IDs do not match their active room, except `chat.matched`, which introduces a room. Simultaneous Next commands may leave the second visitor idle with `invalid_state`; its old command cannot affect a new room.

Matching uses the oldest eligible waiting connection. A client cannot match itself or its last partner while either still remembers that pairing. Each connection holds one last-partner reference; departing clients are removed from all such references. Waiting expires after 120 seconds. There is no polling retry loop when only a former partner is available.

## Errors and limits

Public codes: `invalid_payload`, `unknown_event`, `invalid_state`, `stale_room`, `rate_limited`, `queue_full`, `server_busy`, and `internal_error`. Errors from a valid envelope retain `request_id` and, for room commands, `room_id`. Hub errors also include the resulting `state` (`idle`, `waiting`, or `matched`). Validation errors created before the hub omit it. An unreadable envelope cannot always provide correlation. If Next ends a room but a smaller configured queue is full, `queue_full` includes `state: idle`; the browser clears the old room and offers Find someone.

```json
{"type":"error","request_id":"c18","room_id":"old-room","payload":{"code":"stale_room","message":"This conversation has ended."}}
```

Before upgrade: IP bucket burst 10, refill 10/minute. Per connection: all commands 10/second burst 20; messages 2/second burst 5; join/next/cancel 6/minute burst 3; typing 2/second burst 2. All relevant buckets are charged together. A fifth violation within 30 seconds closes the connection with 1008. Payload violations use the same threshold. Ordinary state errors do not count as policy violations. IP limiter entries expire after 10 minutes idle, with a 10,000 entry bound.

Hub commands use a bounded channel (256). A full channel closes the submitting connection with 1008; the hub removes it on its next event or maintenance tick. No goroutine is created to absorb overflow. Outbound buffers hold at most 32 events. Cleanup is idempotent. Failed upgrades release reserved IP/global capacity.

## Close codes

| Code | Meaning |
| --- | --- |
| 1000 | Normal closure |
| 1001 | Server shutdown |
| 1002 | WebSocket protocol error detected by Gorilla |
| 1003 | Binary data unsupported |
| 1007 | Invalid UTF-8 |
| 1008 | Repeated policy violations or overloaded command channel |
| 1009 | Frame too large |
| 1011 | Internal failure |

1006 is never sent; a browser can report it after an interrupted transport. Ping interval is 25 seconds, pong wait 60 seconds, and write deadline 10 seconds. A read timeout ends the connection. Shutdown stops admission, clears the queue, requests 1001 closes through the writers, explicitly closes upgraded sockets, and waits at most 10 seconds.
