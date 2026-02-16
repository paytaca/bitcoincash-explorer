# Bitcoin Cash Explorer (Nuxt)

Explorer UI backed by:

- Bitcoin Cash Node JSON-RPC (blocks/transactions + mempool timestamps)
- Fulcrum (Electrum server) for address indexing (history/balances/tokens)
- BCMR Indexer API (CashTokens metadata enrichment)

## Setup

1) Ensure Node is available (this repo uses `asdf`):

```bash
asdf install
node -v
```

2) Create a `.env.local` file for local development:

```bash
cp .env.local.example .env.local
```

Or create `.env.mainnet` / `.env.chipnet` for deployment configurations:

```bash
cp .env.mainnet.example .env.mainnet
cp .env.chipnet.example .env.chipnet
```

3) Install dependencies.

If your global npm cache has permission issues, you can use a local cache:

```bash
NPM_CONFIG_CACHE="$PWD/.npm-cache" npm install
```

## Run

```bash
NPM_CONFIG_CACHE="$PWD/.npm-cache" npm run dev
```

Open `http://localhost:3000`.

## Required services

This explorer expects the following services to be reachable from the server process:

### Bitcoin Cash Node (BCHN/bitcoind)

Used for:

- Latest blocks/transactions pages
- Transaction details
- Best-effort mempool timestamps (e.g. “seen time”)

Configure via your environment file (`.env.local`, `.env.mainnet`, or `.env.chipnet`):

- `BCH_RPC_URL` (example: `http://127.0.0.1:8332/`)
- `BCH_RPC_USER`
- `BCH_RPC_PASS`

### Fulcrum (Electrum server)

Used for **address pages** (history, BCH balance, and token balances). Bitcoin Cash node does not index address history, so Fulcrum is required for `/address/:address`.

Configure via your environment file:

- `FULCRUM_HOST` (example: `127.0.0.1`)
- `FULCRUM_PORT` (example: `60001`)
- `FULCRUM_TIMEOUT_MS` (default `10000`)

Fulcrum methods used (typical Fulcrum supports all of these):

- `blockchain.headers.subscribe`
- `blockchain.scripthash.get_history`
- `blockchain.scripthash.get_balance`
- `blockchain.scripthash.get_mempool`
- `blockchain.scripthash.listunspent` (token UTXO aggregation where supported)
- `blockchain.transaction.get`
- `blockchain.block.header`
- (optional) `server.version`, `server.banner` (shown on `/status`)

### BCMR Indexer

Used to enrich CashTokens balances/details with metadata (name/symbol/decimals).

Configure via your environment file:

- `BCMR_BASE_URL` (default: `https://bcmr.paytaca.com`)

## Docker (port 8000)

Build:

```bash
docker build -t bch-explorer .
```

Run (maps container port 8000 → host 8000):

```bash
docker run --rm -p 8000:8000 \
  -e BCH_RPC_URL="http://host.docker.internal:8332/" \
  -e BCH_RPC_USER="rpcuser" \
  -e BCH_RPC_PASS="rpcpass" \
  -e FULCRUM_HOST="host.docker.internal" \
  -e FULCRUM_PORT="60001" \
  -e FULCRUM_TIMEOUT_MS="10000" \
  -e BCMR_BASE_URL="https://bcmr.paytaca.com" \
  bch-explorer
```

Open `http://localhost:8000`.

## Environment Files

This project uses explicit environment files for different environments:

- **`.env.local`** - Local development (no deployment config needed)
- **`.env.mainnet`** - Mainnet deployment (includes server config for Fabric)
- **`.env.chipnet`** - Chipnet deployment (includes server config for Fabric)

Example templates are provided:
- `.env.local.example`
- `.env.mainnet.example`  
- `.env.chipnet.example`

## Deployment with Fabric

Deploy to remote servers using Fabric:

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

## Docker Compose

`docker-compose.yml` uses `network_mode: "host"` so the container can reach host-local services bound to `127.0.0.1` (e.g. `bitcoind` RPC and Fulcrum on the same machine). In this mode, `FULCRUM_HOST=127.0.0.1` and `BCH_RPC_URL=http://127.0.0.1:8332/` work as expected.

## Implemented routes

- `/` latest blocks
- `/block/:hash` block details + tx list
- `/tx/:txid` tx details + CashTokens outputs enriched with BCMR metadata
- `/address/:address` address details (tx history, SENT/RECEIVED, BCH balance, token balances)
- `/search?q=...` server-side redirect for txid/address search (works without client-side JS)
- `/status` show BCH node + Fulcrum sync health and connectivity

