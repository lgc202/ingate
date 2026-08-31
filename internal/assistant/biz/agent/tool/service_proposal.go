package tool

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	changebiz "github.com/lgc202/ingate/internal/assistant/biz/change"
	"github.com/lgc202/ingate/internal/pkg/upstreamconfig"
)

type proposeServiceInput struct {
	Name          string                     `json:"name" jsonschema_description:"Service 的展示名称"`
	Endpoints     []proposeServiceEndpoint   `json:"endpoints" jsonschema_description:"完整服务端点列表，至少一项"`
	TLSServerName string                     `json:"tls_server_name,omitempty" jsonschema_description:"使用 HTTPS 访问上游时的 SNI 和证书校验名称；留空表示 HTTP"`
	LoadBalancing string                     `json:"load_balancing,omitempty" jsonschema_description:"round_robin 或 least_request，默认 round_robin"`
	HealthCheck   *proposeServiceHealthCheck `json:"health_check,omitempty" jsonschema_description:"可选的主动 HTTP 健康检查"`
}

type proposeServiceEndpoint struct {
	Address string `json:"address" jsonschema_description:"不含协议和端口的 DNS 主机名或 IP 地址"`
	Port    uint32 `json:"port" jsonschema_description:"服务监听端口，1 到 65535"`
	Weight  uint32 `json:"weight,omitempty" jsonschema_description:"相对权重，默认 1，最大 1000"`
}

type proposeServiceHealthCheck struct {
	Path            string `json:"path" jsonschema_description:"不含查询参数和片段的绝对路径"`
	IntervalSeconds uint32 `json:"interval_seconds,omitempty" jsonschema_description:"检查间隔，默认 10 秒，最大 300 秒"`
	TimeoutSeconds  uint32 `json:"timeout_seconds,omitempty" jsonschema_description:"单次超时，默认 2 秒，必须小于检查间隔"`
}

