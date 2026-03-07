package fulcrum

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Client represents a Fulcrum (Electrum-Cash) client with connection pooling
type Client struct {
	host     string
	port     int
	timeout  time.Duration
	
	// Connection pool
	pool     chan *Connection
	maxConns int
	
	// Request tracking
	reqID    uint64
	mu       sync.RWMutex
}

// Connection represents a pooled TCP connection
type Connection struct {
	conn   net.Conn
	reader *bufio.Reader
	mu     sync.Mutex
	lastUsed time.Time
	inUse    bool
}

// Request represents a JSON-RPC request
type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      uint64      `json:"id"`
}

// Response represents a JSON-RPC response
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *Error          `json:"error"`
	ID      uint64          `json:"id"`
}

// Error represents a JSON-RPC error
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("Fulcrum error %d: %s", e.Code, e.Message)
}

// Config holds client configuration
type Config struct {
	Host     string
	Port     int
	Timeout  time.Duration
	MaxConns int
}

// NewClient creates a new Fulcrum client
func NewClient(cfg Config) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxConns == 0 {
		cfg.MaxConns = 100
	}
	
	return &Client{
		host:     cfg.Host,
		port:     cfg.Port,
		timeout:  cfg.Timeout,
		pool:     make(chan *Connection, cfg.MaxConns),
		maxConns: cfg.MaxConns,
	}
}

// getConnection gets a connection from the pool or creates a new one
func (c *Client) getConnection(ctx context.Context) (*Connection, error) {
	select {
	case conn := <-c.pool:
		// Check if connection is still alive
		if time.Since(conn.lastUsed) > 30*time.Second {
			conn.conn.Close()
			return c.createConnection()
		}
		conn.inUse = true
		return conn, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		// Pool empty, create new connection
		return c.createConnection()
	}
}

// createConnection creates a new TCP connection
func (c *Client) createConnection() (*Connection, error) {
	addr := fmt.Sprintf("%s:%d", c.host, c.port)
	conn, err := net.DialTimeout("tcp", addr, c.timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Fulcrum: %w", err)
	}
	
	return &Connection{
		conn:     conn,
		reader:   bufio.NewReader(conn),
		lastUsed: time.Now(),
		inUse:    true,
	}, nil
}

// releaseConnection returns a connection to the pool
func (c *Client) releaseConnection(conn *Connection) {
	conn.inUse = false
	conn.lastUsed = time.Now()
	
	select {
	case c.pool <- conn:
		// Successfully returned to pool
	default:
		// Pool full, close connection
		conn.conn.Close()
	}
}

// Request makes a JSON-RPC request to Fulcrum
func (c *Client) Request(ctx context.Context, method string, params interface{}, result interface{}) error {
	conn, err := c.getConnection(ctx)
	if err != nil {
		return err
	}
	defer c.releaseConnection(conn)
	
	// Generate request ID
	id := atomic.AddUint64(&c.reqID, 1)
	
	req := Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      id,
	}
	
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// Send request with timeout
	conn.mu.Lock()
	defer conn.mu.Unlock()
	
	// Set write deadline
	if err := conn.conn.SetWriteDeadline(time.Now().Add(c.timeout)); err != nil {
		conn.conn.Close()
		return fmt.Errorf("failed to set write deadline: %w", err)
	}
	
	// Send request (line-delimited JSON)
	if _, err := fmt.Fprintf(conn.conn, "%s\n", reqJSON); err != nil {
		conn.conn.Close()
		return fmt.Errorf("failed to send request: %w", err)
	}
	
	// Set read deadline
	if err := conn.conn.SetReadDeadline(time.Now().Add(c.timeout)); err != nil {
		conn.conn.Close()
		return fmt.Errorf("failed to set read deadline: %w", err)
	}
	
	// Read response
	line, err := conn.reader.ReadString('\n')
	if err != nil {
		conn.conn.Close()
		return fmt.Errorf("failed to read response: %w", err)
	}
	
	var resp Response
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}
	
	if resp.Error != nil {
		return resp.Error
	}
	
	if result != nil && resp.Result != nil {
		if err := json.Unmarshal(resp.Result, result); err != nil {
			return fmt.Errorf("failed to unmarshal result: %w", err)
		}
	}
	
	return nil
}

// Close closes all pooled connections
func (c *Client) Close() error {
	close(c.pool)
	for conn := range c.pool {
		conn.conn.Close()
	}
	return nil
}

// ServerVersion gets the server version
func (c *Client) ServerVersion(ctx context.Context) (string, error) {
	var result []interface{}
	err := c.Request(ctx, "server.version", []interface{}{"bchexplorer", "1.4"}, &result)
	if err != nil {
		return "", err
	}
	if len(result) > 0 {
		if s, ok := result[0].(string); ok {
			return s, nil
		}
	}
	return "", nil
}

