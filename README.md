# Bitcoin Cash Explorer

A high-performance Bitcoin Cash blockchain explorer built with Go and Nuxt.js.

## Architecture

The project uses a clean separation between backend and frontend:

- **Go Backend** (`cmd/api/`): REST API server handling all blockchain data
- **Go ZMQ Listener** (`cmd/zmq-listener/`): Processes real-time blocks and transactions
- **Nuxt Frontend** (`app/`): Static SPA (Single Page Application)

### Backend Services

The Go backend provides:

- `/api/status` - Node health and sync status
- `/api/bch/blockcount` - Current block height
- `/api/bch/blocks/latest` - Recent blocks list
- `/api/bch/block/:hash` - Block details
- `/api/bch/tx/recent` - Recent transactions (mempool + confirmed)
- `/api/bch/tx/:txid` - Transaction details
- `/api/bch/address/:address/txs` - Address history and balances
- `/api/bch/broadcast` - Broadcast raw transactions
- `/api/bcmr/token/:category` - CashTokens metadata
- `/search?q=...` - Search redirect endpoint

### Data Sources

- **Bitcoin Cash Node JSON-RPC**: Blocks, transactions, mempool
- **Fulcrum (Electrum server)**: Address indexing (history, balances, tokens)
- **BCMR Indexer API**: CashTokens metadata enrichment
- **Redis**: Cache for blocks, transactions, and token metadata

### Key Features

- **High Performance**: Go concurrency with goroutines for efficient request handling
- **Connection Pooling**: Efficient TCP connection reuse for Fulcrum
- **Circuit Breaker**: Prevents cascading failures when BCH node is overloaded
- **Request Deduplication**: Identical concurrent RPC calls are merged
- **Graceful Shutdown**: Proper cleanup on SIGTERM/SIGINT
- **Health Checks**: Built-in status endpoint for monitoring

## Prerequisites

- Go 1.23+
- Node.js 20+
- Redis
- Bitcoin Cash Node (BCHN)
- Fulcrum Electrum server

## Setup

### Environment Configuration

Create an environment file:

```bash
# For local development
cp .env.local.example .env.local

# For production deployment
cp .env.mainnet.example .env.mainnet
cp .env.chipnet.example .env.chipnet
```

Required environment variables:

```bash
# BCH Node RPC
BCH_RPC_URL=http://127.0.0.1:8332/
BCH_RPC_USER=rpcuser
BCH_RPC_PASS=rpcpass

# Fulcrum (Electrum server)
FULCRUM_HOST=127.0.0.1
FULCRUM_PORT=60001
FULCRUM_TIMEOUT_MS=30000

# Redis
REDIS_URL=redis://127.0.0.1:6379/0
REDIS_PREFIX=bch

# BCMR (CashTokens metadata)
BCMR_BASE_URL=https://bcmr.paytaca.com

# Chain configuration
CHAIN=mainnet
MAINNET_URL=https://bchexplorer.info
CHIPNET_URL=https://chipnet.bchexplorer.info
```

## Development

### Build

Build all binaries:

```bash
make build
```

This creates:
- `bin/api` - API server
- `bin/zmq-listener` - ZMQ listener

### Run Locally

Start Redis:
```bash
docker run -d -p 127.0.0.1:6379:6379 redis:7-alpine
```

Run the API server:
```bash
make run-api
# or
./bin/api
```

Run the ZMQ listener (in another terminal):
```bash
make run-zmq
# or
./bin/zmq-listener
```

Build and serve the frontend:
```bash
npm install
npm run generate
npx serve .output/public
```

### Development with Hot Reload

```bash
# API server with hot reload (requires air)
make dev-api

# Or run directly
go run ./cmd/api
go run ./cmd/zmq-listener
```

## Docker

### Build Images

```bash
make docker-build
# or
docker-compose build
```

### Run with Docker Compose

```bash
docker-compose up -d
```

This starts:
- Redis on port 6379
- ZMQ listener (host networking for ZMQ)
- Web server (API + static files) on port 8000

### Manual Docker Run

```bash
# Build the image
docker build -t bch-explorer .

# Run API server
docker run --rm -p 8000:8000 \
  -e BCH_RPC_URL="http://host.docker.internal:8332/" \
  -e BCH_RPC_USER="rpcuser" \
  -e BCH_RPC_PASS="rpcpass" \
  -e FULCRUM_HOST="host.docker.internal" \
  -e FULCRUM_PORT="60001" \
  -e REDIS_URL="redis://host.docker.internal:6379/0" \
  bch-explorer

# Run ZMQ listener
docker run --rm --network host \
  -e BCH_ZMQ_HOST="127.0.0.1" \
  -e BCH_ZMQ_PORT="28332" \
  -e BCH_RPC_URL="http://127.0.0.1:8332/" \
  -e REDIS_URL="redis://127.0.0.1:6379/0" \
  bch-explorer /usr/local/bin/zmq-listener
```

## Deployment

### Using Fabric

```bash
# Deploy to mainnet
fab mainnet deploy

# Deploy to chipnet
fab chipnet deploy

# Check status
fab mainnet status
fab mainnet logs
```

Requires `SERVER_HOSTNAME` and `SERVER_USER` in your `.env.mainnet` or `.env.chipnet` file.

### Manual Deployment

```bash
# Build and deploy
docker-compose build
docker-compose up -d

# View logs
docker-compose logs -f
```

## Project Structure

```
.
├── app/                    # Nuxt frontend (SPA)
│   ├── components/         # Vue components
│   ├── composables/        # Vue composables
│   ├── pages/             # Nuxt pages
│   └── utils/             # Frontend utilities
├── cmd/                   # Go applications
│   ├── api/               # REST API server
│   └── zmq-listener/      # ZMQ block/tx processor
├── internal/              # Go internal packages
│   ├── bchrpc/            # BCH RPC client
│   ├── bcmr/              # BCMR API client
│   ├── fulcrum/           # Fulcrum Electrum client
│   ├── redis/             # Redis client
│   ├── types/             # Shared types
│   └── utils/             # Utilities
├── public/                # Static assets
├── Dockerfile             # Main API image
├── Dockerfile.zmq         # ZMQ listener image
├── docker-compose.yml     # Local development
├── fabfile.py            # Deployment automation
└── Makefile              # Build automation
```

## Available Routes

- `/` - Latest blocks and transactions
- `/block/:hash` - Block details with transaction list
- `/tx/:txid` - Transaction details with CashTokens metadata
- `/address/:address` - Address history, balances, and tokens
- `/status` - Node health and sync status
- `/broadcast` - Broadcast raw transactions

## API Endpoints

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

## Performance

The Go implementation provides:
- **Latency**: 50-70% reduction in response time
- **Throughput**: 3-5x increase in requests per second
- **Memory**: More consistent memory usage
- **Stability**: Handles traffic spikes without errors

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

MIT
