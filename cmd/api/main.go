package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
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

// handleStatus returns node status
func (s *Server) handleStatus(c *gin.Context) {
	ctx := c.Request.Context()
	status := types.NodeStatus{}

	// Check Redis
	if err := s.redis.Ping(ctx); err == nil {
		status.Redis.Connected = true
		if latest, err := s.redis.GetLatestBlock(ctx); err == nil && latest != nil {
			status.Redis.LatestBlock = latest.Height
		}
	}

	// Check BCH node
	if s.rpc != nil {
		info, err := s.rpc.GetBlockchainInfo(ctx)
		if err == nil && info != nil {
			status.Node.Connected = true
			if h, ok := info["blocks"].(float64); ok {
				status.Node.BlockHeight = int64(h)
			}
			if h, ok := info["headers"].(float64); ok {
				status.Node.Headers = int64(h)
			}
			if d, ok := info["difficulty"].(float64); ok {
				status.Node.Difficulty = d
			}
			if v, ok := info["version"].(float64); ok {
				status.Node.Version = int(v)
			}
		}

		network, _ := s.rpc.GetNetworkInfo(ctx)
		if network != nil {
			if p, ok := network["protocolversion"].(float64); ok {
				status.Node.ProtocolVersion = int(p)
			}
		}
	}

	// Check Fulcrum
	if s.fulcrum != nil {
		// Try to get server version to check connection
		_, err := s.fulcrum.ServerVersion(ctx)
		if err == nil {
			status.Fulcrum.Connected = true
			status.Fulcrum.BlockHeight = status.Node.BlockHeight
		}
	}

	// Check sync status
	status.InSync = status.Node.Connected && status.Node.BlockHeight >= status.Node.Headers-2

	c.JSON(http.StatusOK, status)
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

	// Try Redis first
	blocks, err := s.redis.GetBlocks(ctx, 15)
	if err == nil && len(blocks) > 0 {
		c.JSON(http.StatusOK, blocks)
		return
	}

	// Fall back to RPC
	if s.rpc != nil {
		count, err := s.rpc.GetBlockCount(ctx)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Unable to get blocks"})
			return
		}

		blocks = make([]*types.Block, 0, 15)
		for i := int64(0); i < 15 && count-i >= 0; i++ {
			hash, err := s.rpc.GetBlockHash(ctx, count-i)
			if err != nil {
				continue
			}

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

			blocks = append(blocks, block)
		}

		c.JSON(http.StatusOK, blocks)
		return
	}

	c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Unable to get blocks"})
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

	c.JSON(http.StatusOK, txData)
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
	c.JSON(http.StatusOK, gin.H{
		"txid":         txid,
		"vout":         vout,
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
