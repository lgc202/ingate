package dto

import (
	"errors"
	"fmt"
	"strings"
)

// PolicyTargetKind 表示控制台可选择的策略作用目标类型
type PolicyTargetKind string

const (
	// PolicyTargetKindGateway 表示策略应用到 Gateway
	PolicyTargetKindGateway PolicyTargetKind = "Gateway"
	// PolicyTargetKindRoute 表示策略应用到 Route
	PolicyTargetKindRoute PolicyTargetKind = "Route"
)

// PolicyTargetReq 表示创建或更新策略时选择的作用目标
type PolicyTargetReq struct {
	Kind PolicyTargetKind `json:"kind"`
	ID   string           `json:"id"`
}

// PolicyTarget 表示控制台展示的策略作用目标及其生效状态
type PolicyTarget struct {
	Kind        PolicyTargetKind `json:"kind"`
	ID          string           `json:"id"`
	DisplayName string           `json:"displayName,omitempty"`
	Status      ResourceStatus   `json:"status"`
}

// ValidatePolicyTargets 校验并规范化策略作用目标
func ValidatePolicyTargets(targets []PolicyTargetReq) error {
	seen := make(map[string]struct{}, len(targets))
	for i := range targets {
		target := &targets[i]
		target.ID = strings.TrimSpace(target.ID)
		if target.ID == "" {
			return errors.New("策略作用目标不能为空")
		}
		switch target.Kind {
		case PolicyTargetKindGateway, PolicyTargetKindRoute:
		default:
			return errors.New("策略作用目标只支持网关或路由")
		}

		key := string(target.Kind) + "\x00" + target.ID
		if _, exists := seen[key]; exists {
			return fmt.Errorf("策略作用目标 %q 重复", target.ID)
		}
		seen[key] = struct{}{}
	}
	return nil
}
