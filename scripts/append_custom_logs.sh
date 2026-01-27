#!/bin/bash

LOG_FILE="/Users/laibin/Downloads/log.mrlb.cc.log"

if [ ! -f "$LOG_FILE" ]; then
    echo "Error: Log file not found at $LOG_FILE"
    exit 1
fi

echo "Appending random logs to $LOG_FILE..."
echo "Press Ctrl+C to stop."

METHODS=("GET" "POST" "PUT")
STATUSES=(200 200 200 200 404 500 302)
PATHS=("/api/v1/users" "/login" "/status" "/api/v1/products" "/images/avatar.png")
UAS=("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)" "curl/7.64.1" "Googlebot/2.1")

while true; do
    METHOD=${METHODS[$RANDOM % ${#METHODS[@]}]}
    STATUS=${STATUSES[$RANDOM % ${#STATUSES[@]}]}
    PATH=${PATHS[$RANDOM % ${#PATHS[@]}]}
    UA=${UAS[$RANDOM % ${#UAS[@]}]}
    BYTES=$((RANDOM % 5000))
    IP="192.168.1.$((RANDOM % 255))"
    TIME=$(date +'%d/%b/%Y:%H:%M:%S %z')
    
    # Custom fields
    QUERY="id=$((RANDOM % 100))&type=test"
    BODY=""
    
    if [ "$METHOD" == "POST" ]; then
       BODY="{\"userId\": $((RANDOM % 1000)), \"action\": \"update\"}"
    fi

    # Format matches the user's custom format: 
    # '$remote_addr - $remote_user [$time_local] "$request" $status GET_ARGS: "$query_string" POST_BODY: "$request_body"'
    
    LOG_LINE="$IP - - [$TIME] \"$METHOD $PATH HTTP/1.1\" $STATUS GET_ARGS: \"$QUERY\" POST_BODY: \"$BODY\""
    
    echo "$LOG_LINE" >> "$LOG_FILE"
    
    # Sleep random time between 0.1 and 1s
    SLEEP_TIME=$(awk -v min=0.1 -v max=1.0 'BEGIN{srand(); print min+rand()*(max-min)}')
    sleep "$SLEEP_TIME"
done
