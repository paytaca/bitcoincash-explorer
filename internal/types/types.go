package types

import "time"

// Block represents a blockchain block
type Block struct {
	Hash          string   `json:"hash"`
	Height        int64    `json:"height"`
	Time          int64    `json:"time"`
	Size          int      `json:"size"`
	TxCount       int      `json:"txCount"`
	Miner         string   `json:"miner,omitempty"`
	Difficulty    float64  `json:"difficulty,omitempty"`
	Bits          string   `json:"bits,omitempty"`
	Nonce         int64    `json:"nonce,omitempty"`
	Version       int32    `json:"version,omitempty"`
	MerkleRoot    string   `json:"merkleRoot,omitempty"`
	Tx            []string `json:"tx,omitempty"`
	Previous      string   `json:"previousblockhash,omitempty"`
	Next          string   `json:"nextblockhash,omitempty"`
	Confirmations int64    `json:"confirmations,omitempty"`
}

// Transaction represents a blockchain transaction
type Transaction struct {
	Txid          string                 `json:"txid"`
	Status        string                 `json:"status"` // confirmed, mempool
	Time          int64                  `json:"time"`
	Amount        float64                `json:"amount,omitempty"`
	HasTokens     bool                   `json:"hasTokens"`
	Fee           float64                `json:"fee,omitempty"`
	Size          int                    `json:"size,omitempty"`
	BlockHeight   int64                  `json:"blockHeight,omitempty"`
	Confirmations int64                  `json:"confirmations,omitempty"`
	Inputs        []TransactionInput     `json:"vin,omitempty"`
	Outputs       []TransactionOutput    `json:"vout,omitempty"`
	TokenData     map[string]interface{} `json:"tokenMeta,omitempty"`
}

// TransactionInput represents a transaction input
type TransactionInput struct {
	Txid         string                 `json:"txid"`
	Vout         int                    `json:"vout"`
	Value        float64                `json:"value,omitempty"`
	ScriptSig    map[string]interface{} `json:"scriptSig,omitempty"`
	Sequence     uint32                 `json:"sequence"`
	Coinbase     string                 `json:"coinbase,omitempty"`
	ScriptPubKey map[string]interface{} `json:"scriptPubKey,omitempty"`
	TokenData    map[string]interface{} `json:"tokenData,omitempty"`
}

// TransactionOutput represents a transaction output
type TransactionOutput struct {
	Value        float64                `json:"value"`
	N            int                    `json:"n"`
	ScriptPubKey map[string]interface{} `json:"scriptPubKey,omitempty"`
	TokenData    map[string]interface{} `json:"tokenData,omitempty"`
}

// TokenMetadata represents BCMR token metadata
type TokenMetadata struct {
	Name     string `json:"name,omitempty"`
	Symbol   string `json:"symbol,omitempty"`
	Decimals int    `json:"decimals,omitempty"`
	IconURL  string `json:"iconUrl,omitempty"`
	Category string `json:"category,omitempty"`
}

// NFTMetadata represents BCMR NFT metadata with commitment
type NFTMetadata struct {
	Category     string            `json:"category,omitempty"`
	Commitment   string            `json:"commitment,omitempty"`
	Name         string            `json:"name,omitempty"`
	Description  string            `json:"description,omitempty"`
	Symbol       string            `json:"symbol,omitempty"`
	Decimals     int               `json:"decimals,omitempty"`
	URIs         map[string]string `json:"uris,omitempty"`
	IsNFT        bool              `json:"is_nft,omitempty"`
	NFTType      string            `json:"nft_type,omitempty"`
	TypeMetadata *NFTTypeMetadata  `json:"type_metadata,omitempty"`
}

// NFTTypeMetadata represents commitment-specific NFT metadata
type NFTTypeMetadata struct {
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	URIs        map[string]string `json:"uris,omitempty"`
}

// AddressBalance represents an address balance
type AddressBalance struct {
	Confirmed   float64                 `json:"confirmed"`
	Unconfirmed float64                 `json:"unconfirmed"`
	Tokens      map[string]TokenBalance `json:"tokens,omitempty"`
}

// TokenBalance represents token balance for an address
type TokenBalance struct {
	Category string  `json:"category"`
	Amount   float64 `json:"amount"`
	Decimals int     `json:"decimals"`
	Symbol   string  `json:"symbol,omitempty"`
	Name     string  `json:"name,omitempty"`
}

// AddressTransaction represents a transaction for an address
type AddressTransaction struct {
	Txid          string  `json:"txid"`
	Height        int64   `json:"height"`
	Fee           float64 `json:"fee,omitempty"`
	Value         float64 `json:"value"`
	Time          int64   `json:"time,omitempty"`
	Confirmations int64   `json:"confirmations,omitempty"`
}

// NodeStatus represents the status of BCH node and Fulcrum
type NodeStatus struct {
	Node struct {
		Connected       bool    `json:"connected"`
		BlockHeight     int64   `json:"blockHeight"`
		Headers         int64   `json:"headers"`
		Difficulty      float64 `json:"difficulty,omitempty"`
		Version         int     `json:"version,omitempty"`
		ProtocolVersion int     `json:"protocolVersion,omitempty"`
	} `json:"node"`
	Fulcrum struct {
		Connected   bool  `json:"connected"`
		BlockHeight int64 `json:"blockHeight"`
	}
	Redis struct {
		Connected   bool  `json:"connected"`
		LatestBlock int64 `json:"latestBlock"`
	}
	InSync bool `json:"inSync"`
}

// BroadcastResult represents the result of a broadcast
type BroadcastResult struct {
	Txid    string `json:"txid,omitempty"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// MempoolInfo represents mempool information
type MempoolInfo struct {
	Size  int   `json:"size"`
	Bytes int64 `json:"bytes"`
	Usage int64 `json:"usage,omitempty"`
}

// CacheEntry represents a cached item with TTL
type CacheEntry struct {
	Data      interface{} `json:"data"`
	ExpiresAt time.Time   `json:"expiresAt"`
}

// ZMQBlock represents a block received via ZMQ
type ZMQBlock struct {
	Hash     string
	Height   int64
	Time     int64
	Size     int
	TxCount  int
	Txids    []string
	Previous string
	Raw      []byte
}

// ZMQTransaction represents a transaction received via ZMQ
type ZMQTransaction struct {
	Txid      string
	Size      int
	Amount    float64
	HasTokens bool
	Raw       []byte
}
