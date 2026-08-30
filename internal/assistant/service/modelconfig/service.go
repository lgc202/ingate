// Package modelconfig 适配运维助手模型连接配置的 HTTP API 和业务对象。
package modelconfig

import (
	"context"
	"errors"
	"time"

	kerrors "github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	assistantv1 "github.com/lgc202/ingate/api/assistant/v1"
	modelconfigbiz "github.com/lgc202/ingate/internal/assistant/biz/modelconfig"
)

// Service 实现模型连接的产品协议。
type Service struct {
	connections *modelconfigbiz.Service
}

// NewService 创建模型连接协议服务。
func NewService(connections *modelconfigbiz.Service) *Service {
	return &Service{connections: connections}
}

// GetModelConnection 返回当前 Assistant 模型连接，且不回传已保存的凭据。
func (s *Service) GetModelConnection(
	ctx context.Context,
	_ *emptypb.Empty,
) (*assistantv1.ModelConnection, error) {
	connection, err := s.connections.Get(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	return modelConnectionResponse(connection), nil
}

// UpdateModelConnection 校验并保存 Assistant 使用的模型连接。
func (s *Service) UpdateModelConnection(
	ctx context.Context,
	request *assistantv1.UpdateModelConnectionRequest,
) (*assistantv1.ModelConnection, error) {
	update, err := modelUpdate(request)
	if err != nil {
		return nil, err
	}
	connection, err := s.connections.Update(ctx, update)
	if err != nil {
		return nil, mapError(err)
	}
	return modelConnectionResponse(connection), nil
}

func modelUpdate(request *assistantv1.UpdateModelConnectionRequest) (modelconfigbiz.Update, error) {
	if request.ApiKey != nil && request.GetClearApiKey() {
		return modelconfigbiz.Update{}, kerrors.BadRequest(
			"INVALID_ARGUMENT", "apiKey and clearApiKey cannot be used together",
		)
	}
	mode, err := modeFromProto(request.GetConnectionMode())
	if err != nil {
		return modelconfigbiz.Update{}, err
	}
	protocol, err := protocolFromProto(request.GetProtocol())
	if err != nil {
		return modelconfigbiz.Update{}, err
	}
	apiKey := request.ApiKey
	if request.GetClearApiKey() {
		empty := ""
		apiKey = &empty
	}
	return modelconfigbiz.Update{
		Connection: modelconfigbiz.Connection{
			Mode:                  mode,
			Protocol:              protocol,
			Endpoint:              request.GetEndpoint(),
			Model:                 request.GetModel(),
			Timeout:               time.Duration(request.GetTimeoutSeconds()) * time.Second,
			MaxOutputTokens:       int(request.GetMaxOutputTokens()),
			ReasoningBudgetTokens: int(request.GetReasoningBudgetTokens()),
		},
		APIKey: apiKey,
	}, nil
}

func protocolFromProto(value assistantv1.ModelProtocol) (modelconfigbiz.Protocol, error) {
	switch value {
	case assistantv1.ModelProtocol_MODEL_PROTOCOL_OPENAI_COMPATIBLE:
		return modelconfigbiz.ProtocolOpenAICompatible, nil
	case assistantv1.ModelProtocol_MODEL_PROTOCOL_ANTHROPIC:
		return modelconfigbiz.ProtocolAnthropic, nil
	default:
		return 0, kerrors.BadRequest("INVALID_ARGUMENT", "protocol is required")
	}
}

func modeFromProto(value assistantv1.ModelConnectionMode) (modelconfigbiz.Mode, error) {
	switch value {
	case assistantv1.ModelConnectionMode_MODEL_CONNECTION_MODE_DIRECT:
		return modelconfigbiz.ModeDirect, nil
	case assistantv1.ModelConnectionMode_MODEL_CONNECTION_MODE_INGATE:
		return modelconfigbiz.ModeIngate, nil
	default:
		return 0, kerrors.BadRequest("INVALID_ARGUMENT", "connectionMode is required")
	}
}

func modelConnectionResponse(connection modelconfigbiz.Connection) *assistantv1.ModelConnection {
	response := &assistantv1.ModelConnection{
		Configured:            connection.Configured,
		ConnectionMode:        modeToProto(connection.Mode),
		Protocol:              protocolToProto(connection.Protocol),
		Endpoint:              connection.Endpoint,
		Model:                 connection.Model,
		ApiKeyConfigured:      connection.APIKey != "",
		TimeoutSeconds:        uint32(connection.Timeout / time.Second),
		MaxOutputTokens:       uint32(connection.MaxOutputTokens),
		ReasoningBudgetTokens: uint32(connection.ReasoningBudgetTokens),
	}
	if connection.Configured {
		response.UpdatedAt = timestamppb.New(connection.UpdatedAt)
	}
	return response
}

func protocolToProto(value modelconfigbiz.Protocol) assistantv1.ModelProtocol {
	switch value {
	case modelconfigbiz.ProtocolOpenAICompatible:
		return assistantv1.ModelProtocol_MODEL_PROTOCOL_OPENAI_COMPATIBLE
	case modelconfigbiz.ProtocolAnthropic:
		return assistantv1.ModelProtocol_MODEL_PROTOCOL_ANTHROPIC
	default:
		return assistantv1.ModelProtocol_MODEL_PROTOCOL_UNSPECIFIED
	}
}

func modeToProto(value modelconfigbiz.Mode) assistantv1.ModelConnectionMode {
	switch value {
	case modelconfigbiz.ModeDirect:
		return assistantv1.ModelConnectionMode_MODEL_CONNECTION_MODE_DIRECT
	case modelconfigbiz.ModeIngate:
		return assistantv1.ModelConnectionMode_MODEL_CONNECTION_MODE_INGATE
	default:
		return assistantv1.ModelConnectionMode_MODEL_CONNECTION_MODE_UNSPECIFIED
	}
}

func mapError(err error) error {
	if errors.Is(err, modelconfigbiz.ErrInvalidConnection) {
		return kerrors.BadRequest("INVALID_ARGUMENT", "model connection is invalid")
	}
	return kerrors.InternalServer("INTERNAL_ERROR", "request failed").WithCause(err)
}
