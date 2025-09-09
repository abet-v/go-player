package server

import (
    "encoding/json"
    "log"
    "net/http"
    "sync"
    "time"

    "github.com/gorilla/websocket"

    "GoPlayer/internal/state"
)

var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
    CheckOrigin: func(r *http.Request) bool { return true },
}

// Room represents a game session with up to two players and spectators.
// Players are assigned colors (B or W) in join order if available.

type Room struct {
    id      string
    game    *state.Game
    conns   map[*Client]struct{}
    mu      sync.Mutex
}

type Client struct {
    conn   *websocket.Conn
    room   *Room
    color  state.Color // 0 for none/spectator
    sendCh chan any
    mu     sync.Mutex
}

type RoomManager struct {
    rooms map[string]*Room
    mu    sync.Mutex
}

func NewRoomManager() *RoomManager {
    return &RoomManager{rooms: make(map[string]*Room)}
}

func (rm *RoomManager) CreateRoom(id string) *Room {
    rm.mu.Lock()
    defer rm.mu.Unlock()
    if _, ok := rm.rooms[id]; ok {
        return rm.rooms[id]
    }
    r := &Room{
        id:    id,
        game:  state.NewGame(19),
        conns: make(map[*Client]struct{}),
    }
    rm.rooms[id] = r
    return r
}

func (rm *RoomManager) getRoom(id string) (*Room, bool) {
    rm.mu.Lock()
    defer rm.mu.Unlock()
    r, ok := rm.rooms[id]
    return r, ok
}

func (rm *RoomManager) ServeWS(roomID string, w http.ResponseWriter, r *http.Request) {
    room, ok := rm.getRoom(roomID)
    if !ok {
        room = rm.CreateRoom(roomID)
    }
    c, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Println("upgrade:", err)
        return
    }
    cl := &Client{conn: c, room: room, color: 0, sendCh: make(chan any, 16)}

    room.mu.Lock()
    room.conns[cl] = struct{}{}
    // Assign color if available
    colorsInUse := map[state.Color]bool{}
    for cli := range room.conns {
        if cli.color != 0 {
            colorsInUse[cli.color] = true
        }
    }
    if !colorsInUse[state.Black] {
        cl.color = state.Black
    } else if !colorsInUse[state.White] {
        cl.color = state.White
    } // else spectator
    // welcome with color info
    colorStr := cl.color.String()
    room.mu.Unlock()

    // Send initial state and welcome
    cl.send(map[string]any{"type":"welcome", "color": colorStr})
    cl.send(gameSnapshot(room))
    // Notify everyone of players/state
    broadcast(room, gameSnapshot(room))

    go cl.writer()
    cl.reader()
}

func (c *Client) send(v any) {
    select {
    case c.sendCh <- v:
    default:
        // drop if buffer full to avoid blocking; connection likely unhealthy
    }
}

func (c *Client) writer() {
    ping := time.NewTicker(20 * time.Second)
    defer ping.Stop()
    for {
        select {
        case v, ok := <-c.sendCh:
            if !ok {
                c.conn.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }
            c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
            if err := c.conn.WriteJSON(v); err != nil {
                log.Println("write:", err)
                return
            }
        case <-ping.C:
            c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
            if err := c.conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(10*time.Second)); err != nil {
                log.Println("ping:", err)
                return
            }
        }
    }
}

func (c *Client) reader() {
    defer func() {
        c.conn.Close()
        c.room.mu.Lock()
        delete(c.room.conns, c)
        c.room.mu.Unlock()
        broadcast(c.room, map[string]any{"type": "players", "players": playersList(c.room)})
    }()

    c.conn.SetReadLimit(1 << 20)
    c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
    c.conn.SetPongHandler(func(string) error {
        c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
        return nil
    })

    for {
        var msg map[string]any
        if err := c.conn.ReadJSON(&msg); err != nil {
            log.Println("read:", err)
            return
        }
        c.handle(msg)
    }
}

func (c *Client) handle(msg map[string]any) {
    t, _ := msg["type"].(string)
    switch t {
    case "place":
        // {type:"place", x:int, y:int}
        x, _ := toInt(msg["x"])
        y, _ := toInt(msg["y"])
        c.room.mu.Lock()
        err := c.room.game.Play(c.color, x, y)
        snap := gameSnapshot(c.room)
        c.room.mu.Unlock()
        if err != nil {
            c.send(map[string]any{"type": "error", "error": err.Error()})
            return
        }
        broadcast(c.room, snap)
    case "pass":
        c.room.mu.Lock()
        c.room.game.Pass(c.color)
        snap := gameSnapshot(c.room)
        c.room.mu.Unlock()
        broadcast(c.room, snap)
    case "resign":
        c.room.mu.Lock()
        c.room.game.Resign(c.color)
        snap := gameSnapshot(c.room)
        c.room.mu.Unlock()
        broadcast(c.room, snap)
    case "request-score":
        c.room.mu.Lock()
        c.room.game.StartScoring()
        snap := gameSnapshot(c.room)
        c.room.mu.Unlock()
        broadcast(c.room, snap)
    case "finalize-score":
        c.room.mu.Lock()
        c.room.game.FinalizeScore()
        snap := gameSnapshot(c.room)
        c.room.mu.Unlock()
        broadcast(c.room, snap)
    case "toggle-dead":
        x, _ := toInt(msg["x"])
        y, _ := toInt(msg["y"])
        c.room.mu.Lock()
        c.room.game.ToggleDeadAt(x, y)
        snap := gameSnapshot(c.room)
        c.room.mu.Unlock()
        broadcast(c.room, snap)
    case "sync":
        c.send(gameSnapshot(c.room))
    default:
        c.send(map[string]any{"type": "error", "error": "unknown message type"})
    }
}

func toInt(v any) (int, bool) {
    switch t := v.(type) {
    case float64:
        return int(t), true
    case int:
        return t, true
    default:
        return 0, false
    }
}

func broadcast(r *Room, v any) {
    r.mu.Lock()
    defer r.mu.Unlock()
    for c := range r.conns {
        c.send(v)
    }
}

func playersList(r *Room) []map[string]any {
    players := []map[string]any{}
    for c := range r.conns {
        if c.color != 0 {
            players = append(players, map[string]any{"color": c.color.String()})
        }
    }
    return players
}

func gameSnapshot(r *Room) map[string]any {
    g := r.game.Snapshot()
    b, _ := json.Marshal(g)
    var m map[string]any
    json.Unmarshal(b, &m)
    m["type"] = "state"
    m["players"] = playersList(r)
    return m
}

