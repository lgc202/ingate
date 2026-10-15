// Package model 编排模型请求的配置捕获、路由选择和出站请求准备。
// 本包不承担 HTTP 读取、公开错误编码或网络转发。
package model

import (
	"context"
	"errors"
	"fmt"

	"github.com/lgc202/ingate/internal/aiproxy/pkg/aiconfig"
	"github.com/lgc202/ingate/internal/aiproxy/pkg/endpointspec"
	"github.com/lgc202/ingate/internal/aiproxy/pkg/translator"
)

var (
	// ErrInvalidArgument 表示请求体不符合当前模型入口的解析要求。
	ErrInvalidArgument = errors.New("invalid model request argument")
	// ErrModelNotFound 表示指定入口没有匹配请求模型的路由。
	ErrModelNotFound = errors.New("model not found")
	// ErrUnavailable 表示尚无可用配置，或首个匹配路由的后端不可用。
	ErrUnavailable = errors.New("model service unavailable")
)

// ChatCompletionParams 保存可信入口和已经读取完毕的原始请求体。
// Body 在调用期间由调用方保持不变，读取大小限制由入站适配层负责。
type ChatCompletionParams struct {
	ListenerPort int32
	// HostName 是 Listener 下的完整编译标识，不是客户端的 Host Header。
	HostName string
	Body     []byte
}

// ChatCompletionRequest 是选定后端及已完成模型名覆盖的出站请求体。
// Backend 不携带配置内部的可变引用，Body 归调用方所有且独立于原始请求体。
// 它不是上游响应，也不表示 Envoy 出口已经准备就绪。
type ChatCompletionRequest struct {
	Backend aiconfig.ModelBackend
	Body    []byte
}

// Service 使用组件内的只读配置能力准备模型请求，不持有底层 API 客户端。
type Service struct {
	loader *aiconfig.Loader
}

// New 创建模型 Service；loader 必须非 nil，其运行和停止由组件组合根负责。
func New(loader *aiconfig.Loader) *Service {
	return &Service{loader: loader}
}

// PrepareChatCompletion 用原始 model 选择目标，再应用所选绑定的模型名覆盖。
// 调用方须先确定可信入口；本方法不根据请求内容或客户端 Header 推断入口。
// 一次调用只捕获一份 Go 配置，不比较 Envoy revision，也不重新读取领域资源。
// 失败返回 nil；已选后端不可用时拒绝，不尝试其他模型规则。
// 取消在处理前和交付结果前检查，不能中断同步 JSON 解码；调用方仍须限制请求体大小。
func (s *Service) PrepareChatCompletion(
	ctx context.Context,
	params ChatCompletionParams,
) (*ChatCompletionRequest, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	config, loaded := s.loader.Capture()
	if !loaded {
		return nil, ErrUnavailable
	}

	request, err := endpointspec.ParseChatCompletions(params.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
	}

	backend, err := config.ResolveModel(params.ListenerPort, params.HostName, request.Model)
	if err != nil {
		switch {
		case errors.Is(err, aiconfig.ErrModelRouteNotFound):
			return nil, fmt.Errorf("%w: %w", ErrModelNotFound, err)
		case errors.Is(err, aiconfig.ErrModelBackendUnavailable):
			return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
		default:
			return nil, err
		}
	}

	body, err := translator.TranslateChatCompletionOpenAIToOpenAI(params.Body, backend.ModelNameOverride)
	if err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return &ChatCompletionRequest{Backend: backend, Body: body}, nil
}
