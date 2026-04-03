package main

import (
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mssola/user_agent"
	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "none"
)

var (
	addr       = flag.String("addr", ":58080", "http service address")
	dbPath     = flag.String("db", "./logs.db", "path to sqlite database")
	staticDir  = flag.String("static", "./frontend/dist", "path to frontend static files")
	configPath = flag.String("config", "", "path to config.json file")
)

type Config struct {
	Addr      string `json:"addr"`
	DBPath    string `json:"db_path"`
	StaticDir string `json:"static_dir"`
}

func loadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var cfg Config
	if err := json.NewDecoder(file).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// IncomingLog is what the client sends
type IncomingLog struct {
	Level string `json:"level"`
	Tag   string `json:"tag"`
	Text  string `json:"text"`
	Time  string `json:"time"` // Optional
	Body  string `json:"body"` // Structured data
}

type BatchRequest struct {
	DeviceID string        `json:"device_id"`
	Logs     []IncomingLog `json:"logs"`
}

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
	DeviceID  string `json:"device_id"`
	Level     string `json:"level"`
	Tag       string `json:"tag"`
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
		device_id TEXT,
		level TEXT,
		tag TEXT,
		query TEXT,
		body TEXT,
		raw TEXT,
		created_at INTEGER
	);`

	if _, err := db.Exec(createTableSQL); err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
	
	log.Printf("Database initialized successfully at %s", *dbPath)
	
	// Index for faster queries
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_created_at ON logs(created_at);`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_device_id ON logs(device_id);`)
	
	// Optimization for concurrent reads/writes
	db.Exec(`PRAGMA journal_mode=WAL;`)
	db.Exec(`PRAGMA synchronous=NORMAL;`)
}

func saveLog(e *LogEntry) {
	stmt, err := db.Prepare(`INSERT INTO logs(ip, time, method, path, status, bytes, referer, ua, browser, os, device, device_id, level, tag, query, body, raw, created_at) values(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		log.Printf("DB Prepare error: %v", err)
		return
	}
	defer stmt.Close()

	res, err := stmt.Exec(e.IP, e.Time, e.Method, e.Path, e.Status, e.Bytes, e.Referer, e.UA, e.Browser, e.OS, e.Device, e.DeviceID, e.Level, e.Tag, e.Query, e.Body, e.Raw, e.CreatedAt)
	if err == nil {
		id, _ := res.LastInsertId()
		e.ID = id
	} else {
		log.Printf("DB Insert error: %v", err)
	}
}

