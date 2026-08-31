package tool

import (
	"context"
	"fmt"
	"strings"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/google/uuid"

	changebiz "github.com/lgc202/ingate/internal/assistant/biz/change"
	"github.com/lgc202/ingate/internal/pkg/gatewayconfig"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
)

type proposeGatewayInput struct {
	Name      string                   `json:"name" jsonschema_description:"Gateway 的展示名称"`
	Enabled   *bool                    `json:"enabled,omitempty" jsonschema_description:"创建后是否立即参与配置下发，默认 true"`
	Listeners []proposeGatewayListener `json:"listeners" jsonschema_description:"完整监听入口列表，至少一项"`
}

type proposeGatewayListener struct {
	Name          string `json:"name" jsonschema_description:"当前 Gateway 内唯一的小写标识，例如 http 或 public-https"`
	Protocol      string `json:"protocol,omitempty" jsonschema_description:"http 或 https，默认 http"`
	Port          uint32 `json:"port" jsonschema_description:"监听端口，1 到 65535"`
	Hostname      string `json:"hostname,omitempty" jsonschema_description:"可选域名，留空表示接受任意 Host"`
	CertificateID string `json:"certificate_id,omitempty" jsonschema_description:"HTTPS 必填的证书资源 ID；HTTP 不得填写"`
}

func newCreateGatewayTool(writer ChangeWriter) (einotool.BaseTool, error) {
	definition, err := utils.InferTool(
		createGatewayTool,
		"创建 Gateway。工具会先中断并等待管理员审批；只有批准当前配置后才会写入。用户提出修改时，必须使用修改后的完整参数再次调用。提交前必须收集完整监听配置。",
		func(ctx context.Context, input proposeGatewayInput) (changeToolOutput, error) {
			prepared, err := proposeGateway(input)
			if err != nil || prepared.Status != "approval_required" {
				return changeToolOutput{Summary: prepared.Summary, Status: prepared.Status}, err
			}
			return executeWithApproval(ctx, writer, prepared)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("define %s tool: %w", createGatewayTool, err)
	}
	return definition, nil
}

func proposeGateway(input proposeGatewayInput) (proposalToolOutput, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 256 {
		return proposalInputResult(invalidInputf("gateway name must contain 1 to 256 bytes"))
	}
	if len(input.Listeners) == 0 || len(input.Listeners) > gatewayconfig.MaxListeners {
		return proposalInputResult(invalidInputf(
			"gateway listeners must contain between 1 and %d entries",
			gatewayconfig.MaxListeners,
		))
	}

	listeners := make([]changebiz.GatewayListener, 0, len(input.Listeners))
	seenNames := make(map[string]bool, len(input.Listeners))
	for _, candidate := range input.Listeners {
		listener, err := normalizeGatewayListener(candidate)
		if err != nil {
			return proposalInputResult(err)
		}
		if seenNames[listener.Name] {
			return proposalInputResult(invalidInputf(
				"gateway listener name %q is duplicated",
				listener.Name,
			))
		}
		if gatewayListenersOverlap(listener, listeners) {
			return proposalInputResult(invalidInputf(
				"listener %q overlaps another listener on port %d",
				listener.Name,
				listener.Port,
			))
		}
		seenNames[listener.Name] = true
		listeners = append(listeners, listener)
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	proposal := changebiz.Proposal{
		Kind: changebiz.KindCreateGateway,
		Gateway: &changebiz.CreateGateway{
			Name:      name,
			Enabled:   enabled,
			Listeners: listeners,
		},
	}
	if err := proposal.Validate(); err != nil {
		return proposalInputResult(invalidInputf("gateway configuration is invalid: %v", err))
	}
	return proposalToolOutput{
		Summary:  fmt.Sprintf("已准备创建网关 %q 的审批项", name),
		Status:   "approval_required",
		Proposal: &proposal,
	}, nil
}

func gatewayListenersOverlap(
	listener changebiz.GatewayListener,
	existing []changebiz.GatewayListener,
) bool {
	for _, current := range existing {
		if listener.Port != current.Port {
			continue
		}
		if listener.Protocol != current.Protocol ||
			hostnameutil.Overlaps(gatewayListenerHostname(listener), gatewayListenerHostname(current)) {
			return true
		}
	}
	return false
}

func gatewayListenerHostname(listener changebiz.GatewayListener) string {
	if listener.Hostname == "" {
		return "*"
	}
	return listener.Hostname
}

func normalizeGatewayListener(input proposeGatewayListener) (changebiz.GatewayListener, error) {
	name := strings.TrimSpace(input.Name)
	if !gatewayconfig.IsValidListenerName(name) {
		return changebiz.GatewayListener{}, invalidInputf(
			"listener name %q must be a lowercase DNS label",
			input.Name,
		)
	}
	if !gatewayconfig.IsValidListenerPort(int(input.Port)) {
		return changebiz.GatewayListener{}, invalidInputf(
			"listener %q port must be between 1 and 65535",
			name,
		)
	}

	hostnameValue := strings.ToLower(strings.TrimSpace(input.Hostname))
	hostname, ok := hostnameutil.Normalize(hostnameValue)
	if !ok || hostnameValue == "*" {
		return changebiz.GatewayListener{}, invalidInputf(
			"listener %q hostname is invalid; omit it to accept any Host",
			name,
		)
	}
	if hostname == "*" {
		hostname = ""
	}

	protocol := changebiz.GatewayProtocol(strings.ToLower(strings.TrimSpace(input.Protocol)))
	if protocol == "" {
		protocol = changebiz.GatewayProtocolHTTP
	}
	certificateID := strings.TrimSpace(input.CertificateID)
	switch protocol {
	case changebiz.GatewayProtocolHTTP:
		if certificateID != "" {
			return changebiz.GatewayListener{}, invalidInputf(
				"HTTP listener %q cannot contain certificate_id",
				name,
			)
		}
	case changebiz.GatewayProtocolHTTPS:
		if uuid.Validate(certificateID) != nil {
			return changebiz.GatewayListener{}, invalidInputf(
				"HTTPS listener %q requires a valid certificate_id",
				name,
			)
		}
	default:
		return changebiz.GatewayListener{}, invalidInputf(
			"listener %q protocol must be http or https",
			name,
		)
	}
	return changebiz.GatewayListener{
		Name:          name,
		Protocol:      protocol,
		Port:          input.Port,
		Hostname:      hostname,
		CertificateID: certificateID,
	}, nil
}
