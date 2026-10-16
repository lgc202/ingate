package envoy

import (
	"fmt"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	matcherv3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
)

// 官方 Envoy 标准 Ubuntu 镜像安装 ca-certificates；此路径由 Envoy 读取，不能读取 Controller 的根证书代替。
const systemCABundle = "/etc/ssl/certs/ca-certificates.crt"

// newUpstreamTLSSocket 转换已校验的 TLS 意图，系统信任根是此适配器的固定能力。
// HTTP 输入已经过权威资源校验，模型/MCP 输入已在 buildAIResources 通过派生配置校验。
func newUpstreamTLSSocket(settings dataplane.BackendTLSSettings) (*corev3.TransportSocket, error) {
	// Envoy 1.39 的 system_root_certs 尚未实现，必须指定系统 CA 文件并显式检查 DNS SAN。
	// 与 Envoy Gateway buildValidationContext 的显式 SNI 分支一致，DNS exact matcher 由 Envoy 执行证书域名匹配。
	// 不使用客户端 Host 或端点地址替换配置的身份；来源见 docs/design/ai-gateway-upstream-alignment.md。
	tlsContext := &tlsv3.UpstreamTlsContext{
		Sni: settings.SNI,
		CommonTlsContext: &tlsv3.CommonTlsContext{
			ValidationContextType: &tlsv3.CommonTlsContext_ValidationContext{
				ValidationContext: &tlsv3.CertificateValidationContext{
					TrustedCa: &corev3.DataSource{
						Specifier: &corev3.DataSource_Filename{Filename: systemCABundle},
					},
					MatchTypedSubjectAltNames: []*tlsv3.SubjectAltNameMatcher{{
						SanType: tlsv3.SubjectAltNameMatcher_DNS,
						Matcher: &matcherv3.StringMatcher{
							MatchPattern: &matcherv3.StringMatcher_Exact{Exact: settings.SNI},
						},
					}},
				},
			},
		},
	}
	if err := tlsContext.ValidateAll(); err != nil {
		return nil, fmt.Errorf("validate upstream TLS context: %w", err)
	}

	config, err := anypb.New(tlsContext)
	if err != nil {
		return nil, fmt.Errorf("marshal upstream TLS context: %w", err)
	}

	return &corev3.TransportSocket{
		Name:       "envoy.transport_sockets.tls",
		ConfigType: &corev3.TransportSocket_TypedConfig{TypedConfig: config},
	}, nil
}
