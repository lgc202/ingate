package domain

import "time"

// Upstream 表示面向产品接口的一组后端地址。
type Upstream struct {
	Name       string
	Endpoints  []Endpoint
	CreateTime time.Time
	Version    string
}

// Endpoint 表示一个后端地址及其权重。
type Endpoint struct {
	Address string
	Port    int32
	Weight  int32
}
