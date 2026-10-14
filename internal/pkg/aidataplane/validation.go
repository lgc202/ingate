package aidataplane

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strings"
	"unicode/utf8"

	"k8s.io/apimachinery/pkg/util/validation"
)

// validate 检查编码边界的结构与引用，不代替 Controller 的领域裁决及排序。
func validate(c Config) error {
	if c.FormatVersion != FormatVersionV1 {
		return fmt.Errorf("unsupported formatVersion %q", c.FormatVersion)
	}
	if c.Listeners == nil || c.Backends == nil || c.NetworkBackends == nil {
		return errors.New("listeners, backends and networkBackends must be arrays")
	}

	networkNames := make(map[string]bool, len(c.NetworkBackends))
	for i, backend := range c.NetworkBackends {
		if backend.Name == "" || networkNames[backend.Name] {
			return fmt.Errorf("networkBackends[%d].name must be nonempty and unique", i)
		}

		if err := validateEndpoints(backend.Endpoints); err != nil {
			return fmt.Errorf("networkBackends[%d]: %w", i, err)
		}

		networkNames[backend.Name] = true
	}

	backendNames := make(map[string]bool, len(c.Backends))
	for i, backend := range c.Backends {
		if backend.Name == "" || backendNames[backend.Name] {
			return fmt.Errorf("backends[%d].name must be nonempty and unique", i)
		}
		if backend.Schema.Name != APISchemaOpenAI {
			return fmt.Errorf("backends[%d].schema.name must be OpenAI", i)
		}
		if err := validatePath(backend.Schema.Prefix); err != nil {
			return fmt.Errorf("backends[%d].schema.prefix: %w", i, err)
		}
		if backend.ModelNameOverride == nil {
			return fmt.Errorf("backends[%d].modelNameOverride must be a string", i)
		}

		if !networkNames[backend.NetworkBackendName] {
			return fmt.Errorf("backends[%d].networkBackendName must reference an existing network backend", i)
		}

		backendNames[backend.Name] = true
	}

	ports := make(map[int32]bool, len(c.Listeners))
	for i, listener := range c.Listeners {
		if listener.Port < 1 || listener.Port > 65535 || ports[listener.Port] {
			return fmt.Errorf("listeners[%d].port must be between 1 and 65535 and unique", i)
		}

		if err := validateHosts(listener.Hosts, backendNames, networkNames); err != nil {
			return fmt.Errorf("listeners[%d]: %w", i, err)
		}

		ports[listener.Port] = true
	}

	return nil
}

func validateEndpoints(endpoints []Endpoint) error {
	if len(endpoints) == 0 {
		return errors.New("endpoints must be a nonempty array")
	}

	type endpointKey struct {
		address string
		port    int32
	}
	seen := make(map[endpointKey]bool, len(endpoints))
	for i, endpoint := range endpoints {
		if _, err := netip.ParseAddr(endpoint.Address); err != nil {
			if messages := validation.IsDNS1123Subdomain(endpoint.Address); len(messages) != 0 {
				return fmt.Errorf("endpoints[%d].address: %s", i, strings.Join(messages, "; "))
			}
		}
		if endpoint.Port < 1 || endpoint.Port > 65535 || endpoint.Weight <= 0 {
			return fmt.Errorf("endpoints[%d] requires port between 1 and 65535 and positive weight", i)
		}

		key := endpointKey{address: endpoint.Address, port: endpoint.Port}
		if seen[key] {
			return fmt.Errorf("endpoints[%d] duplicates an address and port", i)
		}

		seen[key] = true
	}

	return nil
}

