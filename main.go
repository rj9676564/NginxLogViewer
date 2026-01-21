package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hpcloud/tail"
	"github.com/mssola/user_agent"
	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

var (
	addr       = flag.String("addr", ":58080", "http service address")
	logFile    = flag.String("file", "/var/log/nginx/access.log", "path to nginx log file")
	dbPath     = flag.String("db", "./logs.db", "path to sqlite database")
	// Keeping the default format simple, but providing a way to override via code or config later if needed.
	// For now we will use a flexible regex that adapts to the specific custom format the user mentioned.
	// In a real generic tool, we might want to parse the log_format string itself.
	formatStr  = flag.String("format", "", "Nginx log format (not fully implemented in backend cli, hardcoded for now)")
)

// LogEntry roughly matches the DB schema and JSON output
type LogEntry struct {
	ID        int64  `json:"id"`
	IP        string `json:"ip"`
	Time      string `json:"time"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	Status    int    `json:"status"`
	Bytes     int    `json:"bytes"`
	Referer   string `json:"referer"`
	UA        string `json:"ua"`
	Browser   string `json:"browser"`
	OS        string `json:"os"`
	Device    string `json:"device"` // Note: mssola/user_agent doesn't always distinguish 'device' name well, but gives mobile bool
	Query     string `json:"query"`
	Body      string `json:"body"`
	Raw       string `json:"raw"`
	CreatedAt int64  `json:"created_at"` // Unix timestamp
}

// Hub maintains the set of active clients
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan *LogEntry // Changed to broadcast structured LogEntry
	register   chan *Client
	unregister chan *Client
	mu         sync.Mutex
}

func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan *LogEntry),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
		case entry := <-h.broadcast:
			// Marshal once
			data, err := json.Marshal(entry)
			if err != nil {
				log.Printf("JSON marshal error: %v", err)
				continue
			}
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client.send <- data:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

// WebSocket setup
const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(4096)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
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
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 4096)}
	client.hub.register <- client
	go client.writePump()
	go client.readPump()
}

// DB Logic
var db *sql.DB

func initDB() {
	var err error
	db, err = sql.Open("sqlite", *dbPath)
	if err != nil {
		log.Fatal(err)
	}

	createTableSQL := `CREATE TABLE IF NOT EXISTS logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ip TEXT,
		time TEXT,
		method TEXT,
		path TEXT,
		status INTEGER,
		bytes INTEGER,
		referer TEXT,
		ua TEXT,
		browser TEXT,
		os TEXT,
		device TEXT,
		query TEXT,
		body TEXT,
		raw TEXT,
		created_at INTEGER
	);`

	if _, err := db.Exec(createTableSQL); err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
	
	// Index for faster queries
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_created_at ON logs(created_at);`)
}

func saveLog(e *LogEntry) {
	stmt, err := db.Prepare(`INSERT INTO logs(ip, time, method, path, status, bytes, referer, ua, browser, os, device, query, body, raw, created_at) values(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		log.Printf("DB Prepare error: %v", err)
		return
	}
	defer stmt.Close()

	res, err := stmt.Exec(e.IP, e.Time, e.Method, e.Path, e.Status, e.Bytes, e.Referer, e.UA, e.Browser, e.OS, e.Device, e.Query, e.Body, e.Raw, e.CreatedAt)
	if err == nil {
		id, _ := res.LastInsertId()
		e.ID = id
	} else {
		log.Printf("DB Insert error: %v", err)
	}
}

func getRecentLogs(limit int) ([]*LogEntry, error) {
	rows, err := db.Query("SELECT id, ip, time, method, path, status, bytes, referer, ua, browser, os, device, query, body, raw, created_at FROM logs ORDER BY id DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*LogEntry
	for rows.Next() {
		e := &LogEntry{}
		if err := rows.Scan(&e.ID, &e.IP, &e.Time, &e.Method, &e.Path, &e.Status, &e.Bytes, &e.Referer, &e.UA, &e.Browser, &e.OS, &e.Device, &e.Query, &e.Body, &e.Raw, &e.CreatedAt); err != nil {
			continue
		}
		logs = append(logs, e)
	}
	
	// Reverse to keep chronological order if needed, but usually frontend handles it. 
	// Let's return as is (descending) and let frontend prepend or handle it.
	return logs, nil
}

// Stats API
func handleStats(w http.ResponseWriter, r *http.Request) {
	// Simple PV stat
	var pv int
	db.QueryRow("SELECT COUNT(*) FROM logs").Scan(&pv)
	
	// UV (approx by distinct IP)
	var uv int
	db.QueryRow("SELECT COUNT(DISTINCT ip) FROM logs").Scan(&uv)
	
	stats := map[string]interface{}{
		"pv": pv,
		"uv": uv,
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(stats)
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	logs, err := getRecentLogs(100)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(logs)
}

// Regex Parser logic
// Custom format provided by user:
// '$remote_addr - $remote_user [$time_local] "$request" $status GET_ARGS: "$query_string" POST_BODY: "$request_body"'
// Note: $request usually matches "METHOD PATH PROTOCOL"
// We need to robustly matching this.
var logRegex = regexp.MustCompile(`^(?P<ip>\S+) - (?P<user>\S+) \[(?P<time>[^\]]+)\] "(?P<method>\S+) (?P<path>\S+) (?P<proto>[^"]+)" (?P<status>\d+) (?:GET_ARGS: "(?P<query>.*?)")? (?:POST_BODY: "(?P<body>.*)")?`)

