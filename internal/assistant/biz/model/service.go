// Package model 管理运维助手的当前模型连接。
package model

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

// Service 提供模型连接的查询、更新和执行时选取。
type Service struct {
	store Store
}

// NewService 创建模型连接业务服务。
func NewService(store Store) *Service {
	return &Service{store: store}
}

// Get 返回当前配置；未配置时返回可用于表单的默认值。
func (s *Service) Get(ctx context.Context) (Connection, error) {
	connection, err := s.store.GetModelConnection(ctx)
	if errors.Is(err, ErrNotConfigured) {
		return DefaultConnection(), nil
	}
	return connection, err
}

// ActiveConnection 返回新 Agent 执行应使用的连接；未配置时拒绝调用模型。
func (s *Service) ActiveConnection(ctx context.Context) (Connection, error) {
	connection, err := s.store.GetModelConnection(ctx)
	if err != nil {
		return Connection{}, err
	}
	if err := connection.validate(); err != nil {
		return Connection{}, err
	}
	return connection, nil
}

// Update 验证并替换当前模型连接。
func (s *Service) Update(ctx context.Context, update Update) (Connection, error) {
	connection := update.Connection.normalized()
	if err := connection.validate(); err != nil {
		return Connection{}, err
	}
	if update.APIKey != nil && len(*update.APIKey) > maxAPIKeyLength {
		return Connection{}, ErrInvalidConnection
	}
	update.Connection = connection
	return s.store.UpdateModelConnection(ctx, update)
}

// DefaultConnection 集中定义首次配置表单的安全默认值。
func DefaultConnection() Connection {
	return Connection{
		Mode:            ModeIngate,
		Protocol:        ProtocolOpenAICompatible,
		Timeout:         DefaultTimeout,
		MaxOutputTokens: DefaultMaxOutputTokens,
	}
}

func (connection Connection) normalized() Connection {
	connection.Endpoint = strings.TrimRight(strings.TrimSpace(connection.Endpoint), "/")
	connection.Model = strings.TrimSpace(connection.Model)
	return connection
}

func (connection Connection) validate() error {
	if connection.Mode != ModeDirect && connection.Mode != ModeIngate {
		return ErrInvalidConnection
	}
	if connection.Protocol != ProtocolOpenAICompatible && connection.Protocol != ProtocolAnthropic {
		return ErrInvalidConnection
	}
	// Ingate 对客户端公开 OpenAI 兼容接口，具体厂商协议由数据面负责转换。
	if connection.Mode == ModeIngate && connection.Protocol != ProtocolOpenAICompatible {
		return ErrInvalidConnection
	}
	if len(connection.Endpoint) > maxEndpointLength || len(connection.Model) > maxModelLength {
		return ErrInvalidConnection
	}
	target, err := url.Parse(connection.Endpoint)
	if err != nil || target.Host == "" || target.Scheme != "http" && target.Scheme != "https" {
		return ErrInvalidConnection
	}
	if connection.Model == "" || connection.Timeout <= 0 || connection.Timeout > maxTimeout ||
		connection.MaxOutputTokens <= 0 || connection.MaxOutputTokens > maxOutputTokens {
		return ErrInvalidConnection
	}
	reasoningBudget := connection.ReasoningBudgetTokens
	if reasoningBudget < 0 ||
		(reasoningBudget > 0 && reasoningBudget < minReasoningBudget) ||
		reasoningBudget >= connection.MaxOutputTokens ||
		(connection.Protocol != ProtocolAnthropic && reasoningBudget != 0) {
		return ErrInvalidConnection
	}
	return nil
}
