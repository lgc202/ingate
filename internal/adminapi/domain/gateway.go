package domain

import "time"

// Gateway 表示面向产品接口的网关。
type Gateway struct {
	Name        string
	DisplayName string
	Listeners   []Listener
	CreateTime  time.Time
	Version     string
}

// Listener 表示 Gateway 对外暴露的监听入口。
type Listener struct {
	Name     string
	Protocol string
	Port     int32
	Hostname string
}
