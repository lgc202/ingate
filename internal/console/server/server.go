// Package server 提供控制台静态资源服务和管理 API 反向代理。
package server

import "github.com/google/wire"

// ProviderSet 汇总 Console 的管理 API 代理和 HTTP 服务。
var ProviderSet = wire.NewSet(NewAdminAPIProxy, NewAssistantProxy, NewSessionAuth, NewHTTPServer)
