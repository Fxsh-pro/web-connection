package rooms

import "sync"

type Client struct {
	ID        string
	RoomID    string
	Name      string
	SendQueue chan []byte
}

type Room struct {
	ID      string
	Members map[string]*Client
}

type Manager struct {
	mu    sync.RWMutex
	rooms map[string]*Room
}

type Peer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func NewManager() *Manager {
	return &Manager{rooms: make(map[string]*Room)}
}

func (m *Manager) AddClient(roomID string, c *Client) ([]Peer, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rooms[roomID]
	if !ok {
		r = &Room{ID: roomID, Members: make(map[string]*Client)}
		m.rooms[roomID] = r
	}
	existing := make([]Peer, 0, len(r.Members))
	for id, member := range r.Members {
		existing = append(existing, Peer{ID: id, Name: member.Name})
	}
	r.Members[c.ID] = c
	return existing, len(r.Members)
}

func (m *Manager) RemoveClient(c *Client) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rooms[c.RoomID]
	if !ok {
		return 0
	}
	delete(r.Members, c.ID)
	if len(r.Members) == 0 {
		delete(m.rooms, c.RoomID)
		return 0
	}
	return len(r.Members)
}

func (m *Manager) BroadcastToRoom(roomID string, exceptID string, data []byte) {
	m.mu.RLock()
	r, ok := m.rooms[roomID]
	if !ok {
		m.mu.RUnlock()
		return
	}
	members := make([]*Client, 0, len(r.Members))
	for _, c := range r.Members {
		if c.ID == exceptID {
			continue
		}
		members = append(members, c)
	}
	m.mu.RUnlock()
	for _, c := range members {
		select {
		case c.SendQueue <- data:
		default:
		}
	}
}

func (m *Manager) SendToClient(roomID string, toID string, data []byte) {
	m.mu.RLock()
	r, ok := m.rooms[roomID]
	if !ok {
		m.mu.RUnlock()
		return
	}
	target, ok := r.Members[toID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	select {
	case target.SendQueue <- data:
	default:
	}
}

func (m *Manager) IsMember(roomID string, clientID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.rooms[roomID]
	if !ok {
		return false
	}
	_, ok = r.Members[clientID]
	return ok
}
