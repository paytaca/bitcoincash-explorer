package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"bchexplorer/internal/bchrpc"
	"bchexplorer/internal/bcmr"
	"bchexplorer/internal/fulcrum"
	"bchexplorer/internal/redis"
	"bchexplorer/internal/types"
	"bchexplorer/internal/utils"
)

// Config holds server configuration
type Config struct {
	Host           string
	Port           int
	RedisURL       string
	RedisPrefix    string
	BCHRPCURL      string
	BCHRPCUser     string
	BCHRPCPass     string
	FulcrumHost    string
	FulcrumPort    int
	FulcrumTimeout int
	BCMRBaseURL    string
	PublicChain    string
	MainnetURL     string
	ChipnetURL     string
}

// Server represents the HTTP API server
type Server struct {
	config  Config
	router  *gin.Engine
	redis   *redis.Client
	rpc     *bchrpc.Client
	fulcrum *fulcrum.Client
	bcmr    *bcmr.Client
	pools   *MiningPools
}

// MiningPool represents a mining pool configuration
type MiningPool struct {
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
	Tags      []string `json:"tags"`
	Urls      []string `json:"urls"`
}

// MiningPools holds all mining pool configurations
type MiningPools struct {
	Pools []MiningPool
}

// loadMiningPools loads mining pool configurations from pools.json
func loadMiningPools() (*MiningPools, error) {
	// Try multiple paths
	paths := []string{"pools.json", "/app/pools.json", "./pools.json"}
	var data []byte
	var err error
	var usedPath string

	for _, path := range paths {
		data, err = os.ReadFile(path)
		if err == nil {
			usedPath = path
			break
		}
	}

	if data == nil {
		return nil, fmt.Errorf("failed to read pools.json from any path: %v", err)
	}

	log.Printf("[POOLS] Loaded pools.json from: %s (%d bytes)", usedPath, len(data))

	var pools MiningPools
	if err := json.Unmarshal(data, &pools); err != nil {
		return nil, fmt.Errorf("failed to parse pools.json: %w", err)
	}

	log.Printf("[POOLS] Successfully loaded %d pools", len(pools.Pools))
	for i, p := range pools.Pools {
		if i < 5 {
			log.Printf("[POOLS] Pool %d: %s with %d tags", i, p.Name, len(p.Tags))
		}
	}

	return &pools, nil
}

