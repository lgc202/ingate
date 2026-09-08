//go:build darwin || linux

package diskqueue

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"path/filepath"

	"golang.org/x/sys/unix"
)

const diskBlockBytes int64 = 512

func inspectStorage(path string) (storageUsage, error) {
	var filesystem unix.Statfs_t
	if err := unix.Statfs(path, &filesystem); err != nil {
		return storageUsage{}, fmt.Errorf("measure disk queue free space: %w", err)
	}
	blockBytes := int64(filesystem.Bsize)
	if blockBytes <= 0 {
		return storageUsage{}, errors.New("disk queue filesystem block size is invalid")
	}
	freeBytes := saturatedProduct(uint64(filesystem.Bavail), uint64(filesystem.Bsize))

	var diskBytes, largestFileBytes int64
	err := filepath.WalkDir(path, func(entryPath string, _ fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		var stat unix.Stat_t
		if err := unix.Lstat(entryPath, &stat); err != nil {
			return err
		}
		if stat.Blocks < 0 || stat.Blocks > (math.MaxInt64-diskBytes)/diskBlockBytes {
			return errors.New("disk queue physical usage exceeds the supported range")
		}
		fileBytes := stat.Blocks * diskBlockBytes
		diskBytes += fileBytes
		if stat.Mode&unix.S_IFMT == unix.S_IFREG {
			logicalBytes, ok := allocationSize(stat.Size, blockBytes)
			if !ok {
				return errors.New("disk queue file size exceeds the supported range")
			}
			largestFileBytes = max(largestFileBytes, fileBytes, logicalBytes)
		}
		return nil
	})
	if err != nil {
		return storageUsage{}, fmt.Errorf("measure disk queue usage: %w", err)
	}

	return storageUsage{
		diskBytes:        diskBytes,
		largestFileBytes: largestFileBytes,
		freeBytes:        freeBytes,
		blockBytes:       blockBytes,
	}, nil
}

func saturatedProduct(left, right uint64) int64 {
	if right != 0 && left > math.MaxInt64/right {
		return math.MaxInt64
	}
	return int64(left * right)
}
