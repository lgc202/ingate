//go:build unix

package aiproxy

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
)

// listenUnix 要求部署先提供运行用户独占的目录，避免 Listen 到 Chmod 之间暴露入口。
// 不修改已有目录权限，不删除残留文件或 socket；占用时由原生 Bind 返回错误。
// 部署侧还须保护祖先目录，避免其他用户替换目录；本检查不是文件系统隔离机制。
func listenUnix(socket string) (*net.UnixListener, error) {
	directory := filepath.Dir(socket)
	info, err := os.Stat(directory)
	if err != nil {
		return nil, fmt.Errorf("inspect AI ingress directory %q: %w", directory, err)
	}
	stat := info.Sys().(*syscall.Stat_t)
	if !info.IsDir() || info.Mode().Perm()&0o077 != 0 || int(stat.Uid) != os.Geteuid() {
		return nil, fmt.Errorf("AI ingress directory %q must be private and owned by the runtime user", directory)
	}

	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		return nil, fmt.Errorf("listen on AI ingress socket: %w", err)
	}
	if err := os.Chmod(socket, 0o600); err != nil {
		return nil, errors.Join(fmt.Errorf("protect AI ingress socket: %w", err), listener.Close())
	}

	return listener, nil
}
