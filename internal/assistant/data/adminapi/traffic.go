package adminapi

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	agenttool "github.com/lgc202/ingate/internal/assistant/biz/agent/tool"
)

// AnalyzeTraffic 查询指定时间和资源范围内的汇总指标与资源排名。
func (c *Client) AnalyzeTraffic(
	ctx context.Context,
	query agenttool.TrafficQuery,
) (agenttool.TrafficAnalysis, error) {
	request := &adminv1.GetTrafficAnalysisRequest{
		StartTime:          timestamppb.New(query.StartTime),
		EndTime:            timestamppb.New(query.EndTime),
		BreakdownDimension: trafficDimension(query.GroupBy),
		BreakdownLimit:     query.Limit,
		BreakdownOrder:     trafficOrder(query.OrderBy),
	}
	applyTrafficScope(request, query.ScopeType, query.ScopeID)
	result, err := c.traffic.GetTrafficAnalysis(ctx, request)
	if err != nil {
		return agenttool.TrafficAnalysis{}, fmt.Errorf("get traffic analysis from Admin API: %w", err)
	}
	if result == nil {
		return agenttool.TrafficAnalysis{}, errors.New("get traffic analysis from Admin API: empty response")
	}
	if result.GetBreakdownDimension() != request.GetBreakdownDimension() ||
		result.GetBreakdownOrder() != request.GetBreakdownOrder() {
		return agenttool.TrafficAnalysis{}, errors.New(
			"get traffic analysis from Admin API: response breakdown does not match request",
		)
	}
	if err := validateTrafficMetricsResponse(result.GetSummary()); err != nil {
		return agenttool.TrafficAnalysis{}, fmt.Errorf("validate traffic summary: %w", err)
	}

	breakdown := result.GetBreakdown()
	if len(breakdown) > int(query.Limit) {
		return agenttool.TrafficAnalysis{}, errors.New(
			"get traffic analysis from Admin API: response exceeds the requested limit",
		)
	}
	items := make([]agenttool.ResourceTrafficMetrics, len(breakdown))
	seen := make(map[string]bool, len(breakdown))
	group, lookupCtx := errgroup.WithContext(ctx)
	group.SetLimit(resourceLookupConcurrency)
	for index, item := range breakdown {
		if item == nil || !validResourceID(item.GetResourceId()) {
			return agenttool.TrafficAnalysis{}, errors.New(
				"get traffic analysis from Admin API: invalid breakdown item",
			)
		}
		resourceID := item.GetResourceId()
		if seen[resourceID] {
			return agenttool.TrafficAnalysis{}, errors.New(
				"get traffic analysis from Admin API: duplicate breakdown item",
			)
		}
		seen[resourceID] = true
		if err := validateTrafficMetricsResponse(item.GetMetrics()); err != nil {
			return agenttool.TrafficAnalysis{}, fmt.Errorf(
				"validate traffic breakdown for %s: %w",
				resourceID,
				err,
			)
		}
		group.Go(func() error {
			name, err := c.resourceName(lookupCtx, query.GroupBy, resourceID)
			if err != nil {
				return err
			}
			items[index] = agenttool.ResourceTrafficMetrics{
				ID:      resourceID,
				Name:    name,
				Metrics: trafficMetrics(item.GetMetrics()),
			}
			return nil
		})
	}
	// 排名已经限制返回数量，各资源名称之间没有依赖。并行精确查询既保持排名顺序，
	// 也避免按条目数线性叠加 Admin API 往返时间。
	if err := group.Wait(); err != nil {
		return agenttool.TrafficAnalysis{}, err
	}

	return agenttool.TrafficAnalysis{
		Summary: trafficMetrics(result.GetSummary()),
		GroupBy: query.GroupBy,
		OrderBy: query.OrderBy,
		Items:   items,
	}, nil
}

// 资源范围由工具业务协议表达，只有此处知道对应的 Admin API 字段。
func applyTrafficScope(request *adminv1.GetTrafficAnalysisRequest, scopeType, scopeID string) {
	switch scopeType {
	case "gateway":
		request.GatewayId = scopeID
	case "route":
		request.RouteId = scopeID
	case "service":
		request.ServiceId = scopeID
	}
}

func trafficDimension(value agenttool.TrafficDimension) adminv1.TrafficBreakdownDimension {
	switch value {
	case agenttool.TrafficDimensionGateway:
		return adminv1.TrafficBreakdownDimension_TRAFFIC_BREAKDOWN_DIMENSION_GATEWAY
	case agenttool.TrafficDimensionService:
		return adminv1.TrafficBreakdownDimension_TRAFFIC_BREAKDOWN_DIMENSION_SERVICE
	default:
		return adminv1.TrafficBreakdownDimension_TRAFFIC_BREAKDOWN_DIMENSION_ROUTE
	}
}

