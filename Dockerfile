# --- Stage 1: Build the React Frontend SPA ---
FROM node:20-alpine AS frontend-builder
WORKDIR /build/frontend

# Copy frontend source files
COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

# --- Stage 2: Build the Go Backend ---
FROM golang:1.25-alpine AS backend-builder
WORKDIR /build

# Copy go mod and dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy backend source files
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Build static Go binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o qalibre ./cmd/qalibre

# --- Stage 3: Create the final runtime container ---
FROM alpine:3.18 AS runtime
WORKDIR /app

# Install runtime dependencies: ImageMagick (thumbnail generation) + p7zip (comic books) + ca-certificates (HTTPS)
RUN apk add --no-cache \
    ca-certificates \
    imagemagick \
    p7zip

# Copy Go binary from backend-builder
COPY --from=backend-builder /build/qalibre /app/qalibre

# Copy built frontend assets and public assets from frontend-builder stage
COPY --from=frontend-builder /build/frontend/dist /app/frontend/dist
COPY --from=frontend-builder /build/frontend/public /app/frontend/public

# Expose Qalibre port
EXPOSE 8083

# Run Qalibre server
ENTRYPOINT ["/app/qalibre"]
