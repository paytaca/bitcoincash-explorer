# Bitcoin Cash Explorer

A Bitcoin Cash blockchain explorer with Go backend and Nuxt frontend.

## Architecture

This project uses a clean separation between backend and frontend:

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

## Setup

### Prerequisites

- Go 1.23+
- Node.js 20+
- Redis
- Bitcoin Cash Node (BCHN)
- Fulcrum Electrum server

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
PUBLIC_CHAIN=mainnet
```

## Development

### Build

Build both binaries:

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

### Using Make

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

See the Go backend code in `cmd/api/main.go` for full API documentation.

## License

MIT
