package main

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/pebbe/zmq4"
	
	"bchexplorer/internal/bchrpc"
	"bchexplorer/internal/redis"
	"bchexplorer/internal/types"
	"bchexplorer/internal/utils"
)

// Config holds service configuration
type Config struct {
	ZMQHost   string
	ZMQPort   int
	RedisURL  string
	RedisPrefix string
	BCHRPCURL string
	BCHRPCUser string
	BCHRPCPass string
	MaxBlocks int
	MaxTxs    int
}

// Service represents the ZMQ listener service
type Service struct {
	config   Config
	zmqCtx   *zmq4.Context
	zmqSocket *zmq4.Socket
	redis    *redis.Client
	rpc      *bchrpc.Client
	lastBlockHash string
}

// loadConfig loads configuration from environment
func loadConfig() Config {
	cfg := Config{
		ZMQHost:     getEnv("BCH_ZMQ_HOST", "127.0.0.1"),
		ZMQPort:     getEnvInt("BCH_ZMQ_PORT", 28332),
		RedisURL:    getEnv("REDIS_URL", "redis://127.0.0.1:6379/0"),
		RedisPrefix: getEnv("REDIS_PREFIX", "bch"),
		BCHRPCURL:   getEnv("BCH_RPC_URL", ""),
		BCHRPCUser:  getEnv("BCH_RPC_USER", ""),
		BCHRPCPass:  getEnv("BCH_RPC_PASS", ""),
		MaxBlocks:   getEnvInt("MAX_BLOCKS", 15),
		MaxTxs:      getEnvInt("MAX_TRANSACTIONS", 20),
	}
	return cfg
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

// NewService creates a new ZMQ listener service
func NewService(cfg Config) (*Service, error) {
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
			MaxConns: 10,
		})
	}
	
	return &Service{
		config: cfg,
		redis:  redisClient,
		rpc:    rpcClient,
	}, nil
}

// Start starts the ZMQ listener
func (s *Service) Start(ctx context.Context) error {
	// Create ZMQ context
	zctx, err := zmq4.NewContext()
	if err != nil {
		return fmt.Errorf("failed to create ZMQ context: %w", err)
	}
	s.zmqCtx = zctx
	
	// Connect to ZMQ
	if err := s.connectZMQ(); err != nil {
		return err
	}
	
	log.Println("ZMQ listener started")
	
	// Process messages
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		
		// Poll for messages with timeout
		if err := s.processMessage(); err != nil {
			log.Printf("Error processing message: %v", err)
			
			// Try to reconnect
			s.zmqSocket.Close()
			time.Sleep(time.Second)
			if err := s.connectZMQ(); err != nil {
				log.Printf("Failed to reconnect: %v", err)
				time.Sleep(5 * time.Second)
			}
		}
	}
}

// connectZMQ connects to the ZMQ socket
func (s *Service) connectZMQ() error {
	socket, err := s.zmqCtx.NewSocket(zmq4.SUB)
	if err != nil {
		return fmt.Errorf("failed to create ZMQ socket: %w", err)
	}
	
	addr := fmt.Sprintf("tcp://%s:%d", s.config.ZMQHost, s.config.ZMQPort)
	if err := socket.Connect(addr); err != nil {
		socket.Close()
		return fmt.Errorf("failed to connect to ZMQ: %w", err)
	}
	
	// Subscribe to topics
	socket.SetSubscribe("rawblock")
	socket.SetSubscribe("rawtx")
	
	s.zmqSocket = socket
	log.Printf("Connected to ZMQ at %s", addr)
	
	return nil
}

// processMessage processes a single ZMQ message
func (s *Service) processMessage() error {
	// Receive message with timeout
	s.zmqSocket.SetRcvtimeo(1 * time.Second)
	
	frames, err := s.zmqSocket.RecvMessage(0)
	if err != nil {
		if err.Error() == "resource temporarily unavailable" {
			return nil // Timeout, no message
		}
		return err
	}
	
	if len(frames) < 2 {
		return nil
	}
	
	topic := frames[0]
	data := []byte(frames[1])
	
	switch topic {
	case "rawblock":
		return s.handleBlock(data)
	case "rawtx":
		return s.handleTransaction(data)
	}
	
	return nil
}

