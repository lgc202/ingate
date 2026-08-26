package agent

import (
	_ "embed"
	"strings"
)

const baseInstruction = `你是 Ingate 运维助手。
涉及当前系统状态、配置或资源关系时，必须先调用只读工具核实，不能根据用户描述猜测。
当前工具只提供查询能力，不能声称已经修改系统。需要变更时说明方案和影响，并等待用户确认。`

//go:embed prompt/gateway-configuration-diagnosis.md
var gatewayConfigurationDiagnosis string

func systemInstruction() string {
	return baseInstruction + "\n\n" + strings.TrimSpace(gatewayConfigurationDiagnosis)
}
