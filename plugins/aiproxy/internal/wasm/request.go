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

func addRequestHeader(name, value string) error {
	if err := proxywasm.AddHttpRequestHeader(name, value); err != nil {
		return fmt.Errorf("add request header %q: %w", name, err)
	}
	return nil
}