func saveLogBatch(entries []*LogEntry) {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("TX Begin error: %v", err)
		return
	}
	stmt, err := tx.Prepare(`INSERT INTO logs(ip, time, method, path, status, bytes, referer, ua, browser, os, device, device_id, level, tag, query, body, raw, created_at) values(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		log.Printf("TX Prepare error: %v", err)
		tx.Rollback()
		return
	}
	defer stmt.Close()

	for _, e := range entries {
		res, err := stmt.Exec(e.IP, e.Time, e.Method, e.Path, e.Status, e.Bytes, e.Referer, e.UA, e.Browser, e.OS, e.Device, e.DeviceID, e.Level, e.Tag, e.Query, e.Body, e.Raw, e.CreatedAt)
		if err == nil {
			id, _ := res.LastInsertId()
			e.ID = id
		}
	}
	tx.Commit()
}

func startCleanupTask() {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for range ticker.C {
			// Keep only last 100,000 logs
			res, err := db.Exec("DELETE FROM logs WHERE id < (SELECT MIN(id) FROM (SELECT id FROM logs ORDER BY id DESC LIMIT 100000))")
			if err == nil {
				rows, _ := res.RowsAffected()
				if rows > 0 {
					log.Printf("[Cleanup] Removed %d old logs", rows)
				}
			}
		}
	}()
}

func getRecentLogs(limit int, deviceID, level, tag string) ([]*LogEntry, error) {
	queryStr := "SELECT id, ip, time, method, path, status, bytes, referer, ua, browser, os, device, device_id, level, tag, query, body, raw, created_at FROM logs WHERE 1=1"
	var args []interface{}
	if deviceID != "" {
		queryStr += " AND device_id = ?"
		args = append(args, deviceID)
	}
	if level != "" {
		queryStr += " AND level = ?"
		args = append(args, level)
	}
	if tag != "" {
		queryStr += " AND tag LIKE ?"
		args = append(args, "%"+tag+"%")
	}
	queryStr += " ORDER BY id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := db.Query(queryStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*LogEntry
	for rows.Next() {
		e := &LogEntry{}
		if err := rows.Scan(&e.ID, &e.IP, &e.Time, &e.Method, &e.Path, &e.Status, &e.Bytes, &e.Referer, &e.UA, &e.Browser, &e.OS, &e.Device, &e.DeviceID, &e.Level, &e.Tag, &e.Query, &e.Body, &e.Raw, &e.CreatedAt); err != nil {
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
	q := r.URL.Query()
	deviceID := q.Get("device")
	level := q.Get("level")
	tag := q.Get("tag")
	logs, err := getRecentLogs(200, deviceID, level, tag)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(logs)
}

func handleDevices(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT DISTINCT device_id FROM logs WHERE device_id IS NOT NULL AND device_id != ''")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var devices []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err == nil {
			devices = append(devices, d)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(devices)
}

func handleTags(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT DISTINCT tag FROM logs WHERE tag IS NOT NULL AND tag != ''")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err == nil {
			tags = append(tags, t)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(tags)
}

func handleReceiveLog(hub *Hub, w http.ResponseWriter, r *http.Request) {
	// /log/:deviceid
	path := r.URL.Path
	deviceID := ""
	if strings.HasPrefix(path, "/log/") {
		deviceID = strings.TrimPrefix(path, "/log/")
	}

	entry := &LogEntry{
		IP:        r.RemoteAddr,
		Time:      time.Now().Format("02/Jan/2006:15:04:05 -0700"),
		Method:    r.Method,
		Path:      path,
		Status:    200,
		DeviceID:  deviceID,
		Level:     r.URL.Query().Get("level"),
		Tag:       r.URL.Query().Get("tag"),
		Query:     r.URL.RawQuery,
		CreatedAt: time.Now().Unix(),
		Raw:       fmt.Sprintf("%s %s?%s", r.Method, path, r.URL.RawQuery),
	}

	// Try to get UA
	entry.UA = r.Header.Get("User-Agent")
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

	saveLog(entry)
	hub.broadcast <- entry

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func handleBatchLog(hub *Hub, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req BatchRequest
	bodyReader := r.Body
	if r.Header.Get("Content-Encoding") == "gzip" {
		gr, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, "Invalid Gzip", http.StatusBadRequest)
			return
		}
		defer gr.Close()
		bodyReader = gr
	}
	
	if err := json.NewDecoder(bodyReader).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	deviceID := req.DeviceID
	if deviceID == "" && strings.HasPrefix(r.URL.Path, "/api/log/batch/") {
		deviceID = strings.TrimPrefix(r.URL.Path, "/api/log/batch/")
	}

	nowStr := time.Now().Format("02/Jan/2006:15:04:05 -0700")
	nowUnix := time.Now().Unix()
	var entries []*LogEntry

	for _, l := range req.Logs {
		logTime := l.Time
		if logTime == "" {
			logTime = nowStr
		}

		entry := &LogEntry{
			IP:        r.RemoteAddr,
			Time:      logTime,
			Method:    "BATCH",
			Path:      "/api/log/batch",
			Status:    200,
			DeviceID:  deviceID,
			Level:     l.Level,
			Tag:       l.Tag,
			Query:     l.Text,
			Body:      l.Body,
			CreatedAt: nowUnix,
			Raw:       fmt.Sprintf("[%s] %s: %s", l.Level, l.Tag, l.Text),
			UA: r.Header.Get("User-Agent"),
		}
		entries = append(entries, entry)
	}

	saveLogBatch(entries)
	for _, e := range entries {
		hub.broadcast <- e
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Processed %d logs", len(entries))))
}

func handlePushLog(hub *Hub, w http.ResponseWriter, r *http.Request) {
	// /api/log/push/:device_id
	deviceID := strings.TrimPrefix(r.URL.Path, "/api/log/push/")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Read error", http.StatusInternalServerError)
		return
	}

	content := string(body)
	if content == "" {
		http.Error(w, "Empty body", http.StatusBadRequest)
		return
	}

	var entry *LogEntry
	nowStr := time.Now().Format("02/Jan/2006:15:04:05 -0700")
	nowUnix := time.Now().Unix()

	// Try JSON first
	var incoming IncomingLog
	if json.Unmarshal(body, &incoming) == nil && (incoming.Text != "" || incoming.Body != "") {
		entry = &LogEntry{
			IP:        r.RemoteAddr,
			Time:      nowStr,
			Method:    "PUSH",
			Path:      "/api/log/push",
			Status:    200,
			DeviceID:  deviceID,
			Level:     incoming.Level,
			Tag:       incoming.Tag,
			Query:     incoming.Text,
			Body:      incoming.Body,
			CreatedAt: nowUnix,
			Raw:       fmt.Sprintf("[%s] %s: %s", incoming.Level, incoming.Tag, incoming.Text),
		}
	} else {
		// Treat as raw text
		entry = &LogEntry{
			IP:        r.RemoteAddr,
			Time:      nowStr,
			Method:    "PUSH",
			Path:      "/api/log/push",
			Status:    200,
			DeviceID:  deviceID,
			Level:     r.URL.Query().Get("level"),
			Tag:       r.URL.Query().Get("tag"),
			Query:     content,
			CreatedAt: nowUnix,
			Raw:       content,
		}
	}

	if entry.Level == "" {
		entry.Level = "info"
	}

	// UA Parsing
	entry.UA = r.Header.Get("User-Agent")
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

	saveLog(entry)
	hub.broadcast <- entry

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func main() {
	flag.Parse()

	// 1. Start with defaults
	finalAddr := ":58080"
	finalDBPath := "./logs.db"
	finalStaticDir := "./frontend/dist"

	// 2. Override with Config File if provided
	if *configPath != "" {
		if cfg, err := loadConfig(*configPath); err == nil {
			if cfg.Addr != "" { finalAddr = cfg.Addr }
			if cfg.DBPath != "" { finalDBPath = cfg.DBPath }
			if cfg.StaticDir != "" { finalStaticDir = cfg.StaticDir }
			log.Printf("Loaded config from %s", *configPath)
		} else {
			log.Printf("Warning: Failed to load config from %s: %v", *configPath, err)
		}
	}

	// 3. Override with Environment Variables
	finalAddr = getEnv("LISTEN_ADDR", finalAddr)
	finalDBPath = getEnv("DB_PATH", finalDBPath)
	finalStaticDir = getEnv("STATIC_DIR", finalStaticDir)

	// 4. Override with Command line flags (if they are NOT default values)
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "addr": finalAddr = *addr
		case "db": finalDBPath = *dbPath
		case "static": finalStaticDir = *staticDir
		}
	})

	// Apply final values
	absDBPath, _ := filepath.Abs(finalDBPath)
	
	*addr = finalAddr
	*dbPath = absDBPath
	*staticDir = finalStaticDir

	log.Printf("Starting Log Viewer (Version: %s, Commit: %s)", Version, GitCommit)
	log.Printf("Using database: %s", *dbPath)

	initDB()
	startCleanupTask()
	
	hub := newHub()
	go hub.run()

	// Serve static files (Frontend build)
	fs := http.FileServer(http.Dir(*staticDir))
	http.Handle("/", fs)
	
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})
	
	http.HandleFunc("/api/history", handleHistory)
	http.HandleFunc("/api/stats", handleStats)
	http.HandleFunc("/api/devices", handleDevices)
	http.HandleFunc("/api/tags", handleTags)
	http.HandleFunc("/api/log/batch/", func(w http.ResponseWriter, r *http.Request) {
		handleBatchLog(hub, w, r)
	})
	http.HandleFunc("/api/log/push/", func(w http.ResponseWriter, r *http.Request) {
		handlePushLog(hub, w, r)
	})

	http.HandleFunc("/log/", func(w http.ResponseWriter, r *http.Request) {
		handleReceiveLog(hub, w, r)
	})

	fmt.Printf("Server started at http://localhost%s\n", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
