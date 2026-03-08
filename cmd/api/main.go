package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
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
	cursorStr := c.Query("cursor")
	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	windowStr := c.DefaultQuery("window", "1000")
	window, err := strconv.Atoi(windowStr)
	if err != nil || window < 100 || window > 50000 {
		window = 1000
	}

	cursor := 0
	if cursorStr != "" {
		if parsed, err := strconv.Atoi(cursorStr); err == nil && parsed >= 0 {
			cursor = parsed
		}
	}

	cacheKey := fmt.Sprintf("addr:%s:%d:%d:%d", address, cursor, limit, window)
	var result map[string]interface{}
	if found, _ := s.redis.CacheGet(ctx, cacheKey, &result); found {
		c.JSON(http.StatusOK, result)
		return
	}

	// Get current best height for confirmation counts
	var tipHeight int64
	var tip map[string]interface{}
	if err := s.fulcrum.Request(ctx, "blockchain.headers.subscribe", []interface{}{}, &tip); err == nil {
		tipHeight = toInt64(tip["height"])
	}

	// Get confirmed history from Fulcrum (scripthash-first, address fallback)
	history, err := fetchAddressHistory(ctx, s.fulcrum, scripthash, address)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Unable to get transactions"})
		return
	}

	// Convert/normalize history entries and sort newest first
	type historyItem struct {
		txid   string
		height int64
	}

	confirmedHistory := make([]historyItem, 0, len(history))
	for _, row := range history {
		txid := strings.ToLower(toString(row["tx_hash"]))
		if txid == "" {
			txid = strings.ToLower(toString(row["txid"]))
		}
		if txid == "" {
			continue
		}
		confirmedHistory = append(confirmedHistory, historyItem{txid: txid, height: toInt64(row["height"])})
	}

	sort.SliceStable(confirmedHistory, func(i, j int) bool {
		if confirmedHistory[i].height == confirmedHistory[j].height {
			return confirmedHistory[i].txid > confirmedHistory[j].txid
		}
		return confirmedHistory[i].height > confirmedHistory[j].height
	})

	startIdx := cursor
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx > len(confirmedHistory) {
		startIdx = len(confirmedHistory)
	}

	endIdx := startIdx + limit
	if endIdx > len(confirmedHistory) {
		endIdx = len(confirmedHistory)
	}

	hasMore := endIdx < len(confirmedHistory)
	var nextCursor interface{}
	if hasMore {
		nextCursor = endIdx
	}

	candidateAddrs := normalizeAddressCandidates(address)
	seenTx := make(map[string]struct{})
	items := make([]map[string]interface{}, 0, limit)

	// Newest page includes mempool txs for compatibility.
	if cursor == 0 {
		mempool, memErr := fetchAddressMempool(ctx, s.fulcrum, scripthash, address)
		if memErr == nil {
			for _, row := range mempool {
				txid := strings.ToLower(toString(row["tx_hash"]))
				if txid == "" {
					txid = strings.ToLower(toString(row["txid"]))
				}
				if txid == "" || len(items) >= limit {
					continue
				}
				if _, exists := seenTx[txid]; exists {
					continue
				}
				items = append(items, fetchAddressTxItem(ctx, s, txid, "mempool", 0, tipHeight, candidateAddrs))
				seenTx[txid] = struct{}{}
			}
		}
	}

	for i := startIdx; i < endIdx && len(items) < limit; i++ {
		e := confirmedHistory[i]
		if _, exists := seenTx[e.txid]; exists {
			continue
		}

		item := fetchAddressTxItem(ctx, s, e.txid, "confirmed", e.height, tipHeight, candidateAddrs)
		if item["blockHeight"] == nil {
			item["blockHeight"] = e.height
		}
		if toString(item["txid"]) == "" {
			item["txid"] = e.txid
		}
		items = append(items, item)
		seenTx[e.txid] = struct{}{}
	}

	// Compatibility payload with older shape
	transactions := make([]gin.H, 0, len(confirmedHistory))
	for _, e := range confirmedHistory {
		transactions = append(transactions, gin.H{"tx_hash": e.txid, "height": e.height})
	}

	// Get balances
	balance, err := fetchAddressBalance(ctx, s.fulcrum, scripthash, address)
	if err != nil {
		balance = map[string]interface{}{"confirmed": int64(0), "unconfirmed": int64(0)}
	}

	// Optional token balances from tokenized UTXOs
	type tokenAgg struct {
		category   string
		fungible   *big.Int
		nftNone    int
		nftMutable int
		nftMinting int
		utxoCount  int
	}

	tokenMap := make(map[string]*tokenAgg)
	var tokenUTXOs []map[string]interface{}
	if err := fetchAddressTokenUtxos(ctx, s.fulcrum, scripthash, address, &tokenUTXOs); err == nil {
		for _, u := range tokenUTXOs {
			td, ok := u["token_data"].(map[string]interface{})
			if !ok {
				td, ok = u["token"].(map[string]interface{})
				if !ok {
					continue
				}
			}

			cat, ok := td["category"].(string)
			if !ok || cat == "" {
				continue
			}

			agg, ok := tokenMap[cat]
			if !ok {
				agg = &tokenAgg{category: cat, fungible: big.NewInt(0)}
				tokenMap[cat] = agg
			}

			agg.utxoCount++

			if amtRaw, ok := td["amount"].(string); ok {
				if amt, parsed := new(big.Int).SetString(amtRaw, 10); parsed {
					agg.fungible.Add(agg.fungible, amt)
				}
			} else if amtRawNum, ok := td["amount"].(float64); ok {
				agg.fungible.Add(agg.fungible, big.NewInt(int64(amtRawNum)))
			} else if amtRawInt, ok := td["amount"].(int64); ok {
				agg.fungible.Add(agg.fungible, big.NewInt(amtRawInt))
			} else if amtRawInt, ok := td["amount"].(int); ok {
				agg.fungible.Add(agg.fungible, big.NewInt(int64(amtRawInt)))
			}

			if nft, ok := td["nft"].(map[string]interface{}); ok {
				switch nft["capability"] {
				case "none":
					agg.nftNone++
				case "mutable":
					agg.nftMutable++
				case "minting":
					agg.nftMinting++
				}
			}
		}
	}

	tokenBuckets := make([]*tokenAgg, 0, len(tokenMap))
	for _, b := range tokenMap {
		tokenBuckets = append(tokenBuckets, b)
	}

	sort.SliceStable(tokenBuckets, func(i, j int) bool {
		if cmp := tokenBuckets[i].fungible.Cmp(tokenBuckets[j].fungible); cmp != 0 {
			return cmp > 0
		}
		left := tokenBuckets[i].nftNone + tokenBuckets[i].nftMutable + tokenBuckets[i].nftMinting
		right := tokenBuckets[j].nftNone + tokenBuckets[j].nftMutable + tokenBuckets[j].nftMinting
		return left > right
	})

	tokenBalances := make([]map[string]interface{}, 0, len(tokenBuckets))
	for _, b := range tokenBuckets {
		nftCount := b.nftNone + b.nftMutable + b.nftMinting
		tokenBalances = append(tokenBalances, map[string]interface{}{
			"category":       b.category,
			"fungibleAmount": b.fungible.String(),
			"nftCount":       nftCount,
			"nft": map[string]interface{}{
				"none":    b.nftNone,
				"mutable": b.nftMutable,
				"minting": b.nftMinting,
			},
			"utxoCount": b.utxoCount,
		})
	}

	tokenMeta := make(map[string]interface{})
	for _, b := range tokenBuckets {
		if meta, err := s.bcmr.GetTokenMetadata(ctx, b.category); err == nil {
			tokenMeta[b.category] = meta
		}
	}

	result = map[string]interface{}{
		"address": address,
		"type":    addrType,
		"balance": balance,
		"scanned": map[string]interface{}{
			"source":     "fulcrum",
			"scripthash": scripthash,
			"tipHeight":  tipHeight,
			"cursor":     cursor,
			"window":     window,
		},
		"nextCursor":    nextCursor,
		"tokenBalances": tokenBalances,
		"tokenMeta":     tokenMeta,
		"items":         items,
		"transactions":  transactions,
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

func fetchAddressHistory(ctx context.Context, client *fulcrum.Client, scripthash, address string) ([]map[string]interface{}, error) {
	history, err := client.GetHistory(ctx, scripthash)
	if err == nil && len(history) > 0 {
		return history, nil
	}

	var addressHistory []map[string]interface{}
	fallbackErr := client.Request(ctx, "blockchain.address.get_history", []interface{}{address}, &addressHistory)
	if fallbackErr == nil && len(addressHistory) > 0 {
		return addressHistory, nil
	}

	if err != nil {
		if fallbackErr == nil {
			return addressHistory, nil
		}
		return nil, err
	}

	if fallbackErr != nil {
		return history, nil
	}

	return []map[string]interface{}{}, nil
}

func fetchAddressBalance(ctx context.Context, client *fulcrum.Client, scripthash, address string) (map[string]interface{}, error) {
	type balanceAttempt struct {
		label  string
		method string
		args   []interface{}
	}

	attempts := []balanceAttempt{
		{label: "scripthash include_tokens", method: "blockchain.scripthash.get_balance", args: []interface{}{scripthash, "include_tokens"}},
		{label: "address include_tokens", method: "blockchain.address.get_balance", args: []interface{}{address, "include_tokens"}},
		{label: "scripthash tokens_only", method: "blockchain.scripthash.get_balance", args: []interface{}{scripthash, "tokens_only"}},
		{label: "address tokens_only", method: "blockchain.address.get_balance", args: []interface{}{address, "tokens_only"}},
		{label: "scripthash default", method: "blockchain.scripthash.get_balance", args: []interface{}{scripthash}},
		{label: "address default", method: "blockchain.address.get_balance", args: []interface{}{address}},
	}

	normalize := func(raw map[string]interface{}, label string) map[string]interface{} {
		if len(raw) == 0 {
			return nil
		}

		confirmed := toInt64(raw["confirmed"])
		unconfirmed := toInt64(raw["unconfirmed"])
		if confirmed == 0 && unconfirmed == 0 {
			return map[string]interface{}{"confirmed": int64(0), "unconfirmed": int64(0)}
		}

		log.Printf("[ADDRBAL] %s for %s returned non-zero BCH balance: %d confirmed, %d unconfirmed", label, address, confirmed, unconfirmed)
		return map[string]interface{}{"confirmed": confirmed, "unconfirmed": unconfirmed}
	}

	var fallback map[string]interface{}
	var lastErr error

	for _, attempt := range attempts {
		var raw map[string]interface{}
		if err := client.Request(ctx, attempt.method, attempt.args, &raw); err != nil {
			if lastErr == nil {
				lastErr = err
			}
			continue
		}

		parsed := normalize(raw, attempt.label)
		if parsed == nil {
			continue
		}

		if fallback == nil {
			fallback = parsed
		}

		if parsed["confirmed"].(int64)+parsed["unconfirmed"].(int64) > 0 {
			log.Printf("[ADDRBAL] selected non-zero %s result for %s", attempt.label, address)
			return parsed, nil
		}
	}

	if fallback != nil {
		if toInt64(fallback["confirmed"]) == 0 && toInt64(fallback["unconfirmed"]) == 0 {
			log.Printf("[ADDRBAL] all balance attempts returned zero for %s", address)
		}
		return fallback, nil
	}

	if lastErr == nil {
		return map[string]interface{}{"confirmed": int64(0), "unconfirmed": int64(0)}, nil
	}

	return nil, lastErr
}

func fetchAddressMempool(ctx context.Context, client *fulcrum.Client, scripthash, address string) ([]map[string]interface{}, error) {
	mempool, err := client.GetMempool(ctx, scripthash)
	if err == nil && len(mempool) > 0 {
		return mempool, nil
	}

	var addressMempool []map[string]interface{}
	fallbackErr := client.Request(ctx, "blockchain.address.get_mempool", []interface{}{address}, &addressMempool)
	if fallbackErr == nil {
		return addressMempool, nil
	}

	if err != nil {
		return nil, err
	}

	if fallbackErr != nil {
		return nil, fallbackErr
	}

	return []map[string]interface{}{}, nil
}

func fetchAddressTokenUtxos(ctx context.Context, client *fulcrum.Client, scripthash, address string, out *[]map[string]interface{}) error {
	attempts := []struct {
		method string
		args   []interface{}
	}{
		{method: "blockchain.scripthash.listunspent", args: []interface{}{scripthash, "include_tokens"}},
		{method: "blockchain.address.listunspent", args: []interface{}{address, "include_tokens"}},
		{method: "blockchain.scripthash.listunspent", args: []interface{}{scripthash, "tokens_only"}},
		{method: "blockchain.scripthash.listunspent", args: []interface{}{scripthash}},
		{method: "blockchain.address.listunspent", args: []interface{}{address, "tokens_only"}},
		{method: "blockchain.address.listunspent", args: []interface{}{address}},
	}

	for _, attempt := range attempts {
		candidate := make([]map[string]interface{}, 0)
		if err := client.Request(ctx, attempt.method, attempt.args, &candidate); err != nil {
			continue
		}

		if len(candidate) == 0 {
			continue
		}

		*out = append(*out, candidate...)
		return nil
	}

	return nil
}

func toInt64(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	case float32:
		return int64(n)
	case string:
		if parsed, err := strconv.ParseInt(n, 10, 64); err == nil {
			return parsed
		}
		if f, err := strconv.ParseFloat(n, 64); err == nil {
			return int64(f)
		}
	}
	return 0
}

