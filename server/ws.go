package server

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Client struct {
	ID   string
	Conn *websocket.Conn
	Busy bool
	mu   sync.Mutex
}

func (c *Client) Send(msg interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn.WriteJSON(msg)
}

// WSTaskMessage is sent to clients when dispatching a task.
// Contains the full task info including type and all params.
type WSTaskMessage struct {
	Type   string          `json:"type"`   // always "task"
	TaskID string          `json:"taskId"`
	TaskType string        `json:"taskType"`
	Params json.RawMessage `json:"params"`
}

// WSResultMessage is received from clients when a task is solved.
type WSResultMessage struct {
	Type     string          `json:"type"` // "result" or "error"
	TaskID   string          `json:"taskId,omitempty"`
	Solution json.RawMessage `json:"solution,omitempty"`
	Error    string          `json:"error,omitempty"`
}

type ClientManager struct {
	mu      sync.RWMutex
	clients map[string]*Client
}

func NewClientManager() *ClientManager {
	return &ClientManager{
		clients: make(map[string]*Client),
	}
}

func (m *ClientManager) Add(c *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[c.ID] = c
}

func (m *ClientManager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, id)
}

func (m *ClientManager) FindIdle() *Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, c := range m.clients {
		c.mu.Lock()
		if !c.Busy {
			c.mu.Unlock()
			return c
		}
		c.mu.Unlock()
	}
	return nil
}

func (m *ClientManager) SetBusy(id string, busy bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if c, ok := m.clients[id]; ok {
		c.mu.Lock()
		c.Busy = busy
		c.mu.Unlock()
	}
}

func (m *ClientManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clients)
}

func TryAssignTask(cm *ClientManager, task *Task) bool {
	client := cm.FindIdle()
	if client == nil {
		return false
	}

	msg := WSTaskMessage{
		Type:     "task",
		TaskID:   task.ID,
		TaskType: task.Type,
		Params:   task.Params,
	}

	if err := client.Send(msg); err != nil {
		return false
	}

	client.mu.Lock()
	client.Busy = true
	client.mu.Unlock()

	task.Status = "assigned"
	task.AssignedTo = client.ID
	now := time.Now()
	task.AssignedAt = &now

	return true
}

func sendTaskToClient(client *Client, task *Task) {
	msg := WSTaskMessage{
		Type:     "task",
		TaskID:   task.ID,
		TaskType: task.Type,
		Params:   task.Params,
	}
	client.Send(msg)
	client.mu.Lock()
	client.Busy = true
	client.mu.Unlock()
}

func HandleWebSocket(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("WebSocket upgrade error: %v", err)
			return
		}

		client := &Client{
			ID:   uuid.New().String(),
			Conn: conn,
		}

		s.clients.Add(client)
		log.Printf("Client connected: %s (total: %d)", client.ID, s.clients.Count())

		// Try to assign any pending tasks to this new client
		if task := s.tasks.AssignPending(client.ID); task != nil {
			sendTaskToClient(client, task)
		}

		defer func() {
			conn.Close()
			s.clients.Remove(client.ID)
			log.Printf("Client disconnected: %s (total: %d)", client.ID, s.clients.Count())
		}()

		// Ping ticker
		pingTicker := time.NewTicker(30 * time.Second)
		defer pingTicker.Stop()

		go func() {
			for range pingTicker.C {
				client.Send(map[string]string{"type": "ping"})
			}
		}()

		for {
			_, msgBytes, err := conn.ReadMessage()
			if err != nil {
				break
			}

			var raw map[string]json.RawMessage
			if err := json.Unmarshal(msgBytes, &raw); err != nil {
				continue
			}

			var msgType string
			if t, ok := raw["type"]; ok {
				json.Unmarshal(t, &msgType)
			}

			switch msgType {
			case "pong":
				// heartbeat

			case "result":
				var msg WSResultMessage
				json.Unmarshal(msgBytes, &msg)
				if msg.TaskID != "" && msg.Solution != nil {
					s.tasks.Complete(msg.TaskID, msg.Solution)
					s.clients.SetBusy(client.ID, false)
					log.Printf("Task %s completed by client %s", msg.TaskID, client.ID)

					if task := s.tasks.AssignPending(client.ID); task != nil {
						sendTaskToClient(client, task)
					}
				}

			case "error":
				var msg WSResultMessage
				json.Unmarshal(msgBytes, &msg)
				if msg.TaskID != "" {
					errMsg := msg.Error
					if errMsg == "" {
						errMsg = "unknown error from client"
					}
					s.tasks.Fail(msg.TaskID, errMsg)
					s.clients.SetBusy(client.ID, false)
					log.Printf("Task %s failed by client %s: %s", msg.TaskID, client.ID, errMsg)

					if task := s.tasks.AssignPending(client.ID); task != nil {
						sendTaskToClient(client, task)
					}
				}
			}
		}
	}
}
