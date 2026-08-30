package server

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/lgc202/ingate/internal/console/conf"
)

// NewHTTPServer 创建控制台静态资源和管理 API 代理服务。
func NewHTTPServer(
	config *conf.Server,
	adminAPIProxy *AdminAPIProxy,
	assistantProxy *AssistantProxy,
	auth *SessionAuth,
	logger *slog.Logger,
) (*kratoshttp.Server, error) {
	if err := validateConsoleDirectory(config.GetConsoleDir()); err != nil {
		return nil, err
	}
	httpConfig := config.GetHttp()
	server := kratoshttp.NewServer(
		kratoshttp.Network("tcp"),
		kratoshttp.Address(httpConfig.GetAddr()),
		// Assistant 使用 SSE 长连接；普通管理请求的超时由后端服务控制。
		kratoshttp.Filter(
			recovery(logger),
			requestID(),
		),
	)
	server.HandlePrefix("/", NewRouter(adminAPIProxy, assistantProxy, auth, config.GetConsoleDir()))
	return server, nil
}

func validateConsoleDirectory(directory string) error {
	indexPath := filepath.Join(directory, "index.html")
	info, err := os.Stat(indexPath)
	if err != nil {
		return fmt.Errorf("read Console index file %q: %w", indexPath, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("console index path %q is not a regular file", indexPath)
	}
	return nil
}