func toString(v interface{}) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(s))
}

func satsFromBch(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(math.Round(n * 1e8))
	case float32:
		return int64(math.Round(float64(n) * 1e8))
	case string:
		if f, err := strconv.ParseFloat(n, 64); err == nil {
			return int64(math.Round(f * 1e8))
		}
	}
	return 0
}

func normalizeAddressCandidates(address string) map[string]struct{} {
	result := make(map[string]struct{})
	raw := strings.ToLower(strings.TrimSpace(address))
	if raw == "" {
		return result
	}

	result[raw] = struct{}{}
	if strings.Contains(raw, ":") {
		parts := strings.SplitN(raw, ":", 2)
		if len(parts) == 2 && parts[1] != "" {
			result[parts[1]] = struct{}{}
		}
		return result
	}

	result["bitcoincash:"+raw] = struct{}{}
	result["bchtest:"+raw] = struct{}{}
	result["bchreg:"+raw] = struct{}{}
	return result
}

func valueMatchesAddress(v interface{}, candidates map[string]struct{}) bool {
	switch val := v.(type) {
	case string:
		_, ok := candidates[strings.ToLower(strings.TrimSpace(val))]
		return ok
	case []interface{}:
		for _, item := range val {
			if valueMatchesAddress(item, candidates) {
				return true
			}
		}
	}
	return false
}

