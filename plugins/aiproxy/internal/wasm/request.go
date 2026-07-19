package wasm

import (
	"fmt"
	"strings"

	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
)

func removeRequestHeaders(names ...string) error {
	headers, err := proxywasm.GetHttpRequestHeaders()
	if err != nil {
		return fmt.Errorf("read request headers: %w", err)
	}

	removed := make(map[string]bool, len(names))
	for _, name := range names {
		removed[strings.ToLower(name)] = true
	}
	filtered := make([][2]string, 0, len(headers))
	for _, header := range headers {
		if removed[strings.ToLower(header[0])] {
			continue
		}
		filtered = append(filtered, header)
	}
	if len(filtered) == len(headers) {
		return nil
	}
	if err := proxywasm.ReplaceHttpRequestHeaders(filtered); err != nil {
		return fmt.Errorf("replace request headers: %w", err)
	}
	return nil
}

func replaceRequestHeader(name, value string) error {
	if err := proxywasm.ReplaceHttpRequestHeader(name, value); err != nil {
		return fmt.Errorf("replace request header %q: %w", name, err)
	}
	return nil
}

func removeResponseHeaders(names ...string) error {
	headers, err := proxywasm.GetHttpResponseHeaders()
	if err != nil {
		return fmt.Errorf("read response headers: %w", err)
	}

	removed := make(map[string]bool, len(names))
	for _, name := range names {
		removed[strings.ToLower(name)] = true
	}
	filtered := make([][2]string, 0, len(headers))
	for _, header := range headers {
		if removed[strings.ToLower(header[0])] {
			continue
		}
		filtered = append(filtered, header)
	}
	if len(filtered) == len(headers) {
		return nil
	}
	if err := proxywasm.ReplaceHttpResponseHeaders(filtered); err != nil {
		return fmt.Errorf("replace response headers: %w", err)
	}
	return nil
}

func replaceResponseHeader(name, value string) error {
	if err := proxywasm.ReplaceHttpResponseHeader(name, value); err != nil {
		return fmt.Errorf("replace response header %q: %w", name, err)
	}
	return nil
}
