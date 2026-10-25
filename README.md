# Realtime Chat

An anonymous one-to-one text chat built with Go, Gin, Gorilla WebSocket, React, TypeScript, and Tailwind CSS. Visitors join a waiting queue, exchange messages with a partner, and can end the conversation or find someone new.

## Features

- FIFO matchmaking with cancellation and a two-minute queue timeout.
- Stop and Next actions with confirmation before ending a conversation.
- Text messages, server acknowledgements, and typing indicators.
- Room validation and prevention of immediate rematches with the last partner.
- Connection quotas, rate limits, and bounded delivery buffers.
- Automatic reconnect with a manual retry after five attempts.
- Responsive layout, keyboard shortcuts, and IME input support.
- In-memory sessions with no accounts or saved transcripts.

## Main routes

| Method | Route | Description |
| --- | --- | --- |
| GET | `/health` | Process health |
| GET | `/ready` | Whether the chat hub accepts connections |
| GET | `/v1/chat/ws` | WebSocket connection for an anonymous session |

The WebSocket route requires an allowed browser Origin. Commands cover queue join/cancel, message sending, typing, Stop, and Next. See the [WebSocket contract](backend/docs/websocket.md) for payloads, events, and limits.

## Project structure

```text
.
├── backend
│   ├── docs
│   ├── internal
│   │   ├── chat
│   │   │   ├── domain
│   │   │   └── session
│   │   ├── config
│   │   ├── middleware
│   │   └── router
│   ├── pkg
│   ├── scripts
│   │   └── load-test
│   ├── .env.example
│   ├── main.go
│   └── Makefile
├── frontend
│   ├── e2e
│   ├── src
│   │   ├── components
│   │   │   └── ui
│   │   ├── features
│   │   │   └── chat
│   │   ├── pages
│   │   ├── styles
│   │   ├── test
│   │   ├── app.tsx
│   │   └── main.tsx
│   └── .env.example
├── Makefile
└── README.md
```

The backend groups domain entities and session handling under the chat module. Handlers manage WebSocket connections, services validate commands, and the hub owns queue and room state. DTOs, request decoding, and response mapping stay beside the session code. Shared ID and token-bucket helpers live in `pkg`.

The frontend keeps chat components, state, protocol validation, and connection handling in `features/chat`. Shared UI components stay in `components/ui`, and tests sit beside the code they cover.

## Run locally

Requirements:

- Go 1.26 or newer
- Node.js 22.22.1, as specified in `.nvmrc`
- npm

From the repository root:

```bash
make install
```

Start the backend and frontend in separate terminals:

```bash
make backend
```

```bash
make frontend
```

- **Frontend:** `http://localhost:5173`
- **Backend:** `http://127.0.0.1:8080`
- **WebSocket:** `ws://localhost:5173/v1/chat/ws`

Vite proxies `/v1`, `/health`, and `/ready` to the backend. Open the frontend with `localhost` to match the default allowed Origin. No database or external service is required. Fonts are bundled with the frontend.

Defaults work without an environment file. The backend reads exported variables. To use the example configuration, run this from `backend/`:

```bash
cp .env.example .env
set -a
source .env
set +a
make run
```

Vite loads `frontend/.env` automatically. If the backend port changes, copy `frontend/.env.example` to `frontend/.env` and update `CHAT_PROXY_TARGET`.

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `CHAT_ADDRESS` | `127.0.0.1:8080` | Loopback IP and HTTP port |
| `CHAT_ORIGINS` | `http://localhost:5173` | Comma-separated allowed browser origins |
| `CHAT_MAX_CONNECTIONS` | `500` | Total reserved and active connections |
| `CHAT_MAX_CONNECTIONS_PER_IP` | `5` | Connections per IP address |
| `CHAT_QUEUE_CAPACITY` | `500` | Maximum waiting sessions |
| `CHAT_COMMAND_BUFFER` | `256` | Pending hub commands |
| `CHAT_OUTBOUND_BUFFER` | `32` | Pending events per connection |
| `CHAT_FRAME_BYTES` | `8192` | Maximum WebSocket frame size |
| `CHAT_MESSAGE_CHARACTERS` | `2000` | Maximum Unicode code points per message |
| `CHAT_MESSAGE_BYTES` | `8000` | Maximum UTF-8 bytes per message |
| `CHAT_QUEUE_WAIT` | `120s` | Queue timeout |
| `CHAT_SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown deadline |
| `CHAT_PROXY_TARGET` | `http://127.0.0.1:8080` | Frontend development proxy target |

See [backend/.env.example](backend/.env.example) for heartbeat, handshake, and limiter settings. Durations use Go units such as `ms`, `s`, and `m`. If message or frame limits change, update the matching [frontend configuration](frontend/src/features/chat/config.ts).

## Chat workflow

1. Open two tabs and select **Start chatting** in both.
2. Send a message with Enter; use Shift+Enter for a new line.
3. Select **Next person** and confirm to end the current room and enter the queue again. A third tab can provide a new partner.
4. Select **Stop** and confirm, or close one tab, to end the conversation.

The former partner stays idle after Next. Previous partners cannot immediately match again while either connection still remembers the pairing. Each tab has its own anonymous session and shares the local IP connection quota.

A reconnect starts a fresh idle session. It does not rejoin the queue or resend messages. Messages appear after the server acknowledgement; this does not confirm that the partner read them.

Messages and drafts stay in memory and clear when the conversation ends or the connection is lost. A tab retains at most 200 messages. The server processes text for delivery and IP addresses for rate limits; logs contain aggregate operational data. Users can copy messages, and content is not moderated. Local HTTP/WebSocket traffic is unencrypted.

## Development checks

From the repository root:

```bash
make format
make check
make build
make test-critical
```

Formatting uses gofmt for Go and Prettier for the frontend. `make check` runs formatting checks, Go vet, frontend lint, TypeScript compilation, and the frontend build. `make build` also compiles the backend and load-test runner.

`make test-critical` runs selected backend tests with race detection for matching, room isolation, payload validation, admission, concurrent disconnects, and shutdown. Frontend checks cover event validation, session state, and connection lifecycle.

Full unit tests, browser scenarios, and load tests are available separately through `make test`, `make browser`, and `make load`. See [testing](backend/docs/testing.md) for setup and commands.

## Documentation

- [Architecture](backend/docs/architecture.md)
- [WebSocket contract](backend/docs/websocket.md)
- [Testing and load results](backend/docs/testing.md)
