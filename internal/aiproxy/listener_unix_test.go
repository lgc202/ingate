//go:build unix

package aiproxy

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestListenUnixProtectsAndUnlinksOwnedSocket(t *testing.T) {
	socket := filepath.Join(unixSocketDirectory(t), "ingress.sock")

	listener, err := listenUnix(socket)
	if err != nil {
		t.Fatalf("listenUnix(private directory) error = %v", err)
	}
	t.Cleanup(func() {
		// 显式关闭路径之后的重复清理只允许 net.ErrClosed。
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("Close(test listener cleanup) error = %v", err)
		}
	})
	info, err := os.Stat(socket)
	if err != nil {
		t.Fatalf("Stat(bound socket) error = %v", err)
	}

	if info.Mode()&os.ModeSocket == 0 || info.Mode().Perm() != 0o600 {
		t.Errorf("listenUnix() socket mode = %v, want socket with 0600 permissions", info.Mode())
	}

	if err := listener.Close(); err != nil {
		t.Fatalf("Close(owned socket) error = %v", err)
	}

	if _, err := os.Stat(socket); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Close(owned socket) path error = %v, want unlinked socket", err)
	}
}

func TestListenUnixDoesNotAlterOccupiedPath(t *testing.T) {
	socket := filepath.Join(unixSocketDirectory(t), "ingress.sock")
	if err := os.WriteFile(socket, []byte("keep"), 0o600); err != nil {
		t.Fatalf("WriteFile(occupied path) error = %v", err)
	}

	listener, err := listenUnix(socket)
	if err == nil || listener != nil {
		t.Fatalf("listenUnix(occupied path) = (%v, %v), want bind rejection", listener, err)
	}
	data, err := os.ReadFile(socket)
	if err != nil {
		t.Fatalf("ReadFile(occupied path) error = %v", err)
	}

	if string(data) != "keep" {
		t.Errorf("listenUnix(occupied path) data = %q, want original file", data)
	}
}

func TestListenUnixRejectsExposedDirectory(t *testing.T) {
	directory := unixSocketDirectory(t)
	if err := os.Chmod(directory, 0o755); err != nil {
		t.Fatalf("Chmod(exposed directory) error = %v", err)
	}
	socket := filepath.Join(directory, "ingress.sock")

	listener, err := listenUnix(socket)
	if err == nil || listener != nil {
		t.Fatalf("listenUnix(exposed directory) = (%v, %v), want rejection before binding", listener, err)
	}
	if _, err := os.Stat(socket); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("listenUnix(exposed directory) socket path error = %v, want no created socket", err)
	}
	info, err := os.Stat(directory)
	if err != nil {
		t.Fatalf("Stat(exposed directory) error = %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("listenUnix(exposed directory) mode = %v, want unchanged directory", info.Mode())
	}
}