func newCreateServiceTool(writer ChangeWriter) (einotool.BaseTool, error) {
	definition, err := utils.InferTool(
		createServiceTool,
		"创建普通 HTTP Service。工具会先中断并等待管理员审批；只有批准当前配置后才会写入。用户提出修改时，必须使用修改后的完整参数再次调用。模型服务及 API Key 不在此工具能力范围内。",
		func(ctx context.Context, input proposeServiceInput) (changeToolOutput, error) {
			prepared, err := proposeService(input)
			if err != nil || prepared.Status != "approval_required" {
				return changeToolOutput{Summary: prepared.Summary, Status: prepared.Status}, err
			}
			return executeWithApproval(ctx, writer, prepared)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("define %s tool: %w", createServiceTool, err)
	}
	return definition, nil
}

func proposeService(input proposeServiceInput) (proposalToolOutput, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 256 {
		return proposalInputResult(invalidInputf("service name must contain 1 to 256 bytes"))
	}
	if len(input.Endpoints) == 0 || len(input.Endpoints) > upstreamconfig.MaxEndpoints {
		return proposalInputResult(invalidInputf(
			"service endpoints must contain between 1 and %d entries",
			upstreamconfig.MaxEndpoints,
		))
	}

	endpoints := make([]changebiz.ServiceEndpoint, 0, len(input.Endpoints))
	seenEndpoints := make(map[string]bool, len(input.Endpoints))
	for _, candidate := range input.Endpoints {
		endpoint, err := normalizeServiceEndpoint(candidate)
		if err != nil {
			return proposalInputResult(err)
		}
		key := net.JoinHostPort(endpoint.Address, strconv.Itoa(int(endpoint.Port)))
		if seenEndpoints[key] {
			return proposalInputResult(invalidInputf("service endpoint %q is duplicated", key))
		}
		seenEndpoints[key] = true
		endpoints = append(endpoints, endpoint)
	}

	tlsServerName := upstreamconfig.NormalizeAddress(input.TLSServerName)
	if tlsServerName != "" && !upstreamconfig.IsValidAddress(tlsServerName) {
		return proposalInputResult(invalidInputf("tls_server_name must be a valid DNS name or IP address"))
	}
	loadBalancing, err := normalizeLoadBalancing(input.LoadBalancing)
	if err != nil {
		return proposalInputResult(err)
	}
	healthCheck, err := normalizeHealthCheck(input.HealthCheck)
	if err != nil {
		return proposalInputResult(err)
	}

	proposal := changebiz.Proposal{
		Kind: changebiz.KindCreateService,
		Service: &changebiz.CreateService{
			Name:          name,
			Endpoints:     endpoints,
			TLSServerName: tlsServerName,
			LoadBalancing: loadBalancing,
			HealthCheck:   healthCheck,
		},
	}
	if err := proposal.Validate(); err != nil {
		return proposalInputResult(invalidInputf("service configuration is invalid: %v", err))
	}
	return proposalToolOutput{
		Summary:  fmt.Sprintf("已准备创建服务 %q 的审批项", name),
		Status:   "approval_required",
		Proposal: &proposal,
	}, nil
}

func normalizeServiceEndpoint(input proposeServiceEndpoint) (changebiz.ServiceEndpoint, error) {
	address := upstreamconfig.NormalizeAddress(input.Address)
	if !upstreamconfig.IsValidAddress(address) {
		return changebiz.ServiceEndpoint{}, invalidInputf(
			"service endpoint address %q is invalid",
			input.Address,
		)
	}
	if !upstreamconfig.IsValidEndpointPort(int(input.Port)) {
		return changebiz.ServiceEndpoint{}, invalidInputf(
			"service endpoint %q port must be between 1 and 65535",
			address,
		)
	}
	weight := input.Weight
	if weight == 0 {
		weight = upstreamconfig.DefaultEndpointWeight
	}
	if !upstreamconfig.IsValidEndpointWeight(int(weight)) {
		return changebiz.ServiceEndpoint{}, invalidInputf(
			"service endpoint %q weight must be between 1 and 1000",
			address,
		)
	}
	return changebiz.ServiceEndpoint{
		Address: address,
		Port:    input.Port,
		Weight:  weight,
	}, nil
}

func normalizeLoadBalancing(value string) (changebiz.LoadBalancing, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", string(changebiz.LoadBalancingRoundRobin):
		return changebiz.LoadBalancingRoundRobin, nil
	case string(changebiz.LoadBalancingLeastRequest):
		return changebiz.LoadBalancingLeastRequest, nil
	default:
		return "", invalidInputf(
			"load_balancing %q is invalid; use round_robin or least_request",
			value,
		)
	}
}

func normalizeHealthCheck(
	input *proposeServiceHealthCheck,
) (*changebiz.ServiceHealthCheck, error) {
	if input == nil {
		return nil, nil
	}
	path := strings.TrimSpace(input.Path)
	if !upstreamconfig.IsValidHealthCheckPath(path) {
		return nil, invalidInputf("health_check.path must be an absolute path without query or fragment")
	}
	interval := input.IntervalSeconds
	if interval == 0 {
		interval = upstreamconfig.DefaultHealthCheckIntervalSeconds
	}
	if !upstreamconfig.IsValidHealthCheckInterval(int(interval)) {
		return nil, invalidInputf("health_check.interval_seconds must be between 1 and 300")
	}
	timeout := input.TimeoutSeconds
	if timeout == 0 {
		timeout = upstreamconfig.DefaultHealthCheckTimeoutSeconds
	}
	if !upstreamconfig.IsValidHealthCheckTimeout(int(timeout), int(interval)) {
		return nil, invalidInputf(
			"health_check.timeout_seconds must be between 1 and 60 and shorter than the interval",
		)
	}
	return &changebiz.ServiceHealthCheck{
		Path:            path,
		IntervalSeconds: interval,
		TimeoutSeconds:  timeout,
	}, nil
}
