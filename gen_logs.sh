#!/bin/bash
while true; do
  echo "127.0.0.1 - - [$(date +'%d/%b/%Y:%H:%M:%S %z')] \"GET /api/data HTTP/1.1\" 200 $((RANDOM%5000)) \"-\" \"Mozilla/5.0\"" >> access.log
  sleep 1
done
