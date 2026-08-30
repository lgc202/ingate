package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	aiextprocv1 "github.com/lgc202/ingate/api/aiextproc/v1"
	"github.com/lgc202/ingate/internal/aiextproc/biz/tokenquota"
)

// TokenQuotaUsageService 将 AI ExtProc 的实时额度转换为内部 gRPC 协议。
type TokenQuotaUsageService struct {
	quotas *tokenquota.Limiter
}

// NewTokenQuotaUsageService 创建 Token 额度查询服务。
func NewTokenQuotaUsageService(quotas *tokenquota.Limiter) *TokenQuotaUsageService {
	return &TokenQuotaUsageService{quotas: quotas}
}

// GetCallerUsage 查询调用方当前命中的全部自然周期额度。
func (s *TokenQuotaUsageService) GetCallerUsage(
	ctx context.Context,
	request *aiextprocv1.GetCallerUsageRequest,
) (*aiextprocv1.GetCallerUsageResponse, error) {
	callerID, err := uuid.Parse(request.GetCallerId())
	if err != nil || callerID.String() != request.GetCallerId() {
		return nil, status.Error(codes.InvalidArgument, "caller_id must be a canonical UUID")
	}
	usages, err := s.quotas.CurrentUsage(ctx, callerID.String(), time.Now())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "query token quota usage: %v", err)
	}
	response := &aiextprocv1.GetCallerUsageResponse{Usages: make([]*aiextprocv1.TokenQuotaUsage, len(usages))}
	for i, usage := range usages {
		response.Usages[i] = &aiextprocv1.TokenQuotaUsage{
			PolicyId:    usage.PolicyID,
			PolicyName:  usage.PolicyName,
			Period:      tokenQuotaPeriodResponse(usage.Period),
			UsedTokens:  usage.Used,
			LimitTokens: usage.Limit,
			StartedAt:   timestamppb.New(usage.Start),
			ResetsAt:    timestamppb.New(usage.End),
		}
	}
	return response, nil
}

func tokenQuotaPeriodResponse(period tokenquota.Period) aiextprocv1.TokenQuotaPeriod {
	switch period {
	case tokenquota.PeriodDay:
		return aiextprocv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_DAY
	case tokenquota.PeriodWeek:
		return aiextprocv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_WEEK
	case tokenquota.PeriodMonth:
		return aiextprocv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_MONTH
	default:
		return aiextprocv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_UNSPECIFIED
	}
}
