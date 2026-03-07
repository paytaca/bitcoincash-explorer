package utils

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// CashAddress prefixes
const (
	MainnetPrefix = "bitcoincash"
	TestnetPrefix = "bchtest"
	ChipnetPrefix = "bchtest"
	RegtestPrefix = "bchreg"
)

// CashAddress charset
const charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

// PolyMod computes the BCH checksum polynomial
func PolyMod(v []byte) uint64 {
	c := uint64(1)
	for _, b := range v {
		c0 := byte(c >> 35)
		c = ((c & 0x07ffffffff) << 5) ^ uint64(b)
		if c0&0x01 != 0 {
			c ^= 0x98f2bc8e61
		}
		if c0&0x02 != 0 {
			c ^= 0x79b76d99e2
		}
		if c0&0x04 != 0 {
			c ^= 0xf33e5fb3c4
		}
		if c0&0x08 != 0 {
			c ^= 0xae2eabe2a8
		}
		if c0&0x10 != 0 {
			c ^= 0x1e4f43e470
		}
	}
	return c ^ 1
}

// ExpandPrefix expands the human-readable part for checksum computation
func ExpandPrefix(prefix string) []byte {
	result := make([]byte, len(prefix)*2+1)
	for i, c := range prefix {
		result[i] = byte(c >> 5)
		result[i+len(prefix)+1] = byte(c & 31)
	}
	result[len(prefix)] = 0
	return result
}

// ConvertBits converts data from one bit width to another
func ConvertBits(data []byte, fromBits, toBits uint, pad bool) ([]byte, error) {
	acc := uint32(0)
	bits := uint(0)
	ret := make([]byte, 0, len(data)*int(fromBits)/int(toBits)+1)
	maxv := uint32(1<<toBits - 1)
	maxAcc := uint32(1<<(fromBits+toBits-1) - 1)

	for _, b := range data {
		acc = ((acc << fromBits) | uint32(b)) & maxAcc
		bits += fromBits
		for bits >= toBits {
			bits -= toBits
			ret = append(ret, byte((acc>>bits)&maxv))
		}
	}

	if pad {
		if bits > 0 {
			ret = append(ret, byte((acc<<(toBits-bits))&maxv))
		}
	} else if bits >= fromBits || ((acc<<(toBits-bits))&maxv) != 0 {
		return nil, errors.New("conversion requires padding but pad is false")
	}

	return ret, nil
}

// EncodeCashAddress encodes a version byte and hash160 to CashAddress
func EncodeCashAddress(prefix string, version byte, hash160 []byte) string {
	payload := append([]byte{version}, hash160...)
	data, _ := ConvertBits(payload, 8, 5, true)

	combined := make([]byte, 0, len(prefix)*2+1+len(data)+8)
	combined = append(combined, ExpandPrefix(prefix)...)
	combined = append(combined, data...)
	combined = append(combined, 0, 0, 0, 0, 0, 0, 0, 0)

	checksum := PolyMod(combined)

	result := prefix + ":"
	for i := 0; i < len(data); i++ {
		result += string(charset[data[i]])
	}
	for i := 0; i < 8; i++ {
		result += string(charset[(checksum>>(5*(7-i)))&31])
	}

	return result
}

// DecodeCashAddress decodes a CashAddress to version byte and hash
func DecodeCashAddress(addr string) (prefix string, version byte, hash []byte, err error) {
	addr = strings.ToLower(addr)

	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		// Try to detect prefix
		if strings.HasPrefix(addr, MainnetPrefix) {
			prefix = MainnetPrefix
			addr = addr[len(MainnetPrefix):]
		} else if strings.HasPrefix(addr, TestnetPrefix) || strings.HasPrefix(addr, ChipnetPrefix) {
			prefix = TestnetPrefix
			addr = addr[len(TestnetPrefix):]
		} else if strings.HasPrefix(addr, RegtestPrefix) {
			prefix = RegtestPrefix
			addr = addr[len(RegtestPrefix):]
		} else {
			return "", 0, nil, errors.New("invalid cashaddress: no prefix")
		}
	} else {
		prefix = parts[0]
		addr = parts[1]
	}

	data := make([]byte, len(addr))
	for i, c := range addr {
		idx := strings.IndexByte(charset, byte(c))
		if idx < 0 {
			return "", 0, nil, fmt.Errorf("invalid character in address: %c", c)
		}
		data[i] = byte(idx)
	}

	if len(data) < 8 {
		return "", 0, nil, errors.New("address too short")
	}

	payload := data[:len(data)-8]

	// Verify checksum
	combined := make([]byte, 0, len(prefix)*2+1+len(data))
	combined = append(combined, ExpandPrefix(prefix)...)
	combined = append(combined, data...)

	if PolyMod(combined) != 0 {
		return "", 0, nil, errors.New("invalid checksum")
	}

	// Convert 5-bit groups to 8-bit bytes
	decoded, err := ConvertBits(payload, 5, 8, false)
	if err != nil {
		return "", 0, nil, err
	}

	if len(decoded) < 1 {
		return "", 0, nil, errors.New("decoded data too short")
	}

	version = decoded[0]
	hash = decoded[1:]

	return prefix, version, hash, nil
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

// ValidateCashAddress validates a CashAddress
func ValidateCashAddress(addr string) (bool, string) {
	_, version, hash, err := DecodeCashAddress(addr)
	if err != nil {
		return false, ""
	}

	expectedLen := 20
	if version >= 2 {
		expectedLen = 32
	}

	if len(hash) != expectedLen {
		return false, ""
	}

	return true, GetAddressType(version)
}

// AddressToScripthash converts a CashAddress to scripthash for Fulcrum
func AddressToScripthash(addr string) (string, error) {
	_, version, hash, err := DecodeCashAddress(addr)
	if err != nil {
		return "", err
	}

	var script []byte

	if version == 0 || version == 2 || version == 3 {
		// P2PKH: OP_DUP OP_HASH160 <20-byte hash> OP_EQUALVERIFY OP_CHECKSIG
		script = []byte{0x76, 0xa9, 0x14}
		script = append(script, hash...)
		script = append(script, 0x88, 0xac)
	} else if version == 1 || version == 4 || version == 5 {
		// P2SH: OP_HASH160 <20-byte hash> OP_EQUAL
		script = []byte{0xa9, 0x14}
		script = append(script, hash...)
		script = append(script, 0x87)
	} else {
		return "", fmt.Errorf("unsupported address version: %d", version)
	}

	// Double SHA256
	h1 := sha256.Sum256(script)
	h2 := sha256.Sum256(h1[:])

	// Reverse byte order
	scripthash := make([]byte, 32)
	for i := 0; i < 32; i++ {
		scripthash[i] = h2[31-i]
	}

	return hex.EncodeToString(scripthash), nil
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
