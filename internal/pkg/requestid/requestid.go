package requestid

import (
	"crypto/rand"
	"encoding/hex"
)

const (
	// Header 表示请求链路 ID header
	Header = "X-Request-ID"
)

// New 生成新的请求链路 ID
func New() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(bytes[:])
}
