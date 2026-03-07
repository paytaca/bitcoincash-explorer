package bcmr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"bchexplorer/internal/types"
)

// Client represents a BCMR client
type Client struct {
	baseURL string
	client  *http.Client
}

// Config holds BCMR configuration
type Config struct {
	BaseURL string
	Timeout time.Duration
}

// NewClient creates a new BCMR client
func NewClient(cfg Config) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = 15 * time.Second
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://bcmr.paytaca.com"
	}
	
	return &Client{
		baseURL: cfg.BaseURL,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// RegistryResponse represents a BCMR registry response
type RegistryResponse struct {
	Version   string                 `json:"version"`
	RegistryName string              `json:"registryName"`
	Schemas   map[string]interface{} `json:"$schemas"`
	Identities map[string]Identity  `json:"identities"`
}

// Identity represents an identity in BCMR
type Identity struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Token       *TokenInfo             `json:"token,omitempty"`
	URIs        map[string]string      `json:"uris,omitempty"`
	Extensions  map[string]interface{} `json:"extensions,omitempty"`
}

// TokenInfo represents token information
type TokenInfo struct {
	Category       string                 `json:"category"`
	Symbol         string                 `json:"symbol"`
	Decimals       int                    `json:"decimals"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description,omitempty"`
	TokenIcon      *IconInfo              `json:"tokenIcon,omitempty"`
	NftTypes       map[string]interface{} `json:"nftTypes,omitempty"`
}

// IconInfo represents icon information
type IconInfo struct {
	URL string `json:"url,omitempty"`
}

// GetTokenMetadata fetches token metadata by category
func (c *Client) GetTokenMetadata(ctx context.Context, category string) (*types.TokenMetadata, error) {
	// Try multiple URL patterns for compatibility
	urls := []string{
		fmt.Sprintf("%s/api/registries/%s", c.baseURL, category),
		fmt.Sprintf("%s/%s.json", c.baseURL, category),
		fmt.Sprintf("%s/tokens/%s", c.baseURL, category),
	}
	
	var lastErr error
	for _, url := range urls {
		metadata, err := c.fetchMetadata(ctx, url, category)
		if err == nil {
			return metadata, nil
		}
		lastErr = err
	}
	
	return nil, lastErr
}

// fetchMetadata attempts to fetch metadata from a specific URL
func (c *Client) fetchMetadata(ctx context.Context, url, category string) (*types.TokenMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Accept", "application/json")
	
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	
	var registry RegistryResponse
	if err := json.NewDecoder(resp.Body).Decode(&registry); err != nil {
		return nil, err
	}
	
	// Find identity for this category
	identity, ok := registry.Identities[category]
	if !ok {
		return nil, fmt.Errorf("category not found in registry")
	}
	
	metadata := &types.TokenMetadata{
		Category: category,
	}
	
	if identity.Token != nil {
		metadata.Name = identity.Token.Name
		metadata.Symbol = identity.Token.Symbol
		metadata.Decimals = identity.Token.Decimals
	} else {
		metadata.Name = identity.Name
	}
	
	return metadata, nil
}

// GetTokenMetadataFromRegistry fetches from the full registry
func (c *Client) GetTokenMetadataFromRegistry(ctx context.Context, category string) (*types.TokenMetadata, error) {
	url := fmt.Sprintf("%s/.well-known/bitcoin-cash-metadata-registry.json", c.baseURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	
	var registry RegistryResponse
	if err := json.NewDecoder(resp.Body).Decode(&registry); err != nil {
		return nil, err
	}
	
	identity, ok := registry.Identities[category]
	if !ok {
		return nil, fmt.Errorf("category not found")
	}
	
	metadata := &types.TokenMetadata{
		Category: category,
	}
	
	if identity.Token != nil {
		metadata.Name = identity.Token.Name
		metadata.Symbol = identity.Token.Symbol
		metadata.Decimals = identity.Token.Decimals
	} else {
		metadata.Name = identity.Name
	}
	
	return metadata, nil
}

// BatchGetTokenMetadata fetches metadata for multiple categories
func (c *Client) BatchGetTokenMetadata(ctx context.Context, categories []string) (map[string]*types.TokenMetadata, error) {
	result := make(map[string]*types.TokenMetadata)
	
	for _, category := range categories {
		metadata, err := c.GetTokenMetadata(ctx, category)
		if err != nil {
			continue
		}
		result[category] = metadata
	}
	
	return result, nil
}