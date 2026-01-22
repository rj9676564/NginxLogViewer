# Stage 1: Build Frontend
FROM node:20-alpine AS frontend-builder

# Install pnpm
ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
RUN corepack enable && corepack prepare pnpm@latest --activate

WORKDIR /app/frontend

# Copy package.json and pnpm-lock.yaml (if exists) first
COPY frontend/package.json ./
COPY frontend/pnpm-lock.yaml* ./

# Install dependencies using a cache mount for the pnpm store
# This makes subsequent builds incredibly fast
RUN --mount=type=cache,id=pnpm,target=/pnpm/store \
    pnpm install --frozen-lockfile || pnpm install

# Copy source files
COPY frontend ./

# Build frontend
RUN pnpm run build

# Stage 2: Build Backend
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS backend-builder

WORKDIR /src

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source code
COPY *.go ./

# Build arguments provided by Docker Buildx
ARG TARGETOS
ARG TARGETARCH
ARG BUILD_TIME
ARG GIT_COMMIT
ARG VERSION

# Build with native speed using Go's cross-compilation
# --platform=$BUILDPLATFORM ensures the compiler runs on the host architecture (fast)
# GOARCH=$TARGETARCH tells Go which architecture to target
RUN CGO_ENABLED=0 \
    GOOS=$TARGETOS \
    GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X 'main.Version=${VERSION}' -X 'main.BuildTime=${BUILD_TIME}' -X 'main.GitCommit=${GIT_COMMIT}'" \
    -o /out/nginx-log-viewer .

# Stage 3: Final Runtime Image
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies in one layer
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S appuser && \
    adduser -S appuser -G appuser

# Copy binaries and assets
COPY --from=backend-builder /out/nginx-log-viewer /app/nginx-log-viewer
COPY --from=frontend-builder /app/frontend/dist /app/frontend/dist

# Set ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Build arguments for labels
ARG BUILD_TIME
ARG GIT_COMMIT
ARG VERSION

# Metadata
LABEL org.opencontainers.image.title="nginx-log-viewer" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${GIT_COMMIT}" \
      org.opencontainers.image.created="${BUILD_TIME}"

EXPOSE 58080

CMD ["/app/nginx-log-viewer"]
