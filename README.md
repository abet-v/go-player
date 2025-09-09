## Go/Baduk Online (WebSocket)

This is a minimal two-player Go/Baduk/Weiqi web app with a Go backend and a static frontend using WebSockets for real-time play.

### Features
- 19x19 interactive board (click to place stones)
- Real-time multiplayer via WebSocket
- Game rooms with shareable links
- Turn indicator, captured stones
- Pass, resign, and scoring mode (Chinese area scoring; simple dead stone toggling)
- Basic styling

### Prerequisites
- Go 1.21+

### Getting started
1. Initialize the Go module and fetch deps:
   - go mod init GoPlayer
   - go get github.com/gorilla/websocket@v1.5.1
2. Run the server:
   - go run ./cmd/server
3. Open the app:
   - Navigate to http://localhost:8080
   - Click "Create a new game" and share the URL with your opponent

### Notes
- This engine implements standard rules: captures, turn order, ko (simple ko), suicide forbidden, two passes enter scoring. Scoring is Chinese-style with configurable dead stone marking. Komi is set to 6.5.
- Spectators can join if both colors already taken; they receive updates but cannot play.
- The code is intentionally compact and not production-hardened. For production, add persistence, auth, and stronger validation.

### Project layout
- cmd/server/main.go — HTTP server and routes
- internal/server/room.go — WebSocket room/session management
- internal/state/game.go — Go rules/game engine and snapshots
- web/ — static assets (HTML/CSS/JS)

### Testing
- Manual: open two different browsers/incognito windows on the same room URL, make moves and verify real-time sync.
- You can add unit tests for the engine in internal/state with `go test`.

