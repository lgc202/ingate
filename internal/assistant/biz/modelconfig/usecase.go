// Package modelconfig 管理运维助手的模型连接配置。
package modelconfig

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

// Store 持久化运维助手当前生效的唯一模型连接。
type Store interface {
	GetModelConnection(context.Context) (Connection, error)
	UpdateModelConnection(context.Context, Update) (Connection, error)
}

// Usecase 提供模型连接的查询、更新和执行时选取。
type Usecase struct {
	store Store
}

// NewUsecase 创建模型连接业务入口。
func NewUsecase(store Store) *Usecase {
	return &Usecase{store: store}
}

// Get 返回当前配置；未配置时返回可用于表单的默认值。
func (uc *Usecase) Get(ctx context.Context) (Connection, error) {
	connection, err := uc.configuredConnection(ctx)
	if errors.Is(err, ErrNotConfigured) {
		return DefaultConnection(), nil
	}
	return connection, err
}

// ActiveConnection 返回新 Agent 执行应使用的连接；未配置时拒绝调用模型。
func (uc *Usecase) ActiveConnection(ctx context.Context) (Connection, error) {
	return uc.configuredConnection(ctx)
}

// Update 验证并替换当前模型连接。
func (uc *Usecase) Update(ctx context.Context, update Update) (Connection, error) {
	connection := update.Connection.normalized()
	if err := connection.validate(); err != nil {
		return Connection{}, err
	}
	if update.APIKey != nil && len(*update.APIKey) > maxAPIKeyLength {
		return Connection{}, ErrInvalidConnection
	}
	update.Connection = connection
	return uc.store.UpdateModelConnection(ctx, update)
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

func (uc *Usecase) configuredConnection(ctx context.Context) (Connection, error) {
	connection, err := uc.store.GetModelConnection(ctx)
	if err != nil {
		return Connection{}, err
	}
	if !connection.Configured || connection.UpdatedAt.IsZero() {
		return Connection{}, errors.New("stored assistant model connection is invalid")
	}
	if err := connection.validate(); err != nil {
		return Connection{}, errors.New("stored assistant model connection is invalid")
	}
	return connection, nil
}

func (c Connection) normalized() Connection {
	c.Endpoint = strings.TrimRight(strings.TrimSpace(c.Endpoint), "/")
	c.Model = strings.TrimSpace(c.Model)
	return c
}

func (c Connection) validate() error {
	normalized := c.normalized()
	if normalized.Endpoint != c.Endpoint || normalized.Model != c.Model {
		return ErrInvalidConnection
	}
	if c.Mode != ModeDirect && c.Mode != ModeIngate {
		return ErrInvalidConnection
	}
	if c.Protocol != ProtocolOpenAICompatible && c.Protocol != ProtocolAnthropic {
		return ErrInvalidConnection
	}
	// Ingate 对客户端公开 OpenAI 兼容接口，具体厂商协议由数据面负责转换。
	if c.Mode == ModeIngate && c.Protocol != ProtocolOpenAICompatible {
		return ErrInvalidConnection
	}
	if len(c.Endpoint) > maxEndpointLength || len(c.Model) > maxModelLength {
		return ErrInvalidConnection
	}
	target, err := url.Parse(c.Endpoint)
	if err != nil || target.Host == "" || target.Scheme != "http" && target.Scheme != "https" {
		return ErrInvalidConnection
	}
	if target.User != nil || target.RawQuery != "" || target.Fragment != "" {
		return ErrInvalidConnection
	}
	if len(c.APIKey) > maxAPIKeyLength {
		return ErrInvalidConnection
	}
	if c.Model == "" || c.Timeout <= 0 || c.Timeout > maxTimeout ||
		c.MaxOutputTokens <= 0 || c.MaxOutputTokens > maxOutputTokens {
		return ErrInvalidConnection
	}
	reasoningBudget := c.ReasoningBudgetTokens
	if reasoningBudget < 0 ||
		(reasoningBudget > 0 && reasoningBudget < minReasoningBudget) ||
		reasoningBudget >= c.MaxOutputTokens ||
		(c.Protocol != ProtocolAnthropic && reasoningBudget != 0) {
		return ErrInvalidConnection
	}
	return nil
}
