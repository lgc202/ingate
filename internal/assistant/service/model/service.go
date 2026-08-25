// Package model 适配运维助手模型连接的 HTTP API 和业务对象。
package model

import (
	"context"
	"errors"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	assistantv1 "github.com/lgc202/ingate/api/assistant/v1"
	modelbiz "github.com/lgc202/ingate/internal/assistant/biz/model"
)

// Service 实现模型连接的产品协议。
type Service struct {
	models *modelbiz.Service
}

// NewService 创建模型连接协议服务。
func NewService(models *modelbiz.Service) *Service {
	return &Service{models: models}
}

func (s *Service) GetModelConnection(
	ctx context.Context,
	_ *emptypb.Empty,
) (*assistantv1.ModelConnection, error) {
	connection, err := s.models.Get(ctx)
	if err != nil {
		return nil, s.mapError(err)
	}
	return s.modelConnectionResponse(connection), nil
}

func (s *Service) UpdateModelConnection(
	ctx context.Context,
	request *assistantv1.UpdateModelConnectionRequest,
) (*assistantv1.ModelConnection, error) {
	update, err := s.modelUpdate(request)
	if err != nil {
		return nil, err
	}
	connection, err := s.models.Update(ctx, update)
	if err != nil {
		return nil, s.mapError(err)
	}
	return s.modelConnectionResponse(connection), nil
}

func (s *Service) modelUpdate(request *assistantv1.UpdateModelConnectionRequest) (modelbiz.Update, error) {
	if request.ApiKey != nil && request.GetClearApiKey() {
		return modelbiz.Update{}, kratoserrors.BadRequest(
			"INVALID_ARGUMENT", "apiKey and clearApiKey cannot be used together",
		)
	}
	mode, err := modeFromProto(request.GetConnectionMode())
	if err != nil {
		return modelbiz.Update{}, err
	}
	protocol, err := protocolFromProto(request.GetProtocol())
	if err != nil {
		return modelbiz.Update{}, err
	}
	apiKey := request.ApiKey
	if request.GetClearApiKey() {
		empty := ""
		apiKey = &empty
	}
	return modelbiz.Update{
		Connection: modelbiz.Connection{
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

func protocolFromProto(value assistantv1.ModelProtocol) (modelbiz.Protocol, error) {
	switch value {
	case assistantv1.ModelProtocol_MODEL_PROTOCOL_OPENAI_COMPATIBLE:
		return modelbiz.ProtocolOpenAICompatible, nil
	case assistantv1.ModelProtocol_MODEL_PROTOCOL_ANTHROPIC:
		return modelbiz.ProtocolAnthropic, nil
	default:
		return 0, kratoserrors.BadRequest("INVALID_ARGUMENT", "protocol is required")
	}
}

func modeFromProto(value assistantv1.ModelConnectionMode) (modelbiz.Mode, error) {
	switch value {
	case assistantv1.ModelConnectionMode_MODEL_CONNECTION_MODE_DIRECT:
		return modelbiz.ModeDirect, nil
	case assistantv1.ModelConnectionMode_MODEL_CONNECTION_MODE_INGATE:
		return modelbiz.ModeIngate, nil
	default:
		return 0, kratoserrors.BadRequest("INVALID_ARGUMENT", "connectionMode is required")
	}
}

func (s *Service) modelConnectionResponse(connection modelbiz.Connection) *assistantv1.ModelConnection {
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

func protocolToProto(value modelbiz.Protocol) assistantv1.ModelProtocol {
	switch value {
	case modelbiz.ProtocolOpenAICompatible:
		return assistantv1.ModelProtocol_MODEL_PROTOCOL_OPENAI_COMPATIBLE
	case modelbiz.ProtocolAnthropic:
		return assistantv1.ModelProtocol_MODEL_PROTOCOL_ANTHROPIC
	default:
		return assistantv1.ModelProtocol_MODEL_PROTOCOL_UNSPECIFIED
	}
}

func modeToProto(value modelbiz.Mode) assistantv1.ModelConnectionMode {
	switch value {
	case modelbiz.ModeDirect:
		return assistantv1.ModelConnectionMode_MODEL_CONNECTION_MODE_DIRECT
	case modelbiz.ModeIngate:
		return assistantv1.ModelConnectionMode_MODEL_CONNECTION_MODE_INGATE
	default:
		return assistantv1.ModelConnectionMode_MODEL_CONNECTION_MODE_UNSPECIFIED
	}
}

func (s *Service) mapError(err error) error {
	if errors.Is(err, modelbiz.ErrInvalidConnection) {
		return kratoserrors.BadRequest("INVALID_ARGUMENT", "model connection is invalid")
	}
	return kratoserrors.InternalServer("INTERNAL_ERROR", "request failed").WithCause(err)
}
