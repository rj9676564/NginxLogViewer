package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hpcloud/tail"
)

var (
	addr    = flag.String("addr", ":58080", "http service address")
	logFile = flag.String("file", "/var/log/nginx/access.log", "path to nginx log file")
)

// Hub maintains the set of active clients and broadcasts messages to the clients.
type Hub struct {
	clients map[*Client]bool
	broadcast chan []byte
	register chan *Client
	unregister chan *Client
	mu sync.Mutex
	history [][]byte // Buffer for last N log lines
}

func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		history:    make([][]byte, 0, 1000), // Cap at 1000
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			// Send history to new client
			for _, msg := range h.history {
				select {
				case client.send <- msg:
				default:
					// If client buffer full, skip (should rarely happen on init unless buffer small)
				}
			}
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.Lock()
			// Append to history
			if len(h.history) >= 1000 {
				h.history = h.history[1:] // simple shift, optimize later if needed
			}
			h.history = append(h.history, message)
			
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

const (
	writeWait = 10 * time.Second
	pongWait = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		log.Printf("CheckOrigin for: %s, Origin header: %s", r.RemoteAddr, r.Header.Get("Origin"))
		return true // Allow all
	},
}

type Client struct {
	hub *Hub
	conn *websocket.Conn
	send chan []byte
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(4096) // Increased for larger log lines
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		// We expect control messages or maybe config updates from client?
		// For now just keep alive
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	log.Printf("New connection attempt from %s", r.RemoteAddr)
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade error: %v", err)
		return
	}
	log.Printf("Connection upgraded successfully for %s", r.RemoteAddr)
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 4096)} // Increase buffer
	client.hub.register <- client

	go client.writePump()
	go client.readPump()
}

func tailLog(hub *Hub, filename string) {
	// Create a tail
	// Changed to Start (Whence: 0) to read existing content for static files analysis
	t, err := tail.TailFile(filename, tail.Config{Follow: true, ReOpen: true, Location: &tail.SeekInfo{Offset: 0, Whence: 0}}) 
	if err != nil {
		log.Printf("Error tailing file: %v", err)
		return
	}
	
	for line := range t.Lines {
		hub.broadcast <- []byte(line.Text)
	}
}

func main() {
	flag.Parse()
	hub := newHub()
	go hub.run()

	// Ensure log file exists or just warn
	if _, err := os.Stat(*logFile); os.IsNotExist(err) {
		log.Printf("Warning: file %s does not exist. Waiting for it...", *logFile)
	}

	go tailLog(hub, *logFile)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})
	
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	// Add an endpoint to change the file being watched?
	// Or maybe just an endpoint to validate log content.
	
	fmt.Printf("Server started at http://localhost%s\n", *addr)
	fmt.Printf("Watching file: %s\n", *logFile)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
