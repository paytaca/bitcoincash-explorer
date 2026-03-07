package utils

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchutil"
)

// CashAddress prefixes
const (
	MainnetPrefix = "bitcoincash"
	TestnetPrefix = "bchtest"
	ChipnetPrefix = "bchtest"
	RegtestPrefix = "bchreg"
)

// ValidateCashAddress validates a CashAddress
func ValidateCashAddress(addr string) (bool, string) {
	addr = strings.ToLower(addr)

	// Decode using bchutil with chaincfg params
	decodedAddr, err := bchutil.DecodeAddress(addr, &chaincfg.MainNetParams)
	if err != nil {
		// Try testnet/chipnet
		decodedAddr, err = bchutil.DecodeAddress(addr, &chaincfg.TestNet3Params)
		if err != nil {
			// Try regtest
			decodedAddr, err = bchutil.DecodeAddress(addr, &chaincfg.RegressionNetParams)
			if err != nil {
				return false, ""
			}
		}
	}

	// Determine address type
	switch decodedAddr.(type) {
	case *bchutil.AddressPubKeyHash:
		return true, "P2PKH"
	case *bchutil.AddressScriptHash:
		return true, "P2SH"
	default:
		return true, "Unknown"
	}
}

// AddressToScripthash converts a CashAddress to scripthash for Fulcrum
func AddressToScripthash(addr string) (string, error) {
	addr = strings.ToLower(addr)

	// Try mainnet first
	decodedAddr, err := bchutil.DecodeAddress(addr, &chaincfg.MainNetParams)
	if err != nil {
		// Try testnet/chipnet
		decodedAddr, err = bchutil.DecodeAddress(addr, &chaincfg.TestNet3Params)
		if err != nil {
			// Try regtest
			decodedAddr, err = bchutil.DecodeAddress(addr, &chaincfg.RegressionNetParams)
			if err != nil {
				return "", fmt.Errorf("failed to decode address: %w", err)
			}
		}
	}

	// Get the script bytes
	script := decodedAddr.ScriptAddress()

	// Build the scriptPubKey
	var scriptPubKey []byte
	switch decodedAddr.(type) {
	case *bchutil.AddressPubKeyHash:
		// P2PKH: OP_DUP OP_HASH160 <20-byte hash> OP_EQUALVERIFY OP_CHECKSIG
		scriptPubKey = []byte{0x76, 0xa9, 0x14}
		scriptPubKey = append(scriptPubKey, script...)
		scriptPubKey = append(scriptPubKey, 0x88, 0xac)
	case *bchutil.AddressScriptHash:
		// P2SH: OP_HASH160 <20-byte hash> OP_EQUAL
		scriptPubKey = []byte{0xa9, 0x14}
		scriptPubKey = append(scriptPubKey, script...)
		scriptPubKey = append(scriptPubKey, 0x87)
	default:
		return "", fmt.Errorf("unsupported address type")
	}

	// Double SHA256
	h1 := sha256.Sum256(scriptPubKey)
	h2 := sha256.Sum256(h1[:])

	// Reverse byte order (Fulcrum expects reversed hash)
	scripthash := make([]byte, 32)
	for i := 0; i < 32; i++ {
		scripthash[i] = h2[31-i]
	}

	return hex.EncodeToString(scripthash), nil
}

// GetAddressType returns the address type from version byte
func GetAddressType(version byte) string {
	switch version {
	case 0:
		return "P2PKH"
	case 1:
		return "P2SH"
	case 2, 3:
		return "P2PKH-TokenAware"
	case 4, 5:
		return "P2SH-TokenAware"
	default:
		return "Unknown"
	}
}

// ReverseHex reverses a hex string byte order
func ReverseHex(s string) (string, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return "", err
	}

	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return hex.EncodeToString(b), nil
}

// DoubleSha256 computes double SHA256
func DoubleSha256(data []byte) []byte {
	h1 := sha256.Sum256(data)
	h2 := sha256.Sum256(h1[:])
	return h2[:]
}

// ComputeTxid computes transaction ID from raw transaction bytes
func ComputeTxid(rawTx []byte) string {
	hash := DoubleSha256(rawTx)
	// Reverse byte order for display
	reversed := make([]byte, 32)
	for i := 0; i < 32; i++ {
		reversed[i] = hash[31-i]
	}
	return hex.EncodeToString(reversed)
}

// ParseVarInt parses a variable length integer from bytes
func ParseVarInt(data []byte, offset int) (uint64, int, error) {
	if offset >= len(data) {
		return 0, 0, errors.New("insufficient data for varint")
	}

	b := data[offset]
	switch {
	case b < 0xfd:
		return uint64(b), 1, nil
	case b == 0xfd:
		if offset+3 > len(data) {
			return 0, 0, errors.New("insufficient data for varint")
		}
		return uint64(binary.LittleEndian.Uint16(data[offset+1:])), 3, nil
	case b == 0xfe:
		if offset+5 > len(data) {
			return 0, 0, errors.New("insufficient data for varint")
		}
		return uint64(binary.LittleEndian.Uint32(data[offset+1:])), 5, nil
	default:
		if offset+9 > len(data) {
			return 0, 0, errors.New("insufficient data for varint")
		}
		return binary.LittleEndian.Uint64(data[offset+1:]), 9, nil
	}
}

// ReadBytes reads n bytes from data at offset
func ReadBytes(data []byte, offset, n int) ([]byte, int, error) {
	if offset+n > len(data) {
		return nil, 0, errors.New("insufficient data")
	}
	return data[offset : offset+n], n, nil
}

// FormatBCH formats satoshis to BCH amount
func FormatBCH(satoshis int64) float64 {
	return float64(satoshis) / 1e8
}

// ParseBCH parses BCH amount to satoshis
func ParseBCH(amount float64) int64 {
	return int64(amount * 1e8)
}

// FormatTimestamp formats a Unix timestamp to human readable string
func FormatTimestamp(timestamp int64) string {
	return ""
}

// Min returns the minimum of two integers
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Max returns the maximum of two integers
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
