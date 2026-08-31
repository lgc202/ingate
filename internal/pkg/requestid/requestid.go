// Package requestid 生成并传递管理请求的链路标识。
package requestid

import (
	"crypto/rand"
	"net/http"
)

const (
	// Header 表示请求链路 ID header。
	Header   = "X-Request-ID"
	maxBytes = 128
)

// New 生成新的请求链路 ID。
func New() string {
	return rand.Text()
}

// GetOrCreate 返回 Header 中已有的有效请求 ID，否则生成并写入一个新 ID。
func GetOrCreate(header http.Header) string {
	id := header.Get(Header)
	if !valid(id) {
		id = New()
		header.Set(Header, id)
	}
	return id
}

func valid(id string) bool {
	if id == "" || len(id) > maxBytes {
		return false
	}
	for index := range len(id) {
		if id[index] < '!' || id[index] > '~' {
			return false
		}
	}
	return true
}
