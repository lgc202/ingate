// Package change 管理运维助手提出并由管理员显式审批的配置变更。
package change

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/pkg/gatewayconfig"
	"github.com/lgc202/ingate/internal/pkg/hostname"
	"github.com/lgc202/ingate/internal/pkg/upstreamconfig"
)

const (
	// KindCreateGateway 创建一个 Gateway。
	KindCreateGateway Kind = "create_gateway"
	// KindCreateService 创建一个普通 HTTP Service。
	KindCreateService Kind = "create_service"
)

const (
	// StatePendingReview 表示变更正在等待管理员决定。
	StatePendingReview State = "pending_review"
	// StateExecuting 表示管理员已经批准，变更正在执行。
	StateExecuting State = "executing"
	// StateSucceeded 表示目标资源已经创建。
	StateSucceeded State = "succeeded"
	// StateRejected 表示管理员拒绝了该变更。
	StateRejected State = "rejected"
	// StateFailed 表示 Admin API 明确拒绝了该变更。
	StateFailed State = "failed"
	// StateOutcomeUnknown 表示无法确认 Admin API 是否已经完成创建。
	StateOutcomeUnknown State = "outcome_unknown"
)

const (
	// GatewayProtocolHTTP 表示普通 HTTP 监听入口。
	GatewayProtocolHTTP GatewayProtocol = "http"
	// GatewayProtocolHTTPS 表示由 Gateway 终止 TLS 的 HTTPS 监听入口。
	GatewayProtocolHTTPS GatewayProtocol = "https"
)

const (
	// LoadBalancingRoundRobin 使用轮询策略。
	LoadBalancingRoundRobin LoadBalancing = "round_robin"
	// LoadBalancingLeastRequest 优先选择活跃请求较少的端点。
	LoadBalancingLeastRequest LoadBalancing = "least_request"
)

const (
	// FailureAdminRejected 表示 Admin API 在执行前明确拒绝了配置。
	FailureAdminRejected FailureCode = "CHANGE_REJECTED"
	// FailureOutcomeUnknown 表示创建请求可能已经生效，系统不会自动重试。
	FailureOutcomeUnknown FailureCode = "CHANGE_OUTCOME_UNKNOWN"
)

var (
	// ErrNotFound 表示变更对当前管理员不可见。
	ErrNotFound = errors.New("assistant proposed change not found")
	// ErrStateConflict 表示当前状态不允许请求的审批操作。
	ErrStateConflict = errors.New("assistant proposed change state conflict")
	// ErrAdminRejected 表示 Admin API 明确未执行创建操作。
	ErrAdminRejected = errors.New("assistant proposed change rejected by Admin API")
)

// Kind 表示审批后执行的确定性操作。
type Kind string

// State 表示配置变更的审批和执行状态。
type State string

// GatewayProtocol 表示 Gateway 监听入口使用的协议。
type GatewayProtocol string

// LoadBalancing 表示 Service 多个端点之间的流量分配方式。
type LoadBalancing string

// FailureCode 是可以持久化并安全返回控制台的稳定失败码。
type FailureCode string

// ProposedChange 是一项已经进入审批流程的不可变配置变更。
type ProposedChange struct {
	ID             string
	ConversationID string
	ExecutionID    string
	InterruptID    string
	State          State
	Summary        string
	Proposal       Proposal
	ResourceID     string
	ErrorCode      FailureCode
	CreatedAt      time.Time
	DecidedAt      *time.Time
	FinishedAt     *time.Time
}

// Proposal 是 Agent 生成的规范化配置快照。
// Kind 与对应配置同时保存，使持久化数据损坏时能够在执行前被明确识别。
type Proposal struct {
	Kind    Kind           `json:"kind"`
	Gateway *CreateGateway `json:"gateway,omitempty"`
	Service *CreateService `json:"service,omitempty"`
}

// CreateGateway 是创建 Gateway 所需的完整配置。
type CreateGateway struct {
	Name      string            `json:"name"`
	Enabled   bool              `json:"enabled"`
	Listeners []GatewayListener `json:"listeners"`
}

// GatewayListener 是一个 Gateway 监听入口。
type GatewayListener struct {
	Name          string          `json:"name"`
	Protocol      GatewayProtocol `json:"protocol"`
	Port          uint32          `json:"port"`
	Hostname      string          `json:"hostname,omitempty"`
	CertificateID string          `json:"certificate_id,omitempty"`
}

// CreateService 是创建普通 HTTP Service 所需的完整配置。
type CreateService struct {
	Name          string              `json:"name"`
	Endpoints     []ServiceEndpoint   `json:"endpoints"`
	TLSServerName string              `json:"tls_server_name,omitempty"`
	LoadBalancing LoadBalancing       `json:"load_balancing"`
	HealthCheck   *ServiceHealthCheck `json:"health_check,omitempty"`
}

// ServiceEndpoint 是 Service 的一个可转发地址。
type ServiceEndpoint struct {
	Address string `json:"address"`
	Port    uint32 `json:"port"`
	Weight  uint32 `json:"weight"`
}