// GetBlockHeader gets a block header
func (c *Client) GetBlockHeader(ctx context.Context, height int64) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := c.Request(ctx, "blockchain.block.header", []interface{}{height}, &result)
	return result, err
}

// GetBalance gets the balance for a scripthash
func (c *Client) GetBalance(ctx context.Context, scripthash string) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := c.Request(ctx, "blockchain.scripthash.get_balance", []interface{}{scripthash}, &result)
	return result, err
}

// GetHistory gets the transaction history for a scripthash
func (c *Client) GetHistory(ctx context.Context, scripthash string) ([]map[string]interface{}, error) {
	var result []map[string]interface{}
	err := c.Request(ctx, "blockchain.scripthash.get_history", []interface{}{scripthash}, &result)
	return result, err
}

// GetMempool gets the mempool for a scripthash
func (c *Client) GetMempool(ctx context.Context, scripthash string) ([]map[string]interface{}, error) {
	var result []map[string]interface{}
	err := c.Request(ctx, "blockchain.scripthash.get_mempool", []interface{}{scripthash}, &result)
	return result, err
}

// ListUnspent gets unspent outputs for a scripthash
func (c *Client) ListUnspent(ctx context.Context, scripthash string) ([]map[string]interface{}, error) {
	var result []map[string]interface{}
	err := c.Request(ctx, "blockchain.scripthash.listunspent", []interface{}{scripthash}, &result)
	return result, err
}

// GetTransaction gets a transaction by txid
func (c *Client) GetTransaction(ctx context.Context, txid string, verbose bool) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := c.Request(ctx, "blockchain.transaction.get", []interface{}{txid, verbose}, &result)
	return result, err
}

// GetHeight gets the current block height
func (c *Client) GetHeight(ctx context.Context) (int64, error) {
	var result int64
	err := c.Request(ctx, "blockchain.headers.subscribe", []interface{}{}, &result)
	return result, err
}

// Broadcast broadcasts a raw transaction
func (c *Client) Broadcast(ctx context.Context, hex string) (string, error) {
	var result string
	err := c.Request(ctx, "blockchain.transaction.broadcast", []interface{}{hex}, &result)
	return result, err
}

// TokenScripthashGetBalance gets token balance for a scripthash
func (c *Client) TokenScripthashGetBalance(ctx context.Context, scripthash string, tokenID string) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := c.Request(ctx, "token.scripthash.get_balance", []interface{}{scripthash, tokenID}, &result)
	return result, err
}

// TokenAddressGetBalance gets token balance for an address
func (c *Client) TokenAddressGetBalance(ctx context.Context, address string, tokenID string) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := c.Request(ctx, "token.address.get_balance", []interface{}{address, tokenID}, &result)
	return result, err
}

// TokenScripthashGetHistory gets token history for a scripthash
func (c *Client) TokenScripthashGetHistory(ctx context.Context, scripthash string, tokenID string) ([]map[string]interface{}, error) {
	var result []map[string]interface{}
	err := c.Request(ctx, "token.scripthash.get_history", []interface{}{scripthash, tokenID}, &result)
	return result, err
}

// TokenScripthashListUnspent lists token UTXOs for a scripthash
func (c *Client) TokenScripthashListUnspent(ctx context.Context, scripthash string, tokenID string) ([]map[string]interface{}, error) {
	var result []map[string]interface{}
	err := c.Request(ctx, "token.scripthash.listunspent", []interface{}{scripthash, tokenID}, &result)
	return result, err
}

// TokenGenesisInfo gets token genesis info
func (c *Client) TokenGenesisInfo(ctx context.Context, tokenID string) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := c.Request(ctx, "token.genesis.info", []interface{}{tokenID}, &result)
	return result, err
}

// TokenNftList gets NFT list for a category
func (c *Client) TokenNftList(ctx context.Context, category string, cursor string, limit int) (map[string]interface{}, error) {
	params := []interface{}{category}
	if cursor != "" {
		params = append(params, cursor)
	}
	if limit > 0 {
		params = append(params, limit)
	}
	var result map[string]interface{}
	err := c.Request(ctx, "token.nft.list", params, &result)
	return result, err
}

// FormatSatoshis converts satoshis to BCH
func FormatSatoshis(satoshis int64) float64 {
	return float64(satoshis) / 1e8
}

// ParseSatoshis converts BCH to satoshis
func ParseSatoshis(bch float64) int64 {
	return int64(bch * 1e8)
}

// ParseInt64 parses an interface{} to int64
func ParseInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case float64:
		return int64(val)
	case int:
		return int64(val)
	case string:
		n, _ := strconv.ParseInt(val, 10, 64)
		return n
	default:
		return 0
	}
}

// ParseFloat64 parses an interface{} to float64
func ParseFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int64:
		return float64(val)
	case int:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	default:
		return 0
	}
}