package ws

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"

	"webConnector/internal/rooms"
	"webConnector/internal/store"
)

type Message struct {
	Type string          `json:"type"`
	Room string          `json:"room,omitempty"`
	From string          `json:"from,omitempty"`
	To   string          `json:"to,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		return strings.Contains(origin, "://localhost:") || strings.Contains(origin, "://127.0.0.1:") || strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "http://127.0.0.1")
	},
}

func SetAllowedOrigin(origin string) {
	if strings.TrimSpace(origin) == "" {
		return
	}
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return r.Header.Get("Origin") == origin
	}
}

func randomID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		ts := time.Now().UnixNano()
		return hex.EncodeToString([]byte{byte(ts), byte(ts >> 8), byte(ts >> 16), byte(ts >> 24)})
	}
	return hex.EncodeToString(b[:])
}

func Handler(manager *rooms.Manager, db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID := r.URL.Query().Get("room")
		if roomID == "" {
			http.Error(w, "room is required", http.StatusBadRequest)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("ws upgrade error: %v", err)
			return
		}
		clientID := randomID()
		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if name == "" {
			name = clientID[:6]
		}
		c := &rooms.Client{ID: clientID, RoomID: roomID, Name: name, SendQueue: make(chan []byte, 64)}
		log.Printf("ws connected: room=%s client=%s remote=%s", roomID, clientID, r.RemoteAddr)

		existingPeers, _ := manager.AddClient(roomID, c)

		// Writer
		go func() {
			defer conn.Close()
			conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
			_ = conn.WriteJSON(Message{Type: "welcome", Room: roomID, From: clientID})
			conn.SetWriteDeadline(time.Time{})
			for msg := range c.SendQueue {
				conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					return
				}
				conn.SetWriteDeadline(time.Time{})
			}
		}()

		// Send peers
		peersPayload := struct {
			Type  string       `json:"type"`
			Peers []rooms.Peer `json:"peers"`
		}{Type: "peers", Peers: existingPeers}
		if data, _ := json.Marshal(peersPayload); data != nil {
			select {
			case c.SendQueue <- data:
			default:
			}
		}

		// Send history
		if db != nil {
			if history, err := store.LoadRecentMessages(r.Context(), db, roomID, 50); err == nil && len(history) > 0 {
				payload := struct {
					Type     string          `json:"type"`
					Messages []store.Message `json:"messages"`
				}{Type: "chat-history", Messages: history}
				if b, err := json.Marshal(payload); err == nil {
					select {
					case c.SendQueue <- b:
					default:
					}
				}
				log.Printf("sent chat history: room=%s client=%s count=%d", roomID, clientID, len(history))
			}
		}

		manager.BroadcastToRoom(roomID, c.ID, mustJSON(Message{Type: "peer-joined", Room: roomID, From: c.ID}))
		log.Printf("peer joined broadcast: room=%s client=%s", roomID, clientID)

		conn.SetReadLimit(1 << 19)
		conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		conn.SetPongHandler(func(string) error { conn.SetReadDeadline(time.Now().Add(90 * time.Second)); return nil })
		ticker := time.NewTicker(45 * time.Second)
		defer ticker.Stop()
		go func() {
			for range ticker.C {
				_ = conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(5*time.Second))
			}
		}()

		for {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				break
			}
			var msg Message
			if err := json.Unmarshal(payload, &msg); err != nil {
				continue
			}
			switch msg.Type {
			case "signal":
				if msg.To == "" || len(msg.Data) == 0 {
					continue
				}
				// Enforce membership: sender and recipient must be in the room
				if !manager.IsMember(roomID, c.ID) {
					log.Printf("unauthorized signal (not a member): room=%s from=%s", roomID, c.ID)
					continue
				}
				if !manager.IsMember(roomID, msg.To) {
					log.Printf("signal target not member: room=%s from=%s to=%s", roomID, c.ID, msg.To)
					continue
				}
				manager.SendToClient(roomID, msg.To, mustJSON(Message{Type: "signal", Room: roomID, From: c.ID, To: msg.To, Data: msg.Data}))
				log.Printf("signal relay: room=%s from=%s to=%s", roomID, c.ID, msg.To)
			case "chat":
				// Enforce membership: sender must be in the room
				if !manager.IsMember(roomID, c.ID) {
					log.Printf("unauthorized chat (not a member): room=%s from=%s", roomID, c.ID)
					continue
				}
				// Accept either {body:"..."} or raw string in Data
				var bodyObj struct {
					Body string `json:"body"`
				}
				var trimmed string
				if err := json.Unmarshal(msg.Data, &bodyObj); err == nil && strings.TrimSpace(bodyObj.Body) != "" {
					trimmed = strings.TrimSpace(bodyObj.Body)
				} else {
					var raw string
					if err := json.Unmarshal(msg.Data, &raw); err == nil {
						trimmed = strings.TrimSpace(raw)
					}
				}
				if trimmed == "" || len(trimmed) > 4000 {
					continue
				}
				created := time.Now().UTC().Format(time.RFC3339)
				var id int64
				if db != nil {
					if savedID, err := store.SaveMessage(context.Background(), db, roomID, c.ID, c.Name, trimmed, created); err == nil {
						id = savedID
						log.Printf("chat saved: room=%s id=%d sender=%s len=%d", roomID, id, c.ID, len(trimmed))
					} else {
						log.Printf("chat save error: room=%s sender=%s err=%v", roomID, c.ID, err)
					}
				}
				out := store.Message{ID: id, Room: roomID, Sender: c.ID, SenderName: c.Name, Body: trimmed, CreatedAt: created}
				manager.BroadcastToRoom(roomID, "", mustJSON(struct {
					Type string        `json:"type"`
					Msg  store.Message `json:"msg"`
				}{Type: "chat", Msg: out}))
				log.Printf("chat broadcast: room=%s sender=%s len=%d", roomID, c.ID, len(trimmed))
			}
		}

		remaining := manager.RemoveClient(c)
		close(c.SendQueue)
		_ = conn.Close()
		if remaining > 0 {
			manager.BroadcastToRoom(roomID, c.ID, mustJSON(Message{Type: "peer-left", Room: roomID, From: c.ID}))
		}
		log.Printf("ws disconnected: room=%s client=%s remaining=%d", roomID, clientID, remaining)
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