// loadConfig loads configuration from environment
func loadConfig() Config {
	return Config{
		Host:           getEnv("API_HOST", "0.0.0.0"),
		Port:           getEnvInt("API_PORT", 8000),
		RedisURL:       getEnv("REDIS_URL", "redis://127.0.0.1:6379/0"),
		RedisPrefix:    getEnv("REDIS_PREFIX", "bch"),
		BCHRPCURL:      getEnv("BCH_RPC_URL", ""),
		BCHRPCUser:     getEnv("BCH_RPC_USER", ""),
		BCHRPCPass:     getEnv("BCH_RPC_PASS", ""),
		FulcrumHost:    getEnv("FULCRUM_HOST", "127.0.0.1"),
		FulcrumPort:    getEnvInt("FULCRUM_PORT", 60001),
		FulcrumTimeout: getEnvInt("FULCRUM_TIMEOUT_MS", 30000),
		BCMRBaseURL:    getEnv("BCMR_BASE_URL", "https://bcmr.paytaca.com"),
		PublicChain:    getEnv("PUBLIC_CHAIN", "mainnet"),
		MainnetURL:     getEnv("MAINNET_URL", ""),
		ChipnetURL:     getEnv("CHIPNET_URL", ""),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return defaultVal
}

// NewServer creates a new HTTP API server
func NewServer(cfg Config) (*Server, error) {
	// Initialize Redis
	redisClient, err := redis.NewClient(redis.Config{
		URL:    cfg.RedisURL,
		Prefix: cfg.RedisPrefix,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Initialize BCH RPC client
	var rpcClient *bchrpc.Client
	if cfg.BCHRPCURL != "" {
		rpcClient = bchrpc.NewClient(bchrpc.Config{
			URL:      cfg.BCHRPCURL,
			Username: cfg.BCHRPCUser,
			Password: cfg.BCHRPCPass,
			Timeout:  30 * time.Second,
			MaxConns: 20,
		})
	}

	// Initialize Fulcrum client
	fulcrumClient := fulcrum.NewClient(fulcrum.Config{
		Host:     cfg.FulcrumHost,
		Port:     cfg.FulcrumPort,
		Timeout:  time.Duration(cfg.FulcrumTimeout) * time.Millisecond,
		MaxConns: 100,
	})

	// Initialize BCMR client
	bcmrClient := bcmr.NewClient(bcmr.Config{
		BaseURL: cfg.BCMRBaseURL,
		Timeout: 15 * time.Second,
	})

	// Load mining pools configuration
	pools, err := loadMiningPools()
	if err != nil {
		log.Printf("[POOLS] ERROR: Failed to load mining pools: %v", err)
		pools = &MiningPools{Pools: []MiningPool{}}
	} else {
		log.Printf("[POOLS] Successfully initialized with %d mining pools", len(pools.Pools))
	}

	// Setup router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())
	router.Use(requestTiming())

	s := &Server{
		config:  cfg,
		router:  router,
		redis:   redisClient,
		rpc:     rpcClient,
		fulcrum: fulcrumClient,
		bcmr:    bcmrClient,
		pools:   pools,
	}

	s.setupRoutes()

	return s, nil
}

// corsMiddleware handles CORS
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// requestTiming logs slow requests
func requestTiming() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		duration := time.Since(start)
		if duration > 5*time.Second {
			log.Printf("SLOW REQUEST: %s %s took %v", c.Request.Method, c.Request.URL.Path, duration)
		}

		if len(c.Errors) > 0 {
			log.Printf("ERROR: %s %s - %v", c.Request.Method, c.Request.URL.Path, c.Errors)
		}
	}
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Health check
	s.router.GET("/api/status", s.handleStatus)

	// Blocks
	s.router.GET("/api/bch/blockcount", s.handleBlockCount)
	s.router.GET("/api/bch/blocks/latest", s.handleLatestBlocks)
	s.router.GET("/api/bch/block/:hash", s.handleBlock)
	s.router.GET("/api/bch/blockhash/:height", s.handleBlockHash)

	// Transactions
	s.router.GET("/api/bch/tx/recent", s.handleRecentTransactions)
	s.router.GET("/api/bch/tx/:txid", s.handleTransaction)
	s.router.GET("/api/bch/txout/:txid/:vout", s.handleTxOut)

	// Address
	s.router.GET("/api/bch/address/:address/txs", s.handleAddressTransactions)

	// Broadcast
	s.router.POST("/api/bch/broadcast", s.handleBroadcast)

	// BCMR
	s.router.GET("/api/bcmr/token/:category", s.handleTokenMetadata)

	// Search
	s.router.GET("/search", s.handleSearch)

	// Static files - serve frontend
	s.router.Static("/_nuxt", "/app/public/_nuxt")
	s.router.Static("/favicon", "/app/public/favicon")
	s.router.StaticFile("/apple-touch-icon.png", "/app/public/apple-touch-icon.png")
	s.router.StaticFile("/apple-touch-icon-precomposed.png", "/app/public/apple-touch-icon-precomposed.png")
	s.router.StaticFile("/logo.svg", "/app/public/logo.svg")
	s.router.StaticFile("/og-image.png", "/app/public/og-image.png")
	s.router.StaticFile("/og-image.svg", "/app/public/og-image.svg")
	s.router.StaticFile("/robots.txt", "/app/public/robots.txt")
	s.router.StaticFile("/theme-init.js", "/app/public/theme-init.js")

	// SPA fallback - serve index.html for all non-API routes
	s.router.NoRoute(func(c *gin.Context) {
		// Don't serve index.html for API routes
		if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
			return
		}
		c.File("/app/public/index.html")
	})
}

// StatusResponse matches the frontend's expected format
type StatusResponse struct {
	GeneratedAt int64            `json:"generatedAt"`
	Node        NodeStatus       `json:"node"`
	Fulcrum     FulcrumStatus    `json:"fulcrum"`
	Comparison  ComparisonStatus `json:"comparison"`
}

type NodeStatus struct {
	Ok                   bool    `json:"ok"`
	Error                string  `json:"error,omitempty"`
	LatencyMs            int64   `json:"latencyMs,omitempty"`
	Chain                string  `json:"chain,omitempty"`
	Blocks               int64   `json:"blocks,omitempty"`
	Headers              int64   `json:"headers,omitempty"`
	Bestblockhash        string  `json:"bestblockhash,omitempty"`
	Difficulty           float64 `json:"difficulty,omitempty"`
	Verificationprogress float64 `json:"verificationprogress,omitempty"`
	Initialblockdownload bool    `json:"initialblockdownload,omitempty"`
	Mediantime           int64   `json:"mediantime,omitempty"`
	Warnings             string  `json:"warnings,omitempty"`
	Version              int64   `json:"version,omitempty"`
	Subversion           string  `json:"subversion,omitempty"`
	Connections          int64   `json:"connections,omitempty"`
	BestBlockTime        int64   `json:"bestBlockTime,omitempty"`
}

type FulcrumStatus struct {
	Ok         bool        `json:"ok"`
	Error      string      `json:"error,omitempty"`
	LatencyMs  int64       `json:"latencyMs,omitempty"`
	Host       string      `json:"host,omitempty"`
	Port       int         `json:"port,omitempty"`
	Version    interface{} `json:"version,omitempty"`
	Banner     interface{} `json:"banner,omitempty"`
	Height     int64       `json:"height,omitempty"`
	HeaderTime int64       `json:"headerTime,omitempty"`
}

