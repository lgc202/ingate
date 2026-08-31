// Package requestrecord 定义请求记录跨组件使用的稳定身份契约。
package requestrecord

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"time"
)

const idLength = sha256.Size * 2

// NewID 根据 Envoy 请求身份和开始时间生成可重复的请求记录 ID。
func NewID(nodeID, streamID, requestID string, startedAt time.Time) string {
	identity := make([]byte, 0, len(nodeID)+len(streamID)+len(requestID)+32)
	identity = appendString(identity, nodeID)
	identity = appendString(identity, streamID)
	identity = appendString(identity, requestID)
	identity = binary.AppendVarint(identity, startedAt.Unix())
	identity = binary.AppendUvarint(identity, uint64(startedAt.Nanosecond()))
	digest := sha256.Sum256(identity)
	return hex.EncodeToString(digest[:])
}

// IsValidID 判断 value 是否为规范的小写请求记录 ID。
func IsValidID(value string) bool {
	if len(value) != idLength {
		return false
	}
	for i := range value {
		character := value[i]
		decimalDigit := character >= '0' && character <= '9'
		lowercaseHexDigit := character >= 'a' && character <= 'f'
		if !decimalDigit && !lowercaseHexDigit {
			return false
		}
	}
	return true
}

func appendString(destination []byte, value string) []byte {
	destination = binary.AppendUvarint(destination, uint64(len(value)))
	return append(destination, value...)
}