// handleBlock processes a raw block
func (s *Service) handleBlock(rawBlock []byte) error {
	// Parse block header to get hash
	if len(rawBlock) < 80 {
		return fmt.Errorf("block too short")
	}
	
	// Block hash is double SHA256 of header
	header := rawBlock[:80]
	hash := utils.DoubleSha256(header)
	
	// Reverse for display
	hashStr := ""
	for i := 31; i >= 0; i-- {
		hashStr += fmt.Sprintf("%02x", hash[i])
	}
	
	log.Printf("Received block: %s", hashStr)
	
	// Get previous block hash
	prevHash := ""
	for i := 3; i >= 0; i-- {
		for j := 3; j >= 0; j-- {
			prevHash += fmt.Sprintf("%02x", header[4+i*4+j])
		}
	}
	
	// Detect reorg
	if s.lastBlockHash != "" && prevHash != s.lastBlockHash {
		log.Printf("Reorg detected! Expected %s, got %s", s.lastBlockHash, prevHash)
		// Handle reorg - this would need to walk back the chain
	}
	
	s.lastBlockHash = hashStr
	
	// Fetch full block details from RPC if available
	var block *types.Block
	if s.rpc != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		blockData, err := s.rpc.GetBlock(ctx, hashStr, 2)
		if err == nil && blockData != nil {
			block = s.parseBlock(blockData)
		}
	}
	
	if block == nil {
		// Parse minimal info from raw block
		timestamp := binary.LittleEndian.Uint32(header[68:72])
		nonce := binary.LittleEndian.Uint32(header[76:80])
		
		// Get transaction count from rest of block
		offset := 80
		txCount, bytesRead, _ := utils.ParseVarInt(rawBlock, offset)
		offset += bytesRead
		
		block = &types.Block{
			Hash:    hashStr,
			Time:    int64(timestamp),
			Size:    len(rawBlock),
			TxCount: int(txCount),
			Nonce:   int64(nonce),
		}
	}
	
	// Store in Redis
	ctx := context.Background()
	if err := s.redis.PushBlock(ctx, block); err != nil {
		log.Printf("Failed to store block: %v", err)
	}
	
	// Process transactions in block
	if s.rpc != nil {
		s.processBlockTransactions(ctx, block)
	}
	
	return nil
}

// parseBlock parses block data from RPC
func (s *Service) parseBlock(data map[string]interface{}) *types.Block {
	block := &types.Block{}
	
	if hash, ok := data["hash"].(string); ok {
		block.Hash = hash
	}
	if height, ok := data["height"].(float64); ok {
		block.Height = int64(height)
	}
	if time, ok := data["time"].(float64); ok {
		block.Time = int64(time)
	}
	if size, ok := data["size"].(float64); ok {
		block.Size = int(size)
	}
	if nTx, ok := data["nTx"].(float64); ok {
		block.TxCount = int(nTx)
	}
	if diff, ok := data["difficulty"].(float64); ok {
		block.Difficulty = diff
	}
	if bits, ok := data["bits"].(string); ok {
		block.Bits = bits
	}
	if nonce, ok := data["nonce"].(float64); ok {
		block.Nonce = int64(nonce)
	}
	if ver, ok := data["version"].(float64); ok {
		block.Version = int32(ver)
	}
	if merkle, ok := data["merkleroot"].(string); ok {
		block.MerkleRoot = merkle
	}
	if prev, ok := data["previousblockhash"].(string); ok {
		block.Previous = prev
	}
	if next, ok := data["nextblockhash"].(string); ok {
		block.Next = next
	}
	if confs, ok := data["confirmations"].(float64); ok {
		block.Confirmations = int64(confs)
	}
	if txs, ok := data["tx"].([]interface{}); ok {
		block.Tx = make([]string, 0, len(txs))
		for _, tx := range txs {
			if txid, ok := tx.(string); ok {
				block.Tx = append(block.Tx, txid)
			}
		}
	}
	
	// Extract miner from coinbase
	if len(block.Tx) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		txData, _ := s.rpc.GetRawTransaction(ctx, block.Tx[0], true)
		if txData != nil {
			block.Miner = s.extractMiner(txData)
		}
	}
	
	return block
}

// extractMiner extracts pool name from coinbase
func (s *Service) extractMiner(txData map[string]interface{}) string {
	if vin, ok := txData["vin"].([]interface{}); ok && len(vin) > 0 {
		if firstIn, ok := vin[0].(map[string]interface{}); ok {
			if coinbase, ok := firstIn["coinbase"].(string); ok && len(coinbase) > 0 {
				// Try to extract pool name from coinbase
				decoded, _ := hex.DecodeString(coinbase)
				if len(decoded) > 0 {
					return string(decoded)
				}
			}
		}
	}
	return ""
}

