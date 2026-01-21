# Stage 1: Build Backend
FROM golang:1.23-alpine AS build-backend
WORKDIR /app
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o nginx-log-viewer .

# Stage 2: Final Image
FROM alpine:latest
WORKDIR /app
RUN apk --no-cache add ca-certificates

COPY --from=build-backend /app/nginx-log-viewer .
COPY frontend/dist ./frontend/dist

EXPOSE 58080
CMD ["./nginx-log-viewer"]