type ComparisonStatus struct {
	HeightDiff      int64 `json:"heightDiff"`
	TimeDiffSeconds int64 `json:"timeDiffSeconds"`
	InSync          bool  `json:"inSync"`
}

// headerTimeFromHex extracts timestamp from block header hex (80 byte header, timestamp at offset 68)
func headerTimeFromHex(headerHex string) int64 {
	if len(headerHex) < 160 {
		return 0
	}
	// Timestamp is 4 bytes LE at offset 68 (136 hex chars)
	tsBytes := headerHex[136:144]
	// Parse as little-endian uint32
	b, _ := hex.DecodeString(tsBytes)
	if len(b) != 4 {
		return 0
	}
	return int64(uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24)
}

// handleStatus returns node status
func (s *Server) handleStatus(c *gin.Context) {
	ctx := c.Request.Context()
	generatedAt := time.Now().Unix()

	node := NodeStatus{Ok: false}
	fulcrum := FulcrumStatus{Ok: false, Host: s.config.FulcrumHost, Port: s.config.FulcrumPort}

	// Check BCH node
	if s.rpc != nil {
		t0 := time.Now()
		chainInfo, err := s.rpc.GetBlockchainInfo(ctx)
		if err == nil && chainInfo != nil {
			netInfo, _ := s.rpc.GetNetworkInfo(ctx)

			node.LatencyMs = time.Since(t0).Milliseconds()
			node.Ok = true

			if v, ok := chainInfo["chain"].(string); ok {
				node.Chain = v
			}
			if v, ok := chainInfo["blocks"].(float64); ok {
				node.Blocks = int64(v)
			}
			if v, ok := chainInfo["headers"].(float64); ok {
				node.Headers = int64(v)
			}
			if v, ok := chainInfo["bestblockhash"].(string); ok {
				node.Bestblockhash = v
			}
			if v, ok := chainInfo["difficulty"].(float64); ok {
				node.Difficulty = v
			}
			if v, ok := chainInfo["verificationprogress"].(float64); ok {
				node.Verificationprogress = v
			}
			if v, ok := chainInfo["initialblockdownload"].(bool); ok {
				node.Initialblockdownload = v
			}
			if v, ok := chainInfo["mediantime"].(float64); ok {
				node.Mediantime = int64(v)
			}
			if v, ok := chainInfo["warnings"].(string); ok {
				node.Warnings = v
			}
			if netInfo != nil {
				if v, ok := netInfo["version"].(float64); ok {
					node.Version = int64(v)
				}
				if v, ok := netInfo["subversion"].(string); ok {
					node.Subversion = v
				}
				if v, ok := netInfo["connections"].(float64); ok {
					node.Connections = int64(v)
				}
			}

			// Get best block time
			if node.Bestblockhash != "" {
				if block, err := s.rpc.GetBlock(ctx, node.Bestblockhash, 1); err == nil {
					if v, ok := block["time"].(float64); ok {
						node.BestBlockTime = int64(v)
					}
				}
			}
		} else {
			node.Error = err.Error()
		}
	} else {
		node.Error = "RPC not configured"
	}

	// Check Fulcrum
	t0 := time.Now()
	if s.fulcrum != nil {
		// Get tip via headers.subscribe
		var tip map[string]interface{}
		err := s.fulcrum.Request(ctx, "blockchain.headers.subscribe", []interface{}{}, &tip)
		if err == nil && tip != nil {
			if v, ok := tip["height"].(float64); ok {
				fulcrum.Height = int64(v)
			}

			// Get version
			var version []interface{}
			if err := s.fulcrum.Request(ctx, "server.version", []interface{}{"bchexplorer", "1.4"}, &version); err == nil {
				fulcrum.Version = version
			}

			// Get banner
			var banner string
			if err := s.fulcrum.Request(ctx, "server.banner", []interface{}{}, &banner); err == nil {
				fulcrum.Banner = banner
			}

			// Get header time
			if fulcrum.Height > 0 {
				var headerHex string
				if err := s.fulcrum.Request(ctx, "blockchain.block.header", []interface{}{fulcrum.Height}, &headerHex); err == nil {
					fulcrum.HeaderTime = headerTimeFromHex(headerHex)
				}
			}

			fulcrum.LatencyMs = time.Since(t0).Milliseconds()
			fulcrum.Ok = true
		} else {
			if err != nil {
				fulcrum.Error = err.Error()
			} else {
				fulcrum.Error = "No response"
			}
		}
	} else {
		fulcrum.Error = "Fulcrum not configured"
	}

	// Calculate comparison
	var heightDiff, timeDiff int64
	if node.Blocks > 0 && fulcrum.Height > 0 {
		heightDiff = node.Blocks - fulcrum.Height
	}
	if node.BestBlockTime > 0 && fulcrum.HeaderTime > 0 {
		timeDiff = node.BestBlockTime - fulcrum.HeaderTime
	}

	inSync := node.Ok && fulcrum.Ok && heightDiff >= 0 && heightDiff <= 1

	response := StatusResponse{
		GeneratedAt: generatedAt,
		Node:        node,
		Fulcrum:     fulcrum,
		Comparison: ComparisonStatus{
			HeightDiff:      heightDiff,
			TimeDiffSeconds: timeDiff,
			InSync:          inSync,
		},
	}

	c.JSON(http.StatusOK, response)
}

