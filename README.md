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

CI/CD (GitHub Actions)
1) Push repo to GitHub with default branch `main`.
2) CI on PRs/commits:
   - `.github/workflows/ci.yml` builds, vets, tests.
3) Publish image to GHCR:
   - Create a tag `vX.Y.Z` and push it; `.github/workflows/publish.yml` builds and pushes `ghcr.io/<owner>/<repo>:vX.Y.Z`.
4) Deploy to your VM via SSH:
   - Set repo secrets: `DEPLOY_HOST`, `DEPLOY_USER`, `DEPLOY_SSH_KEY` (private key), and environment variables `ALLOWED_ORIGIN`, `DATABASE_URL` in your VM shell profile.
   - Trigger `.github/workflows/deploy.yml` manually (workflow_dispatch) with `image_tag` (e.g., `v1.0.0`).

Docker (build locally)
```bash
docker build -t webconnector:dev .
docker run --rm -p 8080:8080 -e LISTEN_ADDR=:8080 webconnector:dev
```

