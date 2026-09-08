package diskqueue

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	directoryMode os.FileMode = 0o700
	fileMode      os.FileMode = 0o600
	lockFilename              = ".lock"
)

// directoryLock 持有 WAL 目录的进程级排他锁，关闭文件才会释放所有权。
type directoryLock struct {
	file *os.File
}

func (l *directoryLock) Close() error {
	return errors.Join(unlock(l.file), l.file.Close())
}

func prepareDirectory(path string) (string, error) {
	if err := os.MkdirAll(path, directoryMode); err != nil {
		return "", fmt.Errorf("create disk queue directory: %w", err)
	}
	if err := os.Chmod(path, directoryMode); err != nil {
		return "", fmt.Errorf("restrict disk queue directory permissions: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve disk queue directory: %w", err)
	}
	return resolved, nil
}

func lockDirectory(path string) (*directoryLock, error) {
	file, err := os.OpenFile(
		filepath.Join(path, lockFilename),
		os.O_CREATE|os.O_RDWR,
		fileMode,
	)
	if err != nil {
		return nil, fmt.Errorf("open disk queue lock: %w", err)
	}
	if err := file.Chmod(fileMode); err != nil {
		return nil, errors.Join(
			fmt.Errorf("restrict disk queue lock permissions: %w", err),
			file.Close(),
		)
	}
	if err := tryLock(file); err != nil {
		return nil, errors.Join(
			fmt.Errorf("lock disk queue directory: %w", err),
			file.Close(),
		)
	}

	return &directoryLock{file: file}, nil
}

func restrictExistingFiles(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("read disk queue directory: %w", err)
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect disk queue file %q: %w", entry.Name(), err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if err := os.Chmod(filepath.Join(path, entry.Name()), fileMode); err != nil {
			return fmt.Errorf("restrict disk queue file %q permissions: %w", entry.Name(), err)
		}
	}
	return nil
}
