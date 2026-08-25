// Package skill 加载运维 Skill，并约束它在单次模型调用中的工具权限。
package skill

import "slices"

// Definition 描述一个可由助手选择的运维流程。
// Instructions 来自受信任的 Skill 内容，AllowedTools 限定流程可调用的原子工具。
type Definition struct {
	Name         string
	Description  string
	Instructions string
	AllowedTools []string
}

func cloneDefinition(definition Definition) Definition {
	definition.AllowedTools = slices.Clone(definition.AllowedTools)
	return definition
}
