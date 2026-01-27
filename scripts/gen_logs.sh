#!/bin/bash
while true; do
  # echo "127.0.0.1 - - [$(date +'%d/%b/%Y:%H:%M:%S %z')] \"GET /api/data HTTP/1.1\" 200 $((RANDOM%5000)) \"-\" \"Mozilla/5.0\"" >> access.log
  echo '223.104.68.151 -  [20/Jan/2026:19:28:07 -0800] "POST /log?level=d&tag=unknow HTTP/1.1" 500 GET_ARGS: "level=d&tag=unknow" POST_BODY: "{\"text\":\"homeAdvs == 15\",\"name\":\"\"}"' >> /Users/laibin/Downloads/log.mrlb.cc.log
  sleep 1
done