// handleBlockCount returns current block count
func (s *Server) handleBlockCount(c *gin.Context) {
	ctx := c.Request.Context()

	// Try Redis first
	if count, err := s.redis.GetBlockCount(ctx); err == nil && count > 0 {
		c.JSON(http.StatusOK, gin.H{"blockcount": count})
		return
	}

	// Fall back to RPC
	if s.rpc != nil {
		count, err := s.rpc.GetBlockCount(ctx)
		if err == nil {
			// Cache it
			s.redis.SetBlockCount(ctx, count)
			c.JSON(http.StatusOK, gin.H{"blockcount": count})
			return
		}
	}

	c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Unable to get block count"})
}

// handleLatestBlocks returns recent blocks
func (s *Server) handleLatestBlocks(c *gin.Context) {
	ctx := c.Request.Context()

	const blockLimit = 15

	// Try Redis first
	blocks, err := s.redis.GetBlocks(ctx, int64(blockLimit))
	if err != nil {
		log.Printf("[API] Redis GetBlocks error: %v", err)
	}

	if len(blocks) >= blockLimit {
		c.JSON(http.StatusOK, blocks)
		return
	}

	// Fall back to RPC
	log.Printf("[API] Falling back to RPC for blocks")
	if s.rpc != nil {
		rpcBlocks, err := s.fetchLatestBlocks(ctx, blockLimit)
		if err == nil && len(rpcBlocks) > 0 {
			if err := s.redis.SetBlocks(ctx, rpcBlocks); err != nil {
				log.Printf("[API] Failed to cache blocks in Redis: %v", err)
			}
			c.JSON(http.StatusOK, rpcBlocks)
			return
		}

		if err != nil {
			log.Printf("[API] RPC fetch error: %v", err)
		}
	}

	if len(blocks) > 0 {
		c.JSON(http.StatusOK, blocks)
		return
	}

	c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Unable to get blocks"})
}

func (s *Server) fetchLatestBlocks(ctx context.Context, limit int) ([]*types.Block, error) {
	if s.rpc == nil {
		return nil, fmt.Errorf("rpc unavailable")
	}

	count, err := s.rpc.GetBlockCount(ctx)
	if err != nil {
		return nil, err
	}

	blocks := make([]*types.Block, 0, limit)
	seen := make(map[string]struct{})

	for i := int64(0); int(i) < limit && count-i >= 0; i++ {
		hash, err := s.rpc.GetBlockHash(ctx, count-i)
		if err != nil {
			continue
		}

		if _, exists := seen[hash]; exists {
			continue
		}
		seen[hash] = struct{}{}

		blockData, err := s.rpc.GetBlock(ctx, hash, 1)
		if err != nil {
			continue
		}

		block := &types.Block{
			Hash:   hash,
			Height: count - i,
		}
		if t, ok := blockData["time"].(float64); ok {
			block.Time = int64(t)
		}
		if size, ok := blockData["size"].(float64); ok {
			block.Size = int(size)
		}
		if nTx, ok := blockData["nTx"].(float64); ok {
			block.TxCount = int(nTx)
		}

		if txs, ok := blockData["tx"].([]interface{}); ok && len(txs) > 0 {
			if txid, ok := txs[0].(string); ok {
				txData, err := s.rpc.GetRawTransaction(ctx, txid, true)
				if err == nil && txData != nil {
					block.Miner = s.extractMinerFromTx(txData)
				}
			}
		}

		blocks = append(blocks, block)
	}

	return blocks, nil
}

// handleBlock returns block details
func (s *Server) handleBlock(c *gin.Context) {
	ctx := c.Request.Context()
	hash := c.Param("hash")

	if hash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Block hash required"})
		return
	}

	if s.rpc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "RPC unavailable"})
		return
	}

	blockData, err := s.rpc.GetBlock(ctx, hash, 2)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Block not found"})
		return
	}

	c.JSON(http.StatusOK, blockData)
}

