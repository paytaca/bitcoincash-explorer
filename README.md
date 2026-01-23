# Bitcoin Cash Explorer (Nuxt)

Explorer UI backed by:

- Bitcoin Cash Node JSON-RPC (blocks/transactions + CashTokens `tokenData`)
- BCMR Indexer API (token metadata enrichment)

## Setup

1) Ensure Node is available (this repo uses `asdf`):

```bash
asdf install
node -v
```

2) Create a `.env` file (Cursor filters `.env*` from editing tools; use `env.example` as reference):

```bash
cp env.example .env
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

## Implemented routes

- `/` latest blocks
- `/block/:hash` block details + tx list
- `/tx/:txid` tx details + CashTokens outputs enriched with BCMR metadata

