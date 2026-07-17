package lastgood

import (
	"errors"
	"time"
)

const (
	// Key 是 Last Good 在内部 etcd 前缀中的固定键
	Key = "/ingate/internal/last-good/envoy"
	// SchemaVersion 是当前 Last Good 记录格式版本
	SchemaVersion = 1
)

var (
	// ErrNotFound 表示尚未保存过 Last Good
	ErrNotFound = errors.New("last good not found")
	// ErrCorrupt 表示记录内容损坏或不是一致的 Envoy 配置
	ErrCorrupt = errors.New("last good corrupt")
	// ErrIncompatible 表示记录格式不受当前版本支持
	ErrIncompatible = errors.New("last good incompatible")
)

// Record 是 Last Good 的持久化协议
type Record struct {
	SchemaVersion int       `json:"schemaVersion"`
	Version       string    `json:"version"`
	ContentHash   string    `json:"contentHash"`
	GeneratedAt   time.Time `json:"generatedAt"`
	Listeners     [][]byte  `json:"listeners"`
	Routes        [][]byte  `json:"routes"`
	Clusters      [][]byte  `json:"clusters"`
	Endpoints     [][]byte  `json:"endpoints"`
}
