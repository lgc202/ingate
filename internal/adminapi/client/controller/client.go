// Package controller 访问 ingate-controller 内部状态接口
package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	statusPath             = "/internal/v1/status"
	maxStatusResponseBytes = 1 << 20
	jsonMediaType          = "application/json"
)

// DeliveryState 表示 Controller 当前的 Envoy 配置交付阶段
type DeliveryState string

const (
	stateNoConfig        DeliveryState = "NoConfig"
	stateWaitingForEnvoy DeliveryState = "WaitingForEnvoy"
	stateWaitingForACK   DeliveryState = "WaitingForACK"
	stateActive          DeliveryState = "Active"
	stateDegraded        DeliveryState = "Degraded"
)

// ACKSummary 表示当前配置版本的 Envoy ACK 进度
type ACKSummary struct {
	Required int `json:"required"`
	Received int `json:"received"`
}

// NACK 表示最近一次 Envoy 配置拒绝
type NACK struct {
	NodeID  string    `json:"nodeID"`
	TypeURL string    `json:"typeURL"`
	Version string    `json:"version"`
	Time    time.Time `json:"time"`
	Message string    `json:"message"`
}

// Status 是 Admin API 使用的 Controller 内部状态子集
type Status struct {
	CandidateVersion string
	ActiveVersion    string
	ConfigReady      bool
	DeliveryState    DeliveryState
	ConnectedEnvoys  int
	ACKs             ACKSummary
	LastNACK         *NACK
	Reconciled       bool
}

type statusResponse struct {
	CandidateVersion string         `json:"candidateVersion"`
	ActiveVersion    string         `json:"activeVersion"`
	ConfigReady      *bool          `json:"configReady"`
	DeliveryState    *DeliveryState `json:"deliveryState"`
	ConnectedEnvoys  *int           `json:"connectedEnvoys"`
	ACKs             *ACKSummary    `json:"acks"`
	LastNACK         *NACK          `json:"lastNack"`
	Reconciled       *bool          `json:"reconciled"`
}

// Client 读取 Controller 实时运行状态
type Client struct {
	statusURL  string
	httpClient *http.Client
}

// New 创建 Controller 状态客户端
func New(baseURL string, timeout time.Duration) (*Client, error) {
	if timeout <= 0 {
		return nil, errors.New("controller status timeout must be positive")
	}

	parsedURL, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, fmt.Errorf("parse controller status URL: %w", err)
	}
	if (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return nil, errors.New("controller status URL must use http or https and include a host")
	}
	if parsedURL.RawQuery != "" || parsedURL.Fragment != "" {
		return nil, errors.New("controller status URL must not include a query or fragment")
	}
	parsedURL.Path = strings.TrimRight(parsedURL.Path, "/") + statusPath

	return &Client{
		statusURL:  parsedURL.String(),
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

// GetStatus 获取 Controller 当前编译和 Envoy 配置交付状态
func (c *Client) GetStatus(ctx context.Context) (*Status, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.statusURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create controller status request: %w", err)
	}
	request.Header.Set("Accept", jsonMediaType)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request controller status: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("controller status returned HTTP %d", response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxStatusResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read controller status response: %w", err)
	}
	if len(body) > maxStatusResponseBytes {
		return nil, errors.New("controller status response is too large")
	}

	var payload statusResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode controller status response: %w", err)
	}
	if err := payload.validate(); err != nil {
		return nil, err
	}

	return &Status{
		CandidateVersion: payload.CandidateVersion,
		ActiveVersion:    payload.ActiveVersion,
		ConfigReady:      *payload.ConfigReady,
		DeliveryState:    *payload.DeliveryState,
		ConnectedEnvoys:  *payload.ConnectedEnvoys,
		ACKs:             *payload.ACKs,
		LastNACK:         payload.LastNACK,
		Reconciled:       *payload.Reconciled,
	}, nil
}

func (r *statusResponse) validate() error {
	if r.ConfigReady == nil || r.DeliveryState == nil || r.ConnectedEnvoys == nil || r.ACKs == nil || r.Reconciled == nil {
		return errors.New("controller status response is missing required fields")
	}
	switch *r.DeliveryState {
	case stateNoConfig, stateWaitingForEnvoy, stateWaitingForACK, stateActive, stateDegraded:
	default:
		return fmt.Errorf("controller status response contains unknown delivery state %q", *r.DeliveryState)
	}
	if *r.ConnectedEnvoys < 0 {
		return errors.New("controller status response contains a negative Envoy count")
	}
	if r.ACKs.Required < 0 || r.ACKs.Received < 0 || r.ACKs.Received > r.ACKs.Required {
		return errors.New("controller status response contains invalid ACK progress")
	}
	return nil
}