func hasTokenMarker(v map[string]interface{}) bool {
	for key := range v {
		if strings.Contains(key, "token") {
			return true
		}
	}
	return false
}

func addressMatchesScriptPubKey(scriptPubKey interface{}, candidates map[string]struct{}) bool {
	if scriptPubKey == nil {
		return false
	}

	sp, ok := scriptPubKey.(map[string]interface{})
	if !ok {
		return false
	}

	if valueMatchesAddress(sp["address"], candidates) {
		return true
	}
	if valueMatchesAddress(sp["addresses"], candidates) {
		return true
	}

	return false
}

func fetchAddressTxItem(
	ctx context.Context,
	s *Server,
	txid string,
	status string,
	blockHeight int64,
	tipHeight int64,
	targetAddrs map[string]struct{},
) map[string]interface{} {
	txData, err := s.fulcrum.GetTransaction(ctx, txid, true)
	if err != nil {
		item := map[string]interface{}{
			"txid":      txid,
			"status":    status,
			"direction": "received",
			"net":       0,
			"inValue":   0,
			"outValue":  0,
		}
		if status == "confirmed" && blockHeight > 0 {
			item["blockHeight"] = blockHeight
		}
		return item
	}

	outValue := int64(0)
	inValue := int64(0)
	hasTokens := false

	if vouts, ok := txData["vout"].([]interface{}); ok {
		for _, raw := range vouts {
			out, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}

			if hasTokenMarker(out) {
				hasTokens = true
			}

			if addressMatchesScriptPubKey(out["scriptPubKey"], targetAddrs) {
				outValue += satsFromBch(out["value"])
			}
		}
	}

	if vins, ok := txData["vin"].([]interface{}); ok {
		for _, raw := range vins {
			vin, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}

			prevout, ok := vin["prevout"].(map[string]interface{})
			if !ok {
				continue
			}

			if hasTokenMarker(prevout) {
				hasTokens = true
			}

			if addressMatchesScriptPubKey(prevout["scriptPubKey"], targetAddrs) {
				inValue += satsFromBch(prevout["value"])
			}
		}
	}

	net := outValue - inValue
	direction := "received"
	if net < 0 {
		direction = "sent"
	}

	timeValue := toInt64(txData["time"])
	if blockTime := toInt64(txData["blocktime"]); blockTime > 0 {
		timeValue = blockTime
	}

	if status == "confirmed" && blockHeight <= 0 {
		blockHeight = toInt64(txData["height"])
	}

	confirmations := int64(0)
	if blockHeight > 0 && tipHeight > 0 {
		confirmations = tipHeight - blockHeight + 1
		if confirmations < 0 {
			confirmations = 0
		}
	}

	item := map[string]interface{}{
		"txid":      txid,
		"status":    status,
		"time":      timeValue,
		"direction": direction,
		"net":       float64(net) / 1e8,
		"inValue":   float64(inValue) / 1e8,
		"outValue":  float64(outValue) / 1e8,
		"hasTokens": hasTokens,
	}

	if blockHeight > 0 {
		item["blockHeight"] = blockHeight
	}

	if confirmations > 0 {
		item["confirmations"] = confirmations
	}

	return item
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
