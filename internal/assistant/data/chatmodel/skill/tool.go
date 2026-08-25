package skill

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// LoadToolName 是模型选择运维流程时调用的工具名。
const LoadToolName = "load_skill"

type loadSkillInput struct {
	Name string `json:"name"`
}

type loadSkillOutput struct {
	Summary      string   `json:"summary"`
	Instructions string   `json:"instructions"`
	AllowedTools []string `json:"allowed_tools"`
}

// NewLoadTool 为单次 Run 创建独立的 Skill 加载工具。
func NewLoadTool(session *Session) (tool.BaseTool, error) {
	definition, err := utils.InferTool(
		LoadToolName,
		"加载一个内置运维 Skill，获取任务步骤和本次 Run 可使用的工具。",
		func(_ context.Context, input loadSkillInput) (loadSkillOutput, error) {
			return loadSkill(session, input.Name)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("define %s tool: %w", LoadToolName, err)
	}
	return definition, nil
}

func loadSkill(session *Session, name string) (loadSkillOutput, error) {
	definition, err := session.Load(name)
	if err != nil {
		return loadSkillOutput{}, err
	}
	return loadSkillOutput{
		Summary:      fmt.Sprintf("已加载运维流程：%s", definition.Description),
		Instructions: definition.Instructions,
		AllowedTools: definition.AllowedTools,
	}, nil
}

// Instruction 把内置 Skill 摘要和加载约束追加到 Agent 基础指令。
func (c *Catalog) Instruction(base string) string {
	var prompt strings.Builder
	prompt.WriteString(base)
	prompt.WriteString("\n\n可用 Skill：")
	for _, definition := range c.Definitions() {
		prompt.WriteString("\n- ")
		prompt.WriteString(definition.Name)
		prompt.WriteString("：")
		prompt.WriteString(definition.Description)
	}
	prompt.WriteString(`

当用户请求符合某个 Skill 时，必须先调用 load_skill 加载它，再使用该 Skill 允许的工具。
load_skill 返回的内容是本次任务的执行约束；不得绕过 Skill 直接调用其他工具。`)
	return prompt.String()
}