// ServiceHealthCheck 是 Service 的主动 HTTP 健康检查配置。
type ServiceHealthCheck struct {
	Path            string `json:"path"`
	IntervalSeconds uint32 `json:"interval_seconds"`
	TimeoutSeconds  uint32 `json:"timeout_seconds"`
}

// CreatedResource 是一次成功创建后需要返回给审批人的最小结果。
type CreatedResource struct {
	ID string
}

// Validate 检查提案是否为可以安全交给确定性执行器的规范化配置。
func (p Proposal) Validate() error {
	switch p.Kind {
	case KindCreateGateway:
		if p.Gateway == nil || p.Service != nil {
			return errors.New("create_gateway proposal contains invalid configuration")
		}
		return p.Gateway.validate()
	case KindCreateService:
		if p.Service == nil || p.Gateway != nil {
			return errors.New("create_service proposal contains invalid configuration")
		}
		return p.Service.validate()
	default:
		return fmt.Errorf("unsupported proposed change kind %q", p.Kind)
	}
}

func (c CreateGateway) validate() error {
	if c.Name == "" || strings.TrimSpace(c.Name) != c.Name || len(c.Name) > 256 ||
		len(c.Listeners) == 0 ||
		len(c.Listeners) > gatewayconfig.MaxListeners {
		return errors.New("invalid create_gateway configuration")
	}
	seenNames := make(map[string]bool, len(c.Listeners))
	for index, listener := range c.Listeners {
		if !gatewayconfig.IsValidListenerName(listener.Name) ||
			!gatewayconfig.IsValidListenerPort(int(listener.Port)) ||
			seenNames[listener.Name] {
			return errors.New("invalid create_gateway listener")
		}
		seenNames[listener.Name] = true
		if listener.Hostname != "" {
			normalized, ok := hostname.Normalize(listener.Hostname)
			if !ok || normalized != listener.Hostname || normalized == "*" {
				return errors.New("invalid create_gateway listener hostname")
			}
		}
		switch listener.Protocol {
		case GatewayProtocolHTTP:
			if listener.CertificateID != "" {
				return errors.New("HTTP listener contains a certificate")
			}
		case GatewayProtocolHTTPS:
			if uuid.Validate(listener.CertificateID) != nil {
				return errors.New("HTTPS listener has no valid certificate")
			}
		default:
			return errors.New("invalid create_gateway listener protocol")
		}
		if listenersOverlap(listener, c.Listeners[:index]) {
			return errors.New("create_gateway listeners contain overlapping claims")
		}
	}
	return nil
}

func listenersOverlap(listener GatewayListener, existing []GatewayListener) bool {
	for _, current := range existing {
		if listener.Port != current.Port {
			continue
		}
		if listener.Protocol != current.Protocol ||
			hostname.Overlaps(listenerHostname(listener), listenerHostname(current)) {
			return true
		}
	}
	return false
}

func listenerHostname(listener GatewayListener) string {
	if listener.Hostname == "" {
		return "*"
	}
	return listener.Hostname
}

func (c CreateService) validate() error {
	if c.Name == "" || strings.TrimSpace(c.Name) != c.Name || len(c.Name) > 256 ||
		len(c.Endpoints) == 0 ||
		len(c.Endpoints) > upstreamconfig.MaxEndpoints {
		return errors.New("invalid create_service configuration")
	}
	if c.LoadBalancing != LoadBalancingRoundRobin &&
		c.LoadBalancing != LoadBalancingLeastRequest {
		return errors.New("invalid create_service load balancing policy")
	}
	if c.TLSServerName != "" &&
		(!upstreamconfig.IsValidAddress(c.TLSServerName) ||
			upstreamconfig.NormalizeAddress(c.TLSServerName) != c.TLSServerName) {
		return errors.New("invalid create_service TLS server name")
	}
	seenEndpoints := make(map[string]bool, len(c.Endpoints))
	for _, endpoint := range c.Endpoints {
		key := net.JoinHostPort(endpoint.Address, strconv.Itoa(int(endpoint.Port)))
		if !upstreamconfig.IsValidAddress(endpoint.Address) ||
			upstreamconfig.NormalizeAddress(endpoint.Address) != endpoint.Address ||
			!upstreamconfig.IsValidEndpointPort(int(endpoint.Port)) ||
			!upstreamconfig.IsValidEndpointWeight(int(endpoint.Weight)) ||
			seenEndpoints[key] {
			return errors.New("invalid create_service endpoint")
		}
		seenEndpoints[key] = true
	}
	if c.HealthCheck != nil {
		healthCheck := c.HealthCheck
		if !upstreamconfig.IsValidHealthCheckPath(healthCheck.Path) ||
			!upstreamconfig.IsValidHealthCheckInterval(int(healthCheck.IntervalSeconds)) ||
			!upstreamconfig.IsValidHealthCheckTimeout(
				int(healthCheck.TimeoutSeconds),
				int(healthCheck.IntervalSeconds),
			) {
			return errors.New("invalid create_service health check")
		}
	}
	return nil
}
