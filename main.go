package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
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
	formatStr  = flag.String("format", "", "Nginx log format string")
	configPath = flag.String("config", "", "path to config.json file")
)

type Config struct {
	Addr      string `json:"addr"`
	LogFile   string `json:"log_file"`
	DBPath    string `json:"db_path"`
	LogFormat string `json:"log_format"`
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

var (
	// logRegex will be initialized at runtime
	logRegex *regexp.Regexp
	// Fallback for standard combined if needed
	simpleRegex = regexp.MustCompile(`^(?P<ip>\S+) - \S+ \[(?P<time>[^\]]+)\] "(?P<request>[^"]+)" (?P<status>\d+) (?P<bytes>\d+) "-" "(?P<ua>[^"]+)"`)
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

// buildRegexFromNginx converts an Nginx log_format string into a regular expression
func buildRegexFromNginx(format string) *regexp.Regexp {
	// Normalize format: remove newlines and collapse multiple spaces
	format = strings.ReplaceAll(format, "\n", " ")
	format = strings.ReplaceAll(format, "\r", " ")
	reSpace := regexp.MustCompile(`\s+`)
	format = reSpace.ReplaceAllString(format, " ")
	format = strings.TrimSpace(format)

	// Reference mapping: nginx variable -> regex named group
	replacements := map[string]string{
		"$remote_addr":     `(?P<ip>\S+)`,
		"$remote_user":     `(?P<user>\S*)`,
		"$time_local":      `(?P<time>[^\]]+)`,
		"$request":         `(?P<method>\S+)\s+(?P<path>\S+)\s+(?P<proto>[^"]*)`,
		"$status":          `(?P<status>\d+)`,
		"$body_bytes_sent": `(?P<bytes>\d*)`,
		"$http_referer":    `(?P<referer>[^"]*)`,
		"$http_user_agent": `(?P<ua>[^"]*)`,
		"$query_string":    `(?P<query>[^"]*)`,
		"$request_body":    `(?P<body>.*)`,
	}

	// 1. Escape regex special characters from the format string
	res := regexp.QuoteMeta(format)

	// 2. Handle nginx variables (convert back from quoted \$ to group)
	for k, v := range replacements {
		escapedVar := regexp.QuoteMeta(k)
		res = strings.ReplaceAll(res, escapedVar, v)
	}

	// 3. Handle flexible whitespace: any space in format can match one or more spaces
	res = strings.ReplaceAll(res, `\ `, `\s+`)
	res = strings.ReplaceAll(res, ` `, `\s+`)

	// Final cleanup and ensure it matches the whole line but is robust to trailing junk
	return regexp.MustCompile("^" + res + `(?:\s*.*)?$`)
}

func parseLine(line string) *LogEntry {
	entry := &LogEntry{
		Raw:       line,
		CreatedAt: time.Now().Unix(),
		Path:      "-", // Default values so frontend shows something
		Method:    "LOG",
		Status:    200,
	}

	// Try specific format first
	if matches := logRegex.FindStringSubmatch(line); matches != nil {
		names := logRegex.SubexpNames()
		for i, match := range matches {
			if match == "" { continue }
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
			case "bytes":
				fmt.Sscanf(match, "%d", &entry.Bytes)
			case "ua":
				entry.UA = match
			}
		}
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
	} else {
		// If both fail, try to at least find an IP at the start
		ipMatch := regexp.MustCompile(`^(\S+)`).FindStringSubmatch(line)
		if len(ipMatch) > 1 {
			entry.IP = ipMatch[1]
		}
		log.Printf("[Parse Error] Line did not match any format: %s", line)
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

	// 1. Start with defaults
	finalAddr := ":58080"
	finalLogFile := "/var/log/nginx/access.log"
	finalDBPath := "./logs.db"
	finalFormat := `$remote_addr - $remote_user [$time_local] "$request" $status GET_ARGS: "$query_string" POST_BODY: "$request_body"`

	// 2. Override with Config File if provided
	if *configPath != "" {
		if cfg, err := loadConfig(*configPath); err == nil {
			if cfg.Addr != "" { finalAddr = cfg.Addr }
			if cfg.LogFile != "" { finalLogFile = cfg.LogFile }
			if cfg.DBPath != "" { finalDBPath = cfg.DBPath }
			if cfg.LogFormat != "" { finalFormat = cfg.LogFormat }
			log.Printf("Loaded config from %s", *configPath)
		} else {
			log.Printf("Warning: Failed to load config from %s: %v", *configPath, err)
		}
	}

	// 3. Override with Environment Variables
	finalAddr = getEnv("LISTEN_ADDR", finalAddr)
	finalLogFile = getEnv("LOG_FILE", finalLogFile)
	finalDBPath = getEnv("DB_PATH", finalDBPath)
	finalFormat = getEnv("LOG_FORMAT", finalFormat)

	// 4. Override with Command line flags (if they are NOT default values)
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "addr": finalAddr = *addr
		case "file": finalLogFile = *logFile
		case "db": finalDBPath = *dbPath
		case "format": finalFormat = *formatStr
		}
	})

	// Apply final values
	*addr = finalAddr
	*logFile = finalLogFile
	*dbPath = finalDBPath
	*formatStr = finalFormat

	log.Printf("Using log format: %s", *formatStr)
	logRegex = buildRegexFromNginx(*formatStr)

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
