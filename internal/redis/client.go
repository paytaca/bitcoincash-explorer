package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"bchexplorer/internal/types"
	"github.com/redis/go-redis/v9"
)

// Client wraps go-redis with BCH-specific data structures
type Client struct {
	client *redis.Client
	prefix string
}

// Config holds Redis configuration
type Config struct {
	URL    string
	Prefix string
}

// NewClient creates a new Redis client
func NewClient(cfg Config) (*Client, error) {
	opt, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Client{
		client: client,
		prefix: cfg.Prefix,
	}, nil
}

// key generates a prefixed key
func (c *Client) key(suffix string) string {
	if c.prefix == "" {
		return suffix
	}
	return fmt.Sprintf("%s:%s", c.prefix, suffix)
}

// PushBlock adds a block to the latest blocks list
func (c *Client) PushBlock(ctx context.Context, block *types.Block) error {
	data, err := json.Marshal(block)
	if err != nil {
		return err
	}

	log.Printf("[Redis] Pushing block height=%d miner=%s", block.Height, block.Miner)

	pipe := c.client.Pipeline()
	pipe.LPush(ctx, c.key("blocks:latest"), data)
	pipe.LTrim(ctx, c.key("blocks:latest"), 0, 14) // Keep last 15
	_, err = pipe.Exec(ctx)
	return err
}

// GetBlocks returns the latest blocks
func (c *Client) GetBlocks(ctx context.Context, limit int64) ([]*types.Block, error) {
	data, err := c.client.LRange(ctx, c.key("blocks:latest"), 0, limit-1).Result()
	if err != nil {
		return nil, err
	}

	blocks := make([]*types.Block, 0, len(data))
	for _, d := range data {
		var block types.Block
		if err := json.Unmarshal([]byte(d), &block); err != nil {
			continue
		}
		log.Printf("[Redis] Retrieved block height=%d miner=%s", block.Height, block.Miner)
		blocks = append(blocks, &block)
	}

	return blocks, nil
}

// SetBlocks replaces the cached blocks list with the provided blocks (latest first)
func (c *Client) SetBlocks(ctx context.Context, blocks []*types.Block) error {
	pipe := c.client.Pipeline()
	key := c.key("blocks:latest")
	pipe.Del(ctx, key)
	for i := len(blocks) - 1; i >= 0; i-- {
		data, err := json.Marshal(blocks[i])
		if err != nil {
			continue
		}
		pipe.LPush(ctx, key, data)
	}
	pipe.LTrim(ctx, key, 0, 14)
	_, err := pipe.Exec(ctx)
	return err
}

// GetLatestBlock returns the most recent block
func (c *Client) GetLatestBlock(ctx context.Context) (*types.Block, error) {
	data, err := c.client.LIndex(ctx, c.key("blocks:latest"), 0).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var block types.Block
	if err := json.Unmarshal([]byte(data), &block); err != nil {
		return nil, err
	}

	return &block, nil
}

// RemoveBlock removes a block from the list (for reorgs)
func (c *Client) RemoveBlock(ctx context.Context, hash string) error {
	blocks, err := c.GetBlocks(ctx, 100)
	if err != nil {
		return err
	}

	// Remove all blocks up to and including the specified hash
	found := false
	for _, block := range blocks {
		if found || block.Hash == hash {
			found = true
			data, _ := json.Marshal(block)
			c.client.LRem(ctx, c.key("blocks:latest"), 0, data)
		}
	}

	return nil
}

// PushTransaction adds a transaction to the recent transactions list
func (c *Client) PushTransaction(ctx context.Context, tx *types.Transaction) error {
	data, err := json.Marshal(tx)
	if err != nil {
		return err
	}

	pipe := c.client.Pipeline()
	pipe.LPush(ctx, c.key("txs:latest"), data)
	pipe.LTrim(ctx, c.key("txs:latest"), 0, 19) // Keep last 20
	_, err = pipe.Exec(ctx)
	return err
}

// GetTransactions returns recent transactions
func (c *Client) GetTransactions(ctx context.Context, limit int64) ([]*types.Transaction, error) {
	data, err := c.client.LRange(ctx, c.key("txs:latest"), 0, limit-1).Result()
	if err != nil {
		return nil, err
	}

	txs := make([]*types.Transaction, 0, len(data))
	for _, d := range data {
		var tx types.Transaction
		if err := json.Unmarshal([]byte(d), &tx); err != nil {
			continue
		}
		txs = append(txs, &tx)
	}

	return txs, nil
}