// Fallback for standard combined if needed, but let's prioritize the specific format user asked for
var simpleRegex = regexp.MustCompile(`^(?P<ip>\S+) - \S+ \[(?P<time>[^\]]+)\] "(?P<request>[^"]+)" (?P<status>\d+) (?P<bytes>\d+) "-" "(?P<ua>[^"]+)"`)

func parseLine(line string) *LogEntry {
	entry := &LogEntry{
		Raw:       line,
		CreatedAt: time.Now().Unix(),
	}

	// Try specific format first
	if matches := logRegex.FindStringSubmatch(line); matches != nil {
		names := logRegex.SubexpNames()
		for i, match := range matches {
			switch names[i] {
			case "ip":
				entry.IP = match
			case "time":
				entry.Time = match
			case "method":
				entry.Method = match
			case "path":
				entry.Path = match
			case "status":
				fmt.Sscanf(match, "%d", &entry.Status)
			case "query":
				entry.Query = match
			case "body":
				entry.Body = match
			}
		}
		// Since this format doesn't have UA, we leave it empty or try to find it if we adjust regex
		// Wait, the user's format string: '$remote_addr - $remote_user [$time_local] "$request" $status GET_ARGS: "$query_string" POST_BODY: "$request_body"'
		// IT DOES NOT HAVE BYTES, REFERER or UA.
		// So we won't get UA info from this specific format unless we change the regex to be looser or the format changes.
		// Assuming the line might contain more info or we just parse what we have.
		
		// For the sake of the feature request "Browser/System", we need UA.
		// I will check if the line *actually* has UA at the end (maybe user simplified the description).
		// If strictly following user desc, we have no UA.
		// But let's try to see if there is extra text or use a generic "catch all" at the end if needed.
		// For now, if we don't have UA, Browser/OS will be empty.
	} else if matches := simpleRegex.FindStringSubmatch(line); matches != nil {
		// Fallback to standard combined for testing/other logs
		names := simpleRegex.SubexpNames()
		for i, match := range matches {
			switch names[i] {
			case "ip":
				entry.IP = match
			case "time":
				entry.Time = match
			case "request":
				// Split request
				parts := strings.Split(match, " ")
				if len(parts) >= 2 {
					entry.Method = parts[0]
					entry.Path = parts[1]
				}
			case "status":
				fmt.Sscanf(match, "%d", &entry.Status)
			case "bytes":
				fmt.Sscanf(match, "%d", &entry.Bytes)
			case "ua":
				entry.UA = match
			}
		}
	}

	// Enrich if UA is present
	if entry.UA != "" {
		ua := user_agent.New(entry.UA)
		browser, version := ua.Browser()
		entry.Browser = fmt.Sprintf("%s %s", browser, version)
		entry.OS = ua.OS()
		if ua.Mobile() {
			entry.Device = "Mobile"
		} else {
			entry.Device = "Desktop"
		}
	}

	return entry
}

func tailLog(hub *Hub, filename string) {
	t, err := tail.TailFile(filename, tail.Config{Follow: true, ReOpen: true, Location: &tail.SeekInfo{Offset: 0, Whence: 0}})
	if err != nil {
		log.Printf("Error tailing file: %v", err)
		return
	}
	
	for line := range t.Lines {
		if line.Text == "" { continue }
		
		// Parse
		entry := parseLine(line.Text)
		
		// Save DB
		saveLog(entry)
		
		// Broadcast
		hub.broadcast <- entry
	}
}

func main() {
	flag.Parse()
	initDB()
	
	hub := newHub()
	go hub.run()

	go tailLog(hub, *logFile)

	// Serve static files (Frontend build)
	fs := http.FileServer(http.Dir("./frontend/dist"))
	http.Handle("/", fs)
	
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})
	
	http.HandleFunc("/api/history", handleHistory)
	http.HandleFunc("/api/stats", handleStats)

	fmt.Printf("Server started at http://localhost%s\n", *addr)
	fmt.Printf("Watching file: %s\n", *logFile)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