func trafficOrder(value agenttool.TrafficOrder) adminv1.TrafficBreakdownOrder {
	switch value {
	case agenttool.TrafficOrderServerErrorRate:
		return adminv1.TrafficBreakdownOrder_TRAFFIC_BREAKDOWN_ORDER_SERVER_ERROR_RATE
	case agenttool.TrafficOrderP95Duration:
		return adminv1.TrafficBreakdownOrder_TRAFFIC_BREAKDOWN_ORDER_P95_DURATION
	default:
		return adminv1.TrafficBreakdownOrder_TRAFFIC_BREAKDOWN_ORDER_REQUEST_COUNT
	}
}

// 工具结果必须包含用户可识别的名称，避免模型为解释 UUID 再发起一轮工具调用。
// 已删除资源的历史流量仍可能出现在排名中，此时保留 ID 作为可追溯名称。
func (c *Client) resourceName(
	ctx context.Context,
	dimension agenttool.TrafficDimension,
	resourceID string,
) (string, error) {
	switch dimension {
	case agenttool.TrafficDimensionGateway:
		gateway, err := c.gateways.GetGateway(ctx, &adminv1.GetGatewayRequest{Id: resourceID})
		return resourceNameResult(dimension, resourceID, gateway.GetId(), gateway.GetName(), err)
	case agenttool.TrafficDimensionService:
		service, err := c.services.GetService(ctx, &adminv1.GetServiceRequest{Id: resourceID})
		return resourceNameResult(dimension, resourceID, service.GetId(), service.GetName(), err)
	default:
		route, err := c.routes.GetRoute(ctx, &adminv1.GetRouteRequest{Id: resourceID})
		return resourceNameResult(dimension, resourceID, route.GetId(), route.GetName(), err)
	}
}

func resourceNameResult(
	dimension agenttool.TrafficDimension,
	resourceID string,
	responseID string,
	name string,
	err error,
) (string, error) {
	if status.Code(err) == codes.NotFound {
		return resourceID, nil
	}
	if err != nil {
		return "", fmt.Errorf("get %s %s from Admin API: %w", dimension, resourceID, err)
	}
	if responseID != resourceID || name == "" {
		return "", fmt.Errorf("get %s %s from Admin API: invalid response", dimension, resourceID)
	}
	return name, nil
}

func trafficMetrics(metrics *adminv1.TrafficMetrics) agenttool.TrafficMetrics {
	return agenttool.TrafficMetrics{
		RequestCount:     metrics.GetRequestCount(),
		NonErrorCount:    metrics.GetNonErrorCount(),
		ClientErrorCount: metrics.GetClientErrorCount(),
		ServerErrorCount: metrics.GetServerErrorCount(),
		NoResponseCount:  metrics.GetNoResponseCount(),
		AverageDuration:  protoDuration(metrics.GetAverageDuration()),
		P50Duration:      protoDuration(metrics.GetP50Duration()),
		P95Duration:      protoDuration(metrics.GetP95Duration()),
		P99Duration:      protoDuration(metrics.GetP99Duration()),
	}
}

func validateTrafficMetricsResponse(metrics *adminv1.TrafficMetrics) error {
	if metrics == nil {
		return errors.New("traffic metrics are missing")
	}
	durations := []*durationpb.Duration{
		metrics.GetAverageDuration(),
		metrics.GetP50Duration(),
		metrics.GetP95Duration(),
		metrics.GetP99Duration(),
	}
	for _, duration := range durations {
		if !validDuration(duration) {
			return errors.New("traffic metrics contain an invalid duration")
		}
	}
	if metrics.GetP50Duration().AsDuration() > metrics.GetP95Duration().AsDuration() ||
		metrics.GetP95Duration().AsDuration() > metrics.GetP99Duration().AsDuration() {
		return errors.New("traffic metrics contain unordered duration percentiles")
	}
	remaining := metrics.GetRequestCount()
	for _, count := range []uint64{
		metrics.GetNonErrorCount(),
		metrics.GetClientErrorCount(),
		metrics.GetServerErrorCount(),
		metrics.GetNoResponseCount(),
	} {
		if count > remaining {
			return errors.New("traffic metrics contain inconsistent request counts")
		}
		remaining -= count
	}
	if remaining != 0 {
		return errors.New("traffic metrics do not classify every request")
	}
	return nil
}
