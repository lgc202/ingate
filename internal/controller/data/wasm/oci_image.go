package wasm

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	containerv1 "github.com/google/go-containerregistry/pkg/v1"
)

// extractWasmImageLayer 读取 Wasm Image Specification 的标准 tar layer，其中模块固定保存为 plugin.wasm
func (s *Store) extractWasmImageLayer(layer containerv1.Layer) ([]byte, error) {
	reader, err := layer.Compressed()
	if err != nil {
		return nil, fmt.Errorf("open OCI Wasm image layer: %w", err)
	}
	defer func() { _ = reader.Close() }()

	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("decompress OCI Wasm image layer: %w", err)
	}
	defer func() { _ = gzipReader.Close() }()

	tarReader := tar.NewReader(io.LimitReader(gzipReader, s.maxModuleSize))
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%s not found in OCI Wasm image", wasmFileName)
		}
		if err != nil {
			return nil, fmt.Errorf("read OCI Wasm image layer: %w", err)
		}
		if filepath.Base(header.Name) != wasmFileName {
			continue
		}
		if header.Size > s.maxModuleSize {
			return nil, fmt.Errorf("wasm module exceeds maximum size %d bytes", s.maxModuleSize)
		}
		binary := make([]byte, header.Size)
		if _, err := io.ReadFull(tarReader, binary); err != nil {
			return nil, fmt.Errorf("read %s from OCI Wasm image: %w", wasmFileName, err)
		}
		return binary, nil
	}
}
