# Stage 1: Build Frontend
FROM node:18-alpine AS build-frontend
WORKDIR /app
COPY frontend/package*.json ./
RUN npm install
COPY frontend/ .
RUN npm run build

# Stage 2: Build Backend
FROM golang:1.23-alpine AS build-backend
WORKDIR /app
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o nginx-log-viewer .

# Stage 3: Final Image
FROM alpine:latest
WORKDIR /app
RUN apk --no-cache add ca-certificates

COPY --from=build-backend /app/nginx-log-viewer .
COPY --from=build-frontend /app/dist ./frontend/dist

EXPOSE 58080
CMD ["./nginx-log-viewer"]
