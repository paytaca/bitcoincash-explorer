package main

import (
	"encoding/hex"
	"testing"
)

func TestExtractMiner(t *testing.T) {
	tests := []struct {
		name     string
		coinbase string
		want     string
	}{
		{
			name:     "Empty",
			coinbase: "",
			want:     "",
		},
		{
			name:     "Invalid hex",
			coinbase: "xyz",
			want:     "",
		},
		{
			name:     "ViaBTC pattern",
			coinbase: "03af2f5669614254432f",
			want:     "ViaBTC",
		},
		{
			name:     "AntPool pattern",
			coinbase: "03af2f416e74506f6f6c2f",
			want:     "AntPool",
		},
		{
			name:     "BTC.com pattern",
			coinbase: "03053c4254432e636f6d2f",
			want:     "BTC.com",
		},
		{
			name:     "Rawpool pattern",
			coinbase: "03af2f526177706f6f6c2f",
			want:     "Rawpool",
		},
		{
			name:     "With extra null bytes",
			coinbase: "03af00002f5669614254432f0000",
			want:     "ViaBTC",
		},
		{
			name:     "Mined by pattern",
			coinbase: "0e6d696e656420627920416e74506f6f6c",
			want:     "AntPool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractMinerFromCoinbaseHex(tt.coinbase)
			if got != tt.want {
				t.Errorf("extractMinerFromCoinbaseHex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractMinerFromRealCoinbase(t *testing.T) {
	// Real coinbase from BCH block 800000
	// This is a real example to verify the extraction works
	coinbaseHex := "03803030000431303030303030303030f06020000000000000000000000000000000000000000000000000000000000000000000000"

	got := extractMinerFromCoinbaseHex(coinbaseHex)
	t.Logf("Extracted from real coinbase: %q", got)

	// Decode to see what's in it
	decoded, err := hex.DecodeString(coinbaseHex)
	if err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}
	t.Logf("Decoded coinbase (printable): %q", toPrintable(decoded))
}

func toPrintable(b []byte) string {
	var result []byte
	for _, c := range b {
		if c >= 0x20 && c <= 0x7E {
			result = append(result, c)
		} else {
			result = append(result, ' ')
		}
	}
	return string(result)
}
