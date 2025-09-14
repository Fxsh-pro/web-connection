got WebConnector

A minimal WebRTC rooms app (Discord-like voice/video) with a Go signaling server and a static web frontend.

Features
- Rooms with multi-peer WebRTC
- Audio and video with mute toggles
- Built-in browser noise suppression / echo cancellation / auto gain
- WebSocket signaling via Go
 - Chat with persistence in Postgres

Prereqs
- Go 1.22+

Run
```bash
cd /Users/mnikishaev/GolandProjects/webConnector
go mod tidy
go run ./cmd/webconnector
```

Open two browser tabs to:
```
http://localhost:8080/
```
Enter the same room ID in both tabs and click Join.

Notes
- Signaling is in-memory and not persisted.
- TURN servers are not configured; if peers are behind symmetric NAT, add a TURN server to `RTCPeerConnection.iceServers`.
- For production, serve over HTTPS and set appropriate CORS/origin checks.

Environment
- `LISTEN_ADDR` (default `:8080`)
- `ALLOWED_ORIGIN` (e.g., `https://meet.example.com`) to restrict websocket origin
- `DATABASE_URL` Postgres connection string, e.g. `postgres://user:pass@localhost:5432/webconnector?sslmode=disable`

Dotenv
- Copy `.env.example` to `.env` and edit values:
```bash
cp .env.example .env
```

Database
If `DATABASE_URL` is set, the server will migrate a `messages` table and:
- Persist chat messages
- Load last 50 messages on join

