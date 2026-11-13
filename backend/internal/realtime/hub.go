package realtime

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"geolog/internal/domain"
	"geolog/internal/ids"

	"github.com/gorilla/websocket"
)

const (
	clientBuffer = 32
	pingInterval = 30 * time.Second
	pongTimeout  = 60 * time.Second
	writeTimeout = 5 * time.Second
)

type client struct {
	send chan []byte
	done chan struct{}
}

func newClient() *client {
	return &client{send: make(chan []byte, clientBuffer), done: make(chan struct{})}
}

type Hub struct {
	register, unregister chan *client
	queue                chan []byte
	stop, done           chan struct{}
	closeOnce            sync.Once
	connections          sync.WaitGroup
	connectionMu         sync.Mutex
	upgrader             websocket.Upgrader
}

func NewHub(allowedOrigins string) *Hub {
	origins := strings.Split(allowedOrigins, ",")
	h := &Hub{
		register: make(chan *client), unregister: make(chan *client),
		queue: make(chan []byte, 256), stop: make(chan struct{}), done: make(chan struct{}),
		upgrader: websocket.Upgrader{HandshakeTimeout: writeTimeout, CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}
			for _, entry := range origins {
				if allowed := strings.TrimSpace(entry); allowed == "*" || allowed == origin {
					return true
				}
			}
			return false
		}},
	}
	go h.run()
	return h
}

func (h *Hub) Close() {
	h.closeOnce.Do(func() {
		h.connectionMu.Lock()
		close(h.stop)
		h.connectionMu.Unlock()
	})
	<-h.done
	h.connections.Wait()
}

func (h *Hub) add(c *client) bool {
	select {
	case h.register <- c:
		return true
	case <-h.stop:
		return false
	}
}

func (h *Hub) remove(c *client) {
	select {
	case h.unregister <- c:
	case <-h.stop:
	}
}

func (h *Hub) run() {
	clients := make(map[*client]struct{})
	drop := func(c *client) {
		if _, ok := clients[c]; ok {
			delete(clients, c)
			close(c.done)
		}
	}
	defer close(h.done)
	defer func() {
		for c := range clients {
			drop(c)
		}
	}()
	for {
		select {
		case <-h.stop:
			return
		default:
		}
		select {
		case <-h.stop:
			return
		case c := <-h.register:
			clients[c] = struct{}{}
		case c := <-h.unregister:
			drop(c)
		case payload := <-h.queue:
			for c := range clients {
				select {
				case c.send <- payload:
				default:
					drop(c)
				}
			}
		}
	}
}

func (h *Hub) Publish(a domain.Alert) {
	payload, err := json.Marshal(map[string]any{
		"event_id": ids.FormatEvent(a.ID), "event_type": a.EventType, "timestamp": a.Timestamp.UTC().Format(time.RFC3339),
		"vehicle":  map[string]any{"vehicle_id": ids.FormatVehicle(a.VehicleID), "vehicle_number": a.VehicleNumber, "driver_name": a.DriverName},
		"geofence": map[string]any{"geofence_id": ids.FormatGeofence(a.GeofenceID), "geofence_name": a.GeofenceName, "category": a.Category},
		"location": map[string]any{"latitude": a.Latitude, "longitude": a.Longitude},
	})
	if err != nil {
		log.Printf("encode alert %d: %v", a.ID, err)
		return
	}
	select {
	case <-h.stop:
		return
	default:
	}
	select {
	case h.queue <- payload:
	default:
		log.Printf("live alert queue full, dropped event %d", a.ID)
	}
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.connectionMu.Lock()
	select {
	case <-h.stop:
		h.connectionMu.Unlock()
		http.Error(w, "Alert stream unavailable", http.StatusServiceUnavailable)
		return
	default:
		h.connections.Add(1)
		h.connectionMu.Unlock()
	}
	defer h.connections.Done()

	c := newClient()
	if !h.add(c) {
		http.Error(w, "Alert stream unavailable", http.StatusServiceUnavailable)
		return
	}
	defer h.remove(c)
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		defer conn.Close()
		c.write(conn)
	}()

	conn.SetReadLimit(4096)
	if err := conn.SetReadDeadline(time.Now().Add(pongTimeout)); err != nil {
		h.remove(c)
		<-writerDone
		return
	}
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongTimeout))
	})
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
	h.remove(c)
	<-writerDone
}

func (c *client) write(conn *websocket.Conn) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	closeSocket := func() {
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseGoingAway, "Alert stream closed"), time.Now().Add(writeTimeout))
	}
	for {
		select {
		case <-c.done:
			closeSocket()
			return
		default:
		}
		select {
		case <-c.done:
			closeSocket()
			return
		case payload := <-c.send:
			if err := conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		case <-ticker.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeTimeout)); err != nil {
				return
			}
		}
	}
}
