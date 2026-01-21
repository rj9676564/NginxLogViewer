# Stage 1: Build Frontend
FROM node:20-alpine AS frontend-builder

WORKDIR /app
COPY frontend/package*.json ./frontend/
RUN cd frontend && npm install

COPY frontend ./frontend
RUN cd frontend && npm run build

# Stage 2: Build Backend
FROM golang:1.23-alpine AS backend-builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG TARGETOS
ARG TARGETARCH
ARG BUILD_TIME
ARG GIT_COMMIT
ARG VERSION
RUN CGO_ENABLED=0 \
    GOOS=${TARGETOS:-linux} \
    GOARCH=${TARGETARCH:-amd64} \
    go build -ldflags="-s -w -X 'main.Version=${VERSION}' -X 'main.BuildTime=${BUILD_TIME}' -X 'main.GitCommit=${GIT_COMMIT}'" \
    -o /out/nginx-log-viewer .

# Stage 3: Final Runtime Image
FROM alpine:latest

WORKDIR /app
ARG BUILD_TIME
ARG GIT_COMMIT
ARG VERSION

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S appuser \
    && adduser -S appuser -G appuser

COPY --from=backend-builder /out/nginx-log-viewer /app/nginx-log-viewer
COPY --from=frontend-builder /app/frontend/dist /app/frontend/dist

RUN chown -R appuser:appuser /app

USER appuser

LABEL org.opencontainers.image.title="nginx-log-viewer" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${GIT_COMMIT}" \
      org.opencontainers.image.created="${BUILD_TIME}"

EXPOSE 58080
CMD ["/app/nginx-log-viewer"]
