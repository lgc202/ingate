package server

import "github.com/google/wire"

// ProviderSet 汇总 Console 的管理 API 代理和 HTTP 服务
var ProviderSet = wire.NewSet(NewAdminAPIProxy, NewSessionAuth, NewHTTPServer)
