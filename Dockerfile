# --- Stage 1: Build the React Frontend SPA ---
FROM node:20-alpine AS frontend-builder
WORKDIR /build/frontend

# Copy frontend source files
COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

# --- Stage 2: Build the Go Backend via gödel ---
FROM golang:1.25-alpine AS backend-builder
ARG TARGETARCH
WORKDIR /build

# Install build-time dependencies required by godelw
RUN apk add --no-cache bash curl git tar

# Copy go mod and dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy godel wrapper and config
COPY godelw ./
COPY godel/ ./godel/
RUN chmod +x godelw

# Copy source files
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Build with gödel for the target platform only
# CGO_ENABLED=0 ensures fully static binaries
RUN ./godelw build --os-arch linux-${TARGETARCH}

# Locate the linux binary matching this build's architecture and copy it to
# a predictable path so Stage 3 can COPY from a fixed, unambiguous location.
RUN cp out/build/qalibre/*/linux-${TARGETARCH}/qalibre qalibre

# --- Stage 3: Create the final runtime container ---
FROM debian:bookworm-slim AS runtime
WORKDIR /app

# Install runtime dependencies: ImageMagick (thumbnail generation) + p7zip (comic books) + ca-certificates (HTTPS) + poppler-utils (PDF text extraction) + python3 + pip
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    imagemagick \
    p7zip-full \
    poppler-utils \
    python3 \
    python3-pip \
    python3-venv \
    && rm -rf /var/lib/apt/lists/*

# Install uv from official image
COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/

# Create a virtual environment and configure PATH to bypass PEP 668
RUN python3 -m venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"

# Install pymupdf4llm inside the virtual environment using uv
RUN uv pip install pymupdf4llm

# Copy the linux binary produced in Stage 2 for the target architecture.
# Stage 2 cp'd it to /build/qalibre so this path is stable and unambiguous.
COPY --from=backend-builder /build/qalibre /app/qalibre

# Copy built frontend assets and public assets from frontend-builder stage
COPY --from=frontend-builder /build/frontend/dist /app/frontend/dist
COPY --from=frontend-builder /build/frontend/public /app/frontend/public

# Create directory for persistent data (settings, users, datasets app.db)
RUN mkdir -p /data
VOLUME ["/data"]

# Expose Qalibre port
EXPOSE 8083

# Run Qalibre server
ENTRYPOINT ["/app/qalibre"]