package bcmr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"bchexplorer/internal/types"
)

type Client struct {
	baseURL string
	client  *http.Client
}

type Config struct {
	BaseURL string
	Timeout time.Duration
}

type tokenResponseToken struct {
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
}

type tokenResponse struct {
	Name  string             `json:"name"`
	Token tokenResponseToken `json:"token"`
	Error string             `json:"error"`
}

func NewClient(cfg Config) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = 15 * time.Second
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://bcmr.paytaca.com"
	}

	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")

	return &Client{
		baseURL: cfg.BaseURL,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

func (c *Client) GetTokenMetadata(ctx context.Context, category string) (*types.TokenMetadata, error) {
	url := fmt.Sprintf("%s/api/tokens/%s/", c.baseURL, category)

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

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	if tokenResp.Error != "" {
		return nil, fmt.Errorf("BCMR: %s", tokenResp.Error)
	}

	if tokenResp.Token.Symbol == "" && tokenResp.Name == "" {
		return nil, fmt.Errorf("no token metadata found")
	}

	return &types.TokenMetadata{
		Category: category,
		Name:     tokenResp.Name,
		Symbol:   tokenResp.Token.Symbol,
		Decimals: tokenResp.Token.Decimals,
	}, nil
}