// handleBlockHash returns block hash for height
func (s *Server) handleBlockHash(c *gin.Context) {
	ctx := c.Request.Context()
	heightStr := c.Param("height")

	height, err := strconv.ParseInt(heightStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid height"})
		return
	}

	if s.rpc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "RPC unavailable"})
		return
	}

	hash, err := s.rpc.GetBlockHash(ctx, height)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Block not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"hash": hash})
}

// handleRecentTransactions returns recent transactions
func (s *Server) handleRecentTransactions(c *gin.Context) {
	ctx := c.Request.Context()

	// Get from Redis (includes mempool + recent confirmed)
	txs, err := s.redis.GetTransactions(ctx, 20)
	if err == nil && len(txs) > 0 {
		c.JSON(http.StatusOK, gin.H{
			"items":     txs,
			"updatedAt": time.Now().Unix(),
		})
		return
	}

	// Fallback: get mempool
	if s.rpc != nil {
		mempool, err := s.rpc.GetRawMempool(ctx, true)
		if err == nil {
			now := time.Now().Unix()
			txs = make([]*types.Transaction, 0)
			for txid, info := range mempool {
				if infoMap, ok := info.(map[string]interface{}); ok {
					tx := &types.Transaction{
						Txid:   txid,
						Status: "mempool",
						Time:   now, // Default to now
					}
					// Override with RPC time if available
					if txTime, ok := infoMap["time"].(float64); ok && txTime > 0 {
						tx.Time = int64(txTime)
					}
					if size, ok := infoMap["size"].(float64); ok {
						tx.Size = int(size)
					}
					txs = append(txs, tx)
					if len(txs) >= 20 {
						break
					}
				}
			}
			c.JSON(http.StatusOK, gin.H{
				"items":     txs,
				"updatedAt": time.Now().Unix(),
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"items":     []*types.Transaction{},
		"updatedAt": time.Now().Unix(),
	})
}

// handleTransaction returns transaction details
func (s *Server) handleTransaction(c *gin.Context) {
	ctx := c.Request.Context()
	txid := c.Param("txid")

	if txid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Transaction ID required"})
		return
	}

	// Try Redis cache first
	if tx, err := s.redis.GetFullTransaction(ctx, txid); err == nil && tx != nil {
		// Enrich inputs with address data from previous outputs
		s.enrichInputsWithAddressesTx(ctx, tx)
		// Enrich with BCMR
		s.enrichWithBCMR(ctx, tx)
		c.JSON(http.StatusOK, tx)
		return
	}

	if s.rpc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "RPC unavailable"})
		return
	}

	txData, err := s.rpc.GetRawTransaction(ctx, txid, true)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
		return
	}

	// Enrich inputs with address data from previous outputs
	s.enrichInputsWithAddresses(ctx, txData)

	c.JSON(http.StatusOK, txData)
}

// enrichInputsWithAddresses fetches previous transaction outputs and adds address info to inputs
func (s *Server) enrichInputsWithAddresses(ctx context.Context, txData map[string]interface{}) {
	vin, ok := txData["vin"].([]interface{})
	if !ok {
		return
	}

	for i, v := range vin {
		input, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		prevTxid, _ := input["txid"].(string)
		prevVout, _ := input["vout"].(float64)

		if prevTxid == "" {
			continue
		}

		prevTx, err := s.rpc.GetRawTransaction(ctx, prevTxid, true)
		if err != nil {
			continue
		}

		vouts, ok := prevTx["vout"].([]interface{})
		if !ok || int(prevVout) >= len(vouts) {
			continue
		}

		output, ok := vouts[int(prevVout)].(map[string]interface{})
		if !ok {
			continue
		}

		scriptPubKey, ok := output["scriptPubKey"].(map[string]interface{})
		if !ok {
			continue
		}

		input["scriptPubKey"] = scriptPubKey
		vin[i] = input
	}
}

// enrichInputsWithAddressesTx is the typed version for *types.Transaction
func (s *Server) enrichInputsWithAddressesTx(ctx context.Context, tx *types.Transaction) {
	if len(tx.Inputs) == 0 {
		return
	}

	for i := range tx.Inputs {
		input := &tx.Inputs[i]
		if input.Txid == "" {
			continue
		}

		prevTx, err := s.rpc.GetRawTransaction(ctx, input.Txid, true)
		if err != nil {
			continue
		}

		vouts, ok := prevTx["vout"].([]interface{})
		if !ok || input.Vout >= len(vouts) {
			continue
		}

		output, ok := vouts[input.Vout].(map[string]interface{})
		if !ok {
			continue
		}

		scriptPubKey, ok := output["scriptPubKey"].(map[string]interface{})
		if !ok {
			continue
		}

		input.ScriptPubKey = scriptPubKey
	}
}