// processBlockTransactions processes transactions from a block
func (s *Service) processBlockTransactions(ctx context.Context, block *types.Block) {
	for _, txid := range block.Tx {
		// Check if in mempool
		isInMempool, _ := s.redis.IsInMempool(ctx, txid)
		
		if isInMempool {
			// Mark as confirmed
			s.redis.MarkTransactionConfirmed(ctx, txid, block.Height)
			s.redis.RemoveFromMempool(ctx, txid)
			s.redis.RemoveFullTransaction(ctx, txid)
		} else {
			// Fetch and store
			txData, err := s.rpc.GetRawTransaction(ctx, txid, true)
			if err != nil {
				continue
			}
			
			tx := s.parseTransaction(txData)
			tx.Status = "confirmed"
			tx.BlockHeight = block.Height
			
			s.redis.PushTransaction(ctx, tx)
		}
	}
}

// handleTransaction processes a raw transaction from mempool
func (s *Service) handleTransaction(rawTx []byte) error {
	// Compute txid
	txid := utils.ComputeTxid(rawTx)
	
	// Parse transaction
	tx := &types.Transaction{
		Txid:   txid,
		Status: "mempool",
		Time:   time.Now().Unix(),
		Size:   len(rawTx),
	}
	
	// Fetch full details from RPC
	if s.rpc != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		txData, err := s.rpc.GetRawTransaction(ctx, txid, true)
		if err == nil && txData != nil {
			tx = s.parseTransaction(txData)
			tx.Status = "mempool"
		}
	}
	
	// Store in Redis
	ctx := context.Background()
	if err := s.redis.PushTransaction(ctx, tx); err != nil {
		log.Printf("Failed to store transaction: %v", err)
	}
	
	if err := s.redis.AddToMempool(ctx, txid); err != nil {
		log.Printf("Failed to add to mempool: %v", err)
	}
	
	if err := s.redis.StoreFullTransaction(ctx, txid, tx); err != nil {
		log.Printf("Failed to store full transaction: %v", err)
	}
	
	return nil
}

// parseTransaction parses transaction data from RPC
func (s *Service) parseTransaction(data map[string]interface{}) *types.Transaction {
	tx := &types.Transaction{}
	
	if txid, ok := data["txid"].(string); ok {
		tx.Txid = txid
	}
	if size, ok := data["size"].(float64); ok {
		tx.Size = int(size)
	}
	if time, ok := data["time"].(float64); ok {
		tx.Time = int64(time)
	}
	if blockHeight, ok := data["blockheight"].(float64); ok {
		tx.BlockHeight = int64(blockHeight)
	}
	if confs, ok := data["confirmations"].(float64); ok {
		tx.Confirmations = int64(confs)
	}
	
	// Parse inputs
	if vin, ok := data["vin"].([]interface{}); ok {
		tx.Inputs = make([]types.TransactionInput, 0, len(vin))
		for _, in := range vin {
			if inMap, ok := in.(map[string]interface{}); ok {
				input := types.TransactionInput{}
				if txid, ok := inMap["txid"].(string); ok {
					input.Txid = txid
				}
				if vout, ok := inMap["vout"].(float64); ok {
					input.Vout = int(vout)
				}
				if seq, ok := inMap["sequence"].(float64); ok {
					input.Sequence = uint32(seq)
				}
				if coinbase, ok := inMap["coinbase"].(string); ok {
					input.Coinbase = coinbase
				}
				tx.Inputs = append(tx.Inputs, input)
			}
		}
	}
	
	// Parse outputs and calculate amount
	if vout, ok := data["vout"].([]interface{}); ok {
		tx.Outputs = make([]types.TransactionOutput, 0, len(vout))
		for _, out := range vout {
			if outMap, ok := out.(map[string]interface{}); ok {
				output := types.TransactionOutput{}
				if val, ok := outMap["value"].(float64); ok {
					output.Value = val
					tx.Amount += val
				}
				if n, ok := outMap["n"].(float64); ok {
					output.N = int(n)
				}
				if scriptPubKey, ok := outMap["scriptPubKey"].(map[string]interface{}); ok {
					output.ScriptPubKey = scriptPubKey
				}
				if tokenData, ok := outMap["tokenData"].(map[string]interface{}); ok {
					output.TokenData = tokenData
					tx.HasTokens = true
				}
				tx.Outputs = append(tx.Outputs, output)
			}
		}
	}
	
	return tx
}

// Close closes the service
func (s *Service) Close() {
	if s.zmqSocket != nil {
		s.zmqSocket.Close()
	}
	if s.zmqCtx != nil {
		s.zmqCtx.Term()
	}
	if s.redis != nil {
		s.redis.Close()
	}
	if s.rpc != nil {
		s.rpc.Close()
	}
}

func main() {
	cfg := loadConfig()
	
	service, err := NewService(cfg)
	if err != nil {
		log.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()
	
	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigCh
		log.Println("Shutting down...")
		cancel()
	}()
	
	// Start service
	if err := service.Start(ctx); err != nil {
		log.Fatalf("Service error: %v", err)
	}
}