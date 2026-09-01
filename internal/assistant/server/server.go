// Package server 装配运维助手的 Kratos HTTP transport。
package server

import "github.com/google/wire"

// ProviderSet 提供运维助手的 HTTP 服务。
var ProviderSet = wire.NewSet(NewHTTPServer)
