package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"
)

func main() {
	logFile := "/Users/laibin/Downloads/log.mrlb.cc.log"
	
	// Ensure file exists
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return
	}
	defer f.Close()

	methods := []string{"GET", "POST", "PUT", "DELETE"}
	paths := []string{"/api/v1/users", "/login", "/status", "/api/v1/products", "/images/avatar.png"}
	statuses := []int{200, 200, 200, 404, 500, 302}


	fmt.Printf("Appending logs into %s...\nPress Ctrl+C to stop.\n", logFile)


	for {
		method := methods[rand.Intn(len(methods))]
		path := paths[rand.Intn(len(paths))]
		status := statuses[rand.Intn(len(statuses))]
		// ua unused in this specific short format
		ip := fmt.Sprintf("192.168.1.%d", rand.Intn(255))
		now := time.Now().Format("02/Jan/2006:15:04:05 -0700")
		
		query := fmt.Sprintf("id=%d&type=test", rand.Intn(100))
		body := "-"
		if method == "POST" {
			body = fmt.Sprintf(`{"userId": %d, "action": "update"}`, rand.Intn(1000))
		}

		// Fixed format to match the parser
		// '$remote_addr - $remote_user [$time_local] "$request" $status GET_ARGS: "$query_string" POST_BODY: "$request_body"'
		logLine := fmt.Sprintf("%s - - [%s] \"%s %s HTTP/1.1\" %d GET_ARGS: \"%s\" POST_BODY: \"%s\"\n", 
			ip, now, method, path, status, query, body)

		if _, err := f.WriteString(logLine); err != nil {
			fmt.Printf("Error writing: %v\n", err)
		}

		// Simulating write interval
		time.Sleep(time.Duration(rand.Intn(900)+100) * time.Millisecond)
	}
}
