package bchrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sony/gobreaker"
)

// Client represents a BCH RPC client
type Client struct {
	url      string
	username string
	password string
	client   *http.Client
	
	// Concurrency control
	sem      chan struct{}
	maxConns int
	
	// Request deduplication
	inFlight map[string]*inFlightRequest
	ifMu     sync.RWMutex
	
	// Circuit breaker
	cb       *gobreaker.CircuitBreaker
}

// inFlightRequest tracks in-flight RPC calls
type inFlightRequest struct {
	resp interface{}
	err  error
	done chan struct{}
}

// RPCRequest represents a JSON-RPC request
type RPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      int         `json:"id"`
}

// RPCResponse represents a JSON-RPC response
type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *RPCError       `json:"error"`
	ID      int             `json:"id"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("RPC error %d: %s", e.Code, e.Message)
}

// Config holds client configuration
type Config struct {
	URL         string
	Username    string
	Password    string
	Timeout     time.Duration
	MaxConns    int
}

// NewClient creates a new BCH RPC client
func NewClient(cfg Config) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxConns == 0 {
		cfg.MaxConns = 20
	}
	
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "bch-rpc",
		MaxRequests: 3,
		Interval:    30 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			fmt.Printf("Circuit breaker %s: %s -> %s\n", name, from, to)
		},
	})
	
	c := &Client{
		url:      cfg.URL,
		username: cfg.Username,
		password: cfg.Password,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		sem:      make(chan struct{}, cfg.MaxConns),
		maxConns: cfg.MaxConns,
		inFlight: make(map[string]*inFlightRequest),
		cb:       cb,
	}
	
	// Start cleanup goroutine for in-flight requests
	go c.cleanupInFlight()
	
	return c
}

// cleanupInFlight periodically cleans up stale in-flight requests
func (c *Client) cleanupInFlight() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		c.ifMu.Lock()
		for key, req := range c.inFlight {
			select {
			case <-req.done:
				delete(c.inFlight, key)
			default:
			}
		}
		c.ifMu.Unlock()
	}
}

// requestKey generates a unique key for request deduplication
func requestKey(method string, params interface{}) string {
	paramsJSON, _ := json.Marshal(params)
	return method + ":" + string(paramsJSON)
}

// Call makes an RPC call with deduplication and circuit breaker
func (c *Client) Call(ctx context.Context, method string, params interface{}, result interface{}) error {
	key := requestKey(method, params)
	
	// Check for in-flight request
	c.ifMu.RLock()
	if req, ok := c.inFlight[key]; ok {
		c.ifMu.RUnlock()
		// Wait for existing request
		select {
		case <-req.done:
			if req.err != nil {
				return req.err
			}
			// Copy result
			resultJSON, _ := json.Marshal(req.resp)
			return json.Unmarshal(resultJSON, result)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	c.ifMu.RUnlock()
	
	// Create new in-flight request
	c.ifMu.Lock()
	req := &inFlightRequest{
		done: make(chan struct{}),
	}
	c.inFlight[key] = req
	c.ifMu.Unlock()
	
	// Execute request
	req.err = c.callWithBreaker(ctx, method, params, &req.resp)
	close(req.done)
	
	// Clean up after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		c.ifMu.Lock()
		delete(c.inFlight, key)
		c.ifMu.Unlock()
	}()
	
	if req.err != nil {
		return req.err
	}
	
	// Copy result to caller's result
	resultJSON, _ := json.Marshal(req.resp)
	return json.Unmarshal(resultJSON, result)
}

// callWithBreaker makes RPC call with circuit breaker
func (c *Client) callWithBreaker(ctx context.Context, method string, params interface{}, result interface{}) error {
	resp, err := c.cb.Execute(func() (interface{}, error) {
		return nil, c.call(ctx, method, params, result)
	})
	
	if err != nil {
		return err
	}
	
	_ = resp
	return nil
}

// call makes the actual HTTP RPC call
func (c *Client) call(ctx context.Context, method string, params interface{}, result interface{}) error {
	// Acquire semaphore slot
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return ctx.Err()
	}
	
	reqBody := RPCRequest{
		JSONRPC: "1.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}
	
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	if c.username != "" || c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}
	
	var rpcResp RPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}
	
	if rpcResp.Error != nil {
		// Check for work queue depth exceeded (circuit breaker trigger)
		if strings.Contains(rpcResp.Error.Message, "work queue depth exceeded") {
			return fmt.Errorf("WORK_QUEUE_DEPTH_EXCEEDED: %s", rpcResp.Error.Message)
		}
		return rpcResp.Error
	}
	
	if result != nil && rpcResp.Result != nil {
		if err := json.Unmarshal(rpcResp.Result, result); err != nil {
			return fmt.Errorf("failed to unmarshal result: %w", err)
		}
	}
	
	return nil
}

// GetBlockCount returns the current block count
func (c *Client) GetBlockCount(ctx context.Context) (int64, error) {
	var result int64
	err := c.Call(ctx, "getblockcount", []interface{}{}, &result)
	return result, err
}

// GetBlockHash returns the block hash for a given height
func (c *Client) GetBlockHash(ctx context.Context, height int64) (string, error) {
	var result string
	err := c.Call(ctx, "getblockhash", []interface{}{height}, &result)
	return result, err
}

// GetBlock returns a block by hash
func (c *Client) GetBlock(ctx context.Context, hash string, verbosity int) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := c.Call(ctx, "getblock", []interface{}{hash, verbosity}, &result)
	return result, err
}

// GetRawTransaction returns a raw transaction
func (c *Client) GetRawTransaction(ctx context.Context, txid string, verbose bool) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := c.Call(ctx, "getrawtransaction", []interface{}{txid, verbose}, &result)
	return result, err
}

// GetRawMempool returns the mempool
func (c *Client) GetRawMempool(ctx context.Context, verbose bool) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := c.Call(ctx, "getrawmempool", []interface{}{verbose}, &result)
	return result, err
}

// SendRawTransaction broadcasts a raw transaction
func (c *Client) SendRawTransaction(ctx context.Context, hex string) (string, error) {
	var result string
	err := c.Call(ctx, "sendrawtransaction", []interface{}{hex}, &result)
	return result, err
}

// GetBlockchainInfo returns blockchain information
func (c *Client) GetBlockchainInfo(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := c.Call(ctx, "getblockchaininfo", []interface{}{}, &result)
	return result, err
}

// GetNetworkInfo returns network information
func (c *Client) GetNetworkInfo(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := c.Call(ctx, "getnetworkinfo", []interface{}{}, &result)
	return result, err
}

// GetMempoolInfo returns mempool information
func (c *Client) GetMempoolInfo(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := c.Call(ctx, "getmempoolinfo", []interface{}{}, &result)
	return result, err
}

// Close closes the client
func (c *Client) Close() error {
	c.client.CloseIdleConnections()
	return nil
}