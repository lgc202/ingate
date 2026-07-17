package lastgood

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"k8s.io/client-go/rest"
)

// Client 通过 apiserver 内部接口读写 Last Good
type Client struct {
	httpClient *http.Client
	endpoint   string
}

// NewClient 使用 apiserver rest.Config 创建 Last Good client
func NewClient(config *rest.Config) (*Client, error) {
	httpClient, err := rest.HTTPClientFor(config)
	if err != nil {
		return nil, fmt.Errorf("create last good HTTP client: %w", err)
	}

	baseURL, err := url.Parse(config.Host)
	if err != nil {
		return nil, fmt.Errorf("parse apiserver host: %w", err)
	}
	if baseURL.Scheme == "" || baseURL.Host == "" {
		return nil, fmt.Errorf("apiserver host must be an absolute URL")
	}
	baseURL.Path = strings.TrimRight(baseURL.Path, "/") + APIPath
	baseURL.RawPath = ""
	baseURL.RawQuery = ""
	baseURL.Fragment = ""

	return &Client{
		httpClient: httpClient,
		endpoint:   baseURL.String(),
	}, nil
}

// Load 从 apiserver 读取并校验 Last Good
func (c *Client) Load(ctx context.Context) (Record, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return Record{}, fmt.Errorf("create last good request: %w", err)
	}
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return Record{}, fmt.Errorf("load last good from apiserver: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		_, _ = io.Copy(io.Discard, response.Body)
		return Record{}, ErrNotFound
	}
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, response.Body)
		return Record{}, fmt.Errorf("load last good from apiserver: unexpected status %s", response.Status)
	}

	record, err := Decode(response.Body)
	if err != nil {
		return Record{}, err
	}
	return record, nil
}

// Save 完整校验后通过 apiserver 原子覆盖 Last Good
func (c *Client) Save(ctx context.Context, record Record) error {
	data, err := Marshal(record)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPut, c.endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create last good request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("save last good through apiserver: %w", err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)

	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("save last good through apiserver: unexpected status %s", response.Status)
	}
	return nil
}
