# Build stage for Frontend (Nuxt)
FROM node:20-alpine AS frontend-builder

WORKDIR /app

# Copy package files
COPY package.json package-lock.json ./
RUN npm ci

# Copy frontend source
COPY app/ ./app/
COPY public/ ./public/
COPY nuxt.config.ts tsconfig.json ./

# Build static site
ENV NUXT_PUBLIC_SITE_URL=https://bchexplorer.info
ENV NUXT_PUBLIC_CHAIN=mainnet
RUN npm run generate

# Build stage for API
FROM golang:1.23-alpine AS api-builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git zeromq-dev pkgconfig gcc musl-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY internal/ internal/
COPY cmd/api/ cmd/api/

# Build API binary
RUN CGO_ENABLED=1 go build -o /app/api cmd/api/main.go

# Build stage for ZMQ Listener
FROM golang:1.23-alpine AS zmq-builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git zeromq-dev pkgconfig gcc musl-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY internal/ internal/
COPY cmd/zmq-listener/ cmd/zmq-listener/

# Build ZMQ listener binary
RUN CGO_ENABLED=1 go build -o /app/zmq-listener cmd/zmq-listener/main.go

# Production stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates zeromq

# Copy binaries
COPY --from=api-builder /app/api /usr/local/bin/api
COPY --from=zmq-builder /app/zmq-listener /usr/local/bin/zmq-listener

# Copy built frontend
COPY --from=frontend-builder /app/.output/public /app/public

# Expose port
EXPOSE 8000

# Default command (can be overridden in docker-compose)
CMD ["/usr/local/bin/api"]