// MarkTransactionConfirmed updates a transaction's status to confirmed
func (c *Client) MarkTransactionConfirmed(ctx context.Context, txid string, blockHeight int64) error {
	// Get all transactions
	txs, err := c.GetTransactions(ctx, 100)
	if err != nil {
		return err
	}

	// Find and update
	for i, tx := range txs {
		if tx.Txid == txid {
			txs[i].Status = "confirmed"
			txs[i].BlockHeight = blockHeight

			// Update in list
			data, _ := json.Marshal(txs[i])
			c.client.LSet(ctx, c.key("txs:latest"), int64(i), data)
			break
		}
	}

	return nil
}

// AddToMempool adds a txid to the mempool set
func (c *Client) AddToMempool(ctx context.Context, txid string) error {
	return c.client.SAdd(ctx, c.key("mempool:txids"), txid).Err()
}

// RemoveFromMempool removes a txid from the mempool set
func (c *Client) RemoveFromMempool(ctx context.Context, txid string) error {
	return c.client.SRem(ctx, c.key("mempool:txids"), txid).Err()
}

// IsInMempool checks if a txid is in the mempool
func (c *Client) IsInMempool(ctx context.Context, txid string) (bool, error) {
	return c.client.SIsMember(ctx, c.key("mempool:txids"), txid).Result()
}

// GetMempoolTxids returns all mempool txids
func (c *Client) GetMempoolTxids(ctx context.Context) ([]string, error) {
	return c.client.SMembers(ctx, c.key("mempool:txids")).Result()
}

// StoreFullTransaction stores full transaction details with TTL
func (c *Client) StoreFullTransaction(ctx context.Context, txid string, tx *types.Transaction) error {
	data, err := json.Marshal(tx)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, c.key(fmt.Sprintf("tx:%s", txid)), data, 15*time.Minute).Err()
}

// GetFullTransaction gets full transaction details
func (c *Client) GetFullTransaction(ctx context.Context, txid string) (*types.Transaction, error) {
	data, err := c.client.Get(ctx, c.key(fmt.Sprintf("tx:%s", txid))).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var tx types.Transaction
	if err := json.Unmarshal([]byte(data), &tx); err != nil {
		return nil, err
	}

	return &tx, nil
}

// RemoveFullTransaction removes full transaction details
func (c *Client) RemoveFullTransaction(ctx context.Context, txid string) error {
	return c.client.Del(ctx, c.key(fmt.Sprintf("tx:%s", txid))).Err()
}

// CacheSet sets a cached value with TTL
func (c *Client) CacheSet(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, c.key(fmt.Sprintf("cache:%s", key)), data, ttl).Err()
}

// CacheGet gets a cached value
func (c *Client) CacheGet(ctx context.Context, key string, result interface{}) (bool, error) {
	data, err := c.client.Get(ctx, c.key(fmt.Sprintf("cache:%s", key))).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if err := json.Unmarshal([]byte(data), result); err != nil {
		return false, err
	}

	return true, nil
}

// CacheDelete deletes a cached value
func (c *Client) CacheDelete(ctx context.Context, key string) error {
	return c.client.Del(ctx, c.key(fmt.Sprintf("cache:%s", key))).Err()
}

// GetTokenMetadata gets token metadata from cache
func (c *Client) GetTokenMetadata(ctx context.Context, category string) (*types.TokenMetadata, error) {
	data, err := c.client.Get(ctx, c.key(fmt.Sprintf("bcmr:%s", category))).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var metadata types.TokenMetadata
	if err := json.Unmarshal([]byte(data), &metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

// SetTokenMetadata caches token metadata
func (c *Client) SetTokenMetadata(ctx context.Context, category string, metadata *types.TokenMetadata) error {
	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	// Cache for 24 hours
	return c.client.Set(ctx, c.key(fmt.Sprintf("bcmr:%s", category)), data, 24*time.Hour).Err()
}

// DeleteTokenMetadata deletes cached token metadata
func (c *Client) DeleteTokenMetadata(ctx context.Context, category string) error {
	return c.client.Del(ctx, c.key(fmt.Sprintf("bcmr:%s", category))).Err()
}

// GetBlockCount gets the stored block count
func (c *Client) GetBlockCount(ctx context.Context) (int64, error) {
	val, err := c.client.Get(ctx, c.key("blockcount")).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

// SetBlockCount sets the block count
func (c *Client) SetBlockCount(ctx context.Context, count int64) error {
	return c.client.Set(ctx, c.key("blockcount"), count, 0).Err()
}

// Close closes the Redis connection
func (c *Client) Close() error {
	return c.client.Close()
}

// Ping checks if Redis is connected
func (c *Client) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}