// handleTxOut returns output spent status
func (s *Server) handleTxOut(c *gin.Context) {
	ctx := c.Request.Context()
	txid := c.Param("txid")
	voutStr := c.Param("vout")

	vout, err := strconv.Atoi(voutStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid vout"})
		return
	}

	if s.rpc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "RPC unavailable"})
		return
	}

	// Get transaction to check if output is spent
	txData, err := s.rpc.GetRawTransaction(ctx, txid, true)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
		return
	}

	// Check vout exists
	vouts, ok := txData["vout"].([]interface{})
	if !ok || vout >= len(vouts) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Output not found"})
		return
	}

	output := vouts[vout].(map[string]interface{})
	spent := false
	spender := ""

	// To determine if spent, we'd need to query all inputs
	// This is expensive, so we'll return unconfirmed
	status := "unspent"
	if spent {
		status = "spent"
	}
	c.JSON(http.StatusOK, gin.H{
		"txid":         txid,
		"vout":         vout,
		"status":       status,
		"spent":        spent,
		"spender":      spender,
		"scriptPubKey": output["scriptPubKey"],
		"value":        output["value"],
	})
}

// handleAddressTransactions returns address transactions
func (s *Server) handleAddressTransactions(c *gin.Context) {
	ctx := c.Request.Context()
	address := c.Param("address")

	if address == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Address required"})
		return
	}

	// URL-decode the address (handle URL-encoded colons)
	decodedAddress, err := url.QueryUnescape(address)
	if err == nil && decodedAddress != "" {
		address = decodedAddress
	}

	// Validate address
	valid, addrType := utils.ValidateCashAddress(address)
	if !valid {
		log.Printf("Address validation failed for: %s", address)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid address format"})
		return
	}

	// Convert to scripthash
	scripthash, err := utils.AddressToScripthash(address)
	if err != nil {
		log.Printf("Address to scripthash conversion failed for %s: %v", address, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid address: %v", err)})
		return
	}

	// Get pagination params
	cursor := c.Query("cursor")
	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	// Check cache
	cacheKey := fmt.Sprintf("addr:%s:%s:%d", address, cursor, limit)
	var result map[string]interface{}
	if found, _ := s.redis.CacheGet(ctx, cacheKey, &result); found {
		c.JSON(http.StatusOK, result)
		return
	}

	// Get history from Fulcrum
	history, err := s.fulcrum.GetHistory(ctx, scripthash)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Unable to get transactions"})
		return
	}

	// Apply pagination
	startIdx := 0
	if cursor != "" {
		// Parse cursor as index
		if idx, err := strconv.Atoi(cursor); err == nil {
			startIdx = idx
		}
	}

	endIdx := startIdx + limit
	if endIdx > len(history) {
		endIdx = len(history)
	}

	hasMore := endIdx < len(history)
	nextCursor := ""
	if hasMore {
		nextCursor = strconv.Itoa(endIdx)
	}

	// Get balance
	balance, _ := s.fulcrum.GetBalance(ctx, scripthash)

	// Build response
	result = map[string]interface{}{
		"address":      address,
		"type":         addrType,
		"balance":      balance,
		"transactions": history[startIdx:endIdx],
		"pagination": map[string]interface{}{
			"cursor":     cursor,
			"nextCursor": nextCursor,
			"limit":      limit,
			"hasMore":    hasMore,
		},
	}

	// Cache for 60 seconds
	s.redis.CacheSet(ctx, cacheKey, result, 60*time.Second)

	c.JSON(http.StatusOK, result)
}