func validateHosts(hosts []Host, backendNames, networkNames map[string]bool) error {
	if hosts == nil {
		return errors.New("hosts must be an array")
	}

	names := make(map[string]bool, len(hosts))
	for i, host := range hosts {
		if host.Name == "" || names[host.Name] {
			return fmt.Errorf("hosts[%d].name must be nonempty and unique", i)
		}
		if host.Hostname == nil {
			return fmt.Errorf("hosts[%d].hostname must be a string", i)
		}
		if *host.Hostname != "" {
			if messages := validation.IsDNS1123Subdomain(*host.Hostname); len(messages) != 0 {
				return fmt.Errorf("hosts[%d].hostname: %s", i, strings.Join(messages, "; "))
			}
		}

		if err := validateModelRoutes(host.ModelRoutes, backendNames); err != nil {
			return fmt.Errorf("hosts[%d]: %w", i, err)
		}
		if err := validateMCPRoutes(host.MCPRoutes, networkNames); err != nil {
			return fmt.Errorf("hosts[%d]: %w", i, err)
		}

		names[host.Name] = true
	}

	return nil
}

func validateModelRoutes(routes []ModelRoute, backendNames map[string]bool) error {
	if routes == nil {
		return errors.New("modelRoutes must be an array")
	}

	names := make(map[string]bool, len(routes))
	for i, route := range routes {
		if route.Name == "" || names[route.Name] {
			return fmt.Errorf("modelRoutes[%d].name must be nonempty and unique", i)
		}
		if route.Headers == nil || len(route.Headers) > 1 {
			return fmt.Errorf("modelRoutes[%d].headers must be an array with at most one match", i)
		}
		if len(route.Headers) == 1 {
			header := route.Headers[0]
			if header.Name != "x-ai-eg-model" || header.Type != HeaderMatchExact || header.Value == "" {
				return fmt.Errorf("modelRoutes[%d].headers requires an Exact x-ai-eg-model match with a nonempty value", i)
			}
		}

		if route.BackendName == nil {
			return fmt.Errorf("modelRoutes[%d].backendName must be a string", i)
		}
		if *route.BackendName != "" && !backendNames[*route.BackendName] {
			return fmt.Errorf("modelRoutes[%d].backendName references a missing backend", i)
		}

		names[route.Name] = true
	}

	return nil
}

func validateMCPRoutes(routes []MCPRoute, networkNames map[string]bool) error {
	if routes == nil {
		return errors.New("mcpRoutes must be an array")
	}

	names := make(map[string]bool, len(routes))
	for i, route := range routes {
		if route.Name == "" || names[route.Name] {
			return fmt.Errorf("mcpRoutes[%d].name must be nonempty and unique", i)
		}
		if !strings.HasPrefix(route.Path, "/") || utf8.RuneCountInString(route.Path) > 1024 {
			return fmt.Errorf("mcpRoutes[%d].path must be an absolute path of at most 1024 characters", i)
		}
		if err := validatePath(route.Path); err != nil {
			return fmt.Errorf("mcpRoutes[%d].path: %w", i, err)
		}

		if !strings.HasPrefix(route.BackendPath, "/") || utf8.RuneCountInString(route.BackendPath) > 1024 {
			return fmt.Errorf("mcpRoutes[%d].backendPath must be an absolute path of at most 1024 characters", i)
		}
		if err := validatePath(route.BackendPath); err != nil {
			return fmt.Errorf("mcpRoutes[%d].backendPath: %w", i, err)
		}

		if route.BackendName == nil {
			return fmt.Errorf("mcpRoutes[%d].backendName must be a string", i)
		}
		if *route.BackendName != "" && !networkNames[*route.BackendName] {
			return fmt.Errorf("mcpRoutes[%d].backendName references a missing network backend", i)
		}

		names[route.Name] = true
	}

	return nil
}

func validatePath(value string) error {
	if value == "" {
		return errors.New("path must be nonempty after defaulting")
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("parse path: %w", err)
	}

	if parsed.IsAbs() || parsed.Host != "" || parsed.User != nil || strings.ContainsAny(value, "?#") {
		return errors.New("path must not contain an authority, query or fragment")
	}
	return nil
}
