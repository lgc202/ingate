//go:build !unix

package aiproxy

import (
	"errors"
	"net"
)

func listenUnix(string) (*net.UnixListener, error) {
	return nil, errors.New("AI Proxy requires a Unix platform with socket permission enforcement")
}