// handleBroadcast broadcasts a raw transaction
func (s *Server) handleBroadcast(c *gin.Context) {
	ctx := c.Request.Context()

	var req struct {
		Hex string `json:"hex"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Validate hex
	if len(req.Hex) < 20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid transaction hex"})
		return
	}

	if s.rpc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "RPC unavailable"})
		return
	}

	txid, err := s.rpc.SendRawTransaction(ctx, req.Hex)
	if err != nil {
		// Map common errors
		errStr := err.Error()
		switch {
		case contains(errStr, "txn-already-known"):
			c.JSON(http.StatusConflict, types.BroadcastResult{Success: false, Error: "Transaction already exists"})
		case contains(errStr, "bad-txns"):
			c.JSON(http.StatusBadRequest, types.BroadcastResult{Success: false, Error: "Invalid transaction"})
		case contains(errStr, "insufficient fee"):
			c.JSON(http.StatusBadRequest, types.BroadcastResult{Success: false, Error: "Insufficient fee"})
		case contains(errStr, "missing-inputs"):
			c.JSON(http.StatusBadRequest, types.BroadcastResult{Success: false, Error: "Missing inputs"})
		default:
			c.JSON(http.StatusInternalServerError, types.BroadcastResult{Success: false, Error: errStr})
		}
		return
	}

	c.JSON(http.StatusOK, types.BroadcastResult{Success: true, Txid: txid})
}

// handleTokenMetadata returns BCMR token metadata
func (s *Server) handleTokenMetadata(c *gin.Context) {
	ctx := c.Request.Context()
	category := c.Param("category")

	if category == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Category required"})
		return
	}

	// Check cache first
	if metadata, err := s.redis.GetTokenMetadata(ctx, category); err == nil && metadata != nil {
		c.JSON(http.StatusOK, metadata)
		return
	}

	// Fetch from BCMR
	metadata, err := s.bcmr.GetTokenMetadata(ctx, category)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Token not found"})
		return
	}

	// Cache it
	s.redis.SetTokenMetadata(ctx, category, metadata)

	c.JSON(http.StatusOK, metadata)
}

// handleSearch redirects search queries
func (s *Server) handleSearch(c *gin.Context) {
	query := c.Query("q")

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query required"})
		return
	}

	// Check if it's a transaction ID (64 hex chars)
	if len(query) == 64 {
		c.Redirect(http.StatusFound, fmt.Sprintf("/tx/%s", query))
		return
	}

	// Check if it's an address
	if valid, _ := utils.ValidateCashAddress(query); valid {
		c.Redirect(http.StatusFound, fmt.Sprintf("/address/%s", query))
		return
	}

	// Check if it's a block hash (64 hex chars starting with 0)
	if len(query) == 64 && query[0] == '0' {
		c.Redirect(http.StatusFound, fmt.Sprintf("/block/%s", query))
		return
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "Unknown search query"})
}

// enrichWithBCMR enriches transaction with BCMR metadata
func (s *Server) enrichWithBCMR(ctx context.Context, tx *types.Transaction) {
	if !tx.HasTokens {
		return
	}

	// Collect token categories from outputs
	categories := make(map[string]bool)
	for _, output := range tx.Outputs {
		if output.TokenData != nil {
			if category, ok := output.TokenData["category"].(string); ok {
				categories[category] = true
			}
		}
	}

	// Fetch metadata for each category
	if len(categories) > 0 {
		tx.TokenData = make(map[string]interface{})
		for category := range categories {
			if metadata, err := s.bcmr.GetTokenMetadata(ctx, category); err == nil {
				tx.TokenData[category] = metadata
			}
		}
	}
}

// extractMinerFromTx extracts pool name from coinbase transaction
func (s *Server) extractMinerFromTx(txData map[string]interface{}) string {
	// First: Extract from coinbase and match against known pool tags
	vinRaw, ok := txData["vin"]
	if ok {
		if vin, ok := vinRaw.([]interface{}); ok && len(vin) > 0 {
			if firstIn, ok := vin[0].(map[string]interface{}); ok {
				if coinbase, ok := firstIn["coinbase"].(string); ok && len(coinbase) > 0 {
					// Decode coinbase hex to text
					decoded, err := hex.DecodeString(coinbase)
					if err == nil {
						// Convert to printable ASCII
						var result []byte
						for _, b := range decoded {
							if b >= 0x20 && b <= 0x7E {
								result = append(result, b)
							} else {
								result = append(result, ' ')
							}
						}
						coinbaseText := string(result)

						log.Printf("[MINER DEBUG] coinbase text: %.100s", coinbaseText)

						// Match against known pool tags (case-insensitive)
						coinbaseLower := strings.ToLower(coinbaseText)
						for _, pool := range s.pools.Pools {
							for _, tag := range pool.Tags {
								if strings.Contains(coinbaseLower, strings.ToLower(tag)) {
									log.Printf("[MINER DEBUG] Matched pool: %s via tag: %s", pool.Name, tag)
									return pool.Name
								}
							}
						}

						// Fallback: Extract text between slashes (like /K1Pool.com/ → K1Pool.com)
						// This is what bch-rpc-explorer does
						if idx := strings.Index(coinbaseText, "/"); idx != -1 {
							endIdx := strings.Index(coinbaseText[idx+1:], "/")
							if endIdx != -1 {
								possibleSignal := strings.TrimSpace(coinbaseText[idx+1 : idx+1+endIdx])
								if len(possibleSignal) >= 3 && len(possibleSignal) <= 50 {
									log.Printf("[MINER DEBUG] Using possibleSignal: %s", possibleSignal)
									return possibleSignal
								}
							}
						}
					}
				}
			}
		}
	}

	return "Unknown"
}

// Common mining pool identifiers
var commonPools = []string{
	"ViaBTC", "AntPool", "BTC.com", "Poolin", "F2Pool", "Binance", "SlushPool",
	"EMCD", "Foundry", "Luxor", "SBI", "MARA", "HUT", "Catpool", "Rawpool",
}

// extractMinerFromCoinbaseHex extracts pool name from coinbase hex
func extractMinerFromCoinbaseHex(coinbaseHex string) string {
	if len(coinbaseHex) == 0 || len(coinbaseHex)%2 != 0 {
		return ""
	}

	decoded, err := hex.DecodeString(coinbaseHex)
	if err != nil {
		return ""
	}

	// Convert to printable ASCII
	var result []byte
	for _, b := range decoded {
		if b >= 0x20 && b <= 0x7E {
			result = append(result, b)
		} else {
			result = append(result, ' ')
		}
	}

	cleaned := normalizeWhitespace(string(result))
	if cleaned == "" {
		return ""
	}

	// Strategy 1: Look for /poolname/ pattern
	if pool := extractSlashDelimitedTag(cleaned); pool != "" {
		return pool
	}

	// Strategy 2: Look for "mined by" or "pool" keywords
	if pool := extractFromKeywords(cleaned); pool != "" {
		return pool
	}

	// Strategy 3: Look for known pool names
	if pool := extractKnownPool(cleaned); pool != "" {
		return pool
	}

	// Strategy 4: Extract longest reasonable alphanumeric sequence
	if pool := extractLongestValidSequence(cleaned); pool != "" {
		return pool
	}

	return ""
}

// normalizeWhitespace replaces multiple spaces with single space and trims
func normalizeWhitespace(s string) string {
	var result []rune
	inSpace := false
	for _, r := range s {
		if r == ' ' {
			if !inSpace {
				result = append(result, r)
				inSpace = true
			}
		} else {
			result = append(result, r)
			inSpace = false
		}
	}
	// Trim leading/trailing spaces
	start := 0
	end := len(result)
	for start < end && result[start] == ' ' {
		start++
	}
	for end > start && result[end-1] == ' ' {
		end--
	}
	return string(result[start:end])
}

// extractSlashDelimitedTag extracts pool name from slash-delimited pattern
func extractSlashDelimitedTag(s string) string {
	parts := splitBySlash(s)
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "" {
			tag := strings.TrimRight(parts[i+1], " ")
			if len(tag) > 0 && len(tag) <= 40 && isValidPoolNameStart(tag[0]) {
				if isValidPoolName(tag) {
					return strings.TrimSpace(tag)
				}
			}
		}
	}
	return ""
}

// splitBySlash splits string by "/" preserving empty strings for consecutive slashes
func splitBySlash(s string) []string {
	var parts []string
	current := ""
	for _, r := range s {
		if r == '/' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(r)
		}
	}
	parts = append(parts, current)
	return parts
}

// isValidPoolNameStart checks if character is valid start of pool name
func isValidPoolNameStart(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}

// isValidPoolName checks if string is valid pool name
func isValidPoolName(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == ' ' || c == '.' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

// extractFromKeywords looks for pool names after common keywords
func extractFromKeywords(s string) string {
	lower := strings.ToLower(s)
	keywords := []string{"mined by", "pool", "mining"}

	for _, kw := range keywords {
		idx := strings.Index(lower, kw)
		if idx != -1 {
			start := idx + len(kw)
			if start < len(s) {
				for start < len(s) && s[start] == ' ' {
					start++
				}
				end := start
				for end < len(s) && end < start+30 {
					c := s[end]
					if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
						c == ' ' || c == '.' || c == '_' || c == '-' {
						end++
					} else {
						break
					}
				}
				if end > start {
					return strings.TrimSpace(s[start:end])
				}
			}
		}
	}
	return ""
}

// extractKnownPool looks for known pool names
func extractKnownPool(s string) string {
	for _, pool := range commonPools {
		if strings.Contains(s, pool) {
			return pool
		}
		if strings.Contains(strings.ToLower(s), strings.ToLower(pool)) {
			return pool
		}
	}
	return ""
}

// extractLongestValidSequence extracts longest alphanumeric sequence
func extractLongestValidSequence(s string) string {
	var longest string
	var current string

	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			current += string(c)
		} else if c == ' ' {
			if len(current) > len(longest) && len(current) >= 3 {
				longest = current
			}
			current = ""
		} else {
			current = ""
		}
	}
	if len(current) > len(longest) && len(current) >= 3 {
		longest = current
	}

	if len(longest) >= 3 && len(longest) <= 20 {
		first := longest[0]
		if (first >= 'A' && first <= 'Z') || (first >= 'a' && first <= 'z') {
			return longest
		}
	}
	return ""
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) && containsSubstr(s, substr)))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	log.Printf("Starting HTTP server on %s", addr)

	server := &http.Server{
		Addr:    addr,
		Handler: s.router,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		log.Println("Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	return server.ListenAndServe()
}

// Close closes all connections
func (s *Server) Close() {
	if s.redis != nil {
		s.redis.Close()
	}
	if s.rpc != nil {
		s.rpc.Close()
	}
	if s.fulcrum != nil {
		s.fulcrum.Close()
	}
}

func main() {
	cfg := loadConfig()

	server, err := NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	defer server.Close()

	if err := server.Start(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
