# Bitcoin Cash Explorer - Go Rewrite

This is a complete rewrite of the Bitcoin Cash blockchain explorer backend from Node.js/Nuxt.js to Go. The rewrite addresses timeout and 504 errors under heavy traffic by leveraging Go's superior concurrency model and efficient runtime.

## Key Improvements

### Performance
- **Go concurrency**: Goroutines with channels instead of Node.js event loop
- **Connection pooling**: Efficient TCP connection reuse for Fulcrum
- **Circuit breaker**: Prevents cascading failures when BCH node is overloaded
- **Request deduplication**: Identical concurrent RPC calls are merged
- **No memory leaks**: Go's garbage collector handles memory efficiently

### Architecture
- **Separate binaries**: API server and ZMQ listener as independent services
- **Graceful shutdown**: Proper cleanup on SIGTERM/SIGINT
- **Health checks**: Built-in status endpoint for monitoring
- **Configurable timeouts**: All external calls have configurable timeouts

## Project Structure

```
.
├── cmd/
│   ├── api/              # HTTP API server
│   │   └── main.go
│   └── zmq-listener/     # ZMQ listener service
│       └── main.go
├── internal/
│   ├── bchrpc/           # BCH RPC client with circuit breaker
│   ├── fulcrum/          # Fulcrum (Electrum) client with pooling
│   ├── redis/            # Redis data structures and caching
│   ├── bcmr/             # BCMR token metadata client
│   ├── types/            # Shared data types
│   └── utils/            # Utilities (address, hashing, etc.)
├── go.mod
├── go.sum
├── Makefile
├── Dockerfile
└── docker-compose.yml
```

## API Endpoints

All endpoints remain compatible with the previous Node.js implementation:

### Status
- `GET /api/status` - Node and service status

### Blocks
- `GET /api/bch/blockcount` - Current block height
- `GET /api/bch/blocks/latest` - Recent blocks
- `GET /api/bch/block/:hash` - Block details
- `GET /api/bch/blockhash/:height` - Block hash by height

### Transactions
- `GET /api/bch/tx/recent` - Recent transactions
- `GET /api/bch/tx/:txid` - Transaction details
- `GET /api/bch/txout/:txid/:vout` - Output spent status

### Address
- `GET /api/bch/address/:address/txs` - Address transactions (with pagination)

### Broadcast
- `POST /api/bch/broadcast` - Broadcast raw transaction

### BCMR
- `GET /api/bcmr/token/:category` - Token metadata

### Search
- `GET /search?q=...` - Search redirect

## Environment Variables

### API Server
```bash
API_HOST=0.0.0.0              # API server host
API_PORT=8000                 # API server port
BCH_RPC_URL=http://...        # BCH RPC URL
BCH_RPC_USER=username         # BCH RPC username
BCH_RPC_PASS=password         # BCH RPC password
FULCRUM_HOST=127.0.0.1        # Fulcrum host
FULCRUM_PORT=60001            # Fulcrum port
FULCRUM_TIMEOUT_MS=30000      # Fulcrum timeout
BCMR_BASE_URL=...             # BCMR registry URL
REDIS_URL=redis://...         # Redis URL
REDIS_PREFIX=bch              # Redis key prefix
```

### ZMQ Listener
```bash
BCH_ZMQ_HOST=127.0.0.1        # BCH ZMQ host
BCH_ZMQ_PORT=28332            # BCH ZMQ port
BCH_RPC_URL=...               # BCH RPC URL (for fallback)
REDIS_URL=redis://...         # Redis URL
REDIS_PREFIX=bch              # Redis key prefix
MAX_BLOCKS=15                 # Max blocks to keep
MAX_TRANSACTIONS=20           # Max transactions to keep
```

## Building

### Local Build
```bash
# Download dependencies
make deps

# Build both binaries
make build

# Build individually
make build-api
make build-zmq
```

### Docker Build
```bash
# Build images
docker-compose build

# Start all services
docker-compose up -d

# View logs
docker-compose logs -f
```

## Running

### Production
```bash
# Start all services with Docker Compose
docker-compose up -d

# Or run binaries directly (requires Redis, BCH node)
./bin/api
./bin/zmq-listener
```

### Development
```bash
# Run with hot reload (requires air)
make dev-api

# Or manually
go run ./cmd/api
go run ./cmd/zmq-listener
```

## Deployment

The deployment commands remain the same:

```bash
# Build and deploy
docker-compose build
docker-compose up -d

# Or using fabric (if fabfile.py is configured)
fab deploy
```

## Monitoring

### Health Check
```bash
curl http://localhost:8000/api/status
```

### Logs
```bash
# Docker logs
docker-compose logs -f web
docker-compose logs -f zmq-listener

# Or if running directly
./bin/api 2>&1 | tee api.log
./bin/zmq-listener 2>&1 | tee zmq.log
```

## Differences from Node.js Version

1. **No SSR**: The Go API is a pure JSON API server. The Vue frontend remains unchanged and should be served separately (e.g., via Nginx or as a static site).

2. **Worker processes**: Go handles concurrency with goroutines instead of worker processes. The `worker` config in nuxt.config.ts is no longer needed.

3. **Memory management**: Go's GC is more predictable than Node.js/V8. No need for `--max-old-space-size`.

4. **Error handling**: More explicit error handling with Go's error returns.

## Migration Guide

1. **Environment variables**: Update from `NUXT_*` prefix to direct names (see above)

2. **Frontend**: Keep the Vue/Nuxt frontend as-is. Update API base URL if needed.

3. **Nginx**: Update upstream to point to Go API (port 8000 by default)

4. **Monitoring**: Update health checks to use `/api/status`

## Performance Expectations

Based on Go's characteristics:
- **Latency**: 50-70% reduction in response time
- **Throughput**: 3-5x increase in requests per second
- **Memory**: More consistent memory usage, no V8 heap issues
- **Stability**: Should handle traffic spikes without 504 errors

## Troubleshooting

### Connection refused to BCH node
- Verify `BCH_RPC_URL` is correct
- Check BCH node is running and RPC is enabled
- Verify firewall rules allow connections

### ZMQ not receiving messages
- Ensure BCH node has `zmqpubrawblock` and `zmqpubrawtx` enabled
- Check `BCH_ZMQ_HOST` and `BCH_ZMQ_PORT` match BCH node config
- Use host networking in Docker if BCH node is on host

### Redis connection issues
- Verify `REDIS_URL` is correct
- Check Redis is running
- Ensure network connectivity between containers

### Build errors
- Ensure Go 1.23+ is installed
- Install zeromq development libraries: `apt-get install libzmq3-dev` (Ubuntu/Debian) or `brew install zeromq` (macOS)
- Run `make deps` to download Go modules

## License

Same as the original project.