package skill

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Session 保存一次执行已加载的 Skill 和工具权限。
type Session struct {
	catalog    *Catalog
	activeName string
}

// NewSession 创建只属于当前执行的 Skill 状态。
func NewSession(catalog *Catalog) *Session {
	return &Session{catalog: catalog}
}

// Load 选择本次执行使用的 Skill。
// 同一次执行不允许切换 Skill，避免已经执行的工具失去原有授权上下文。
func (s *Session) Load(name string) (Definition, error) {
	name = strings.TrimSpace(name)
	definition, ok := s.catalog.definition(name)
	if !ok {
		return Definition{}, fmt.Errorf("assistant skill %q is not available", name)
	}
	if s.activeName != "" && s.activeName != definition.Name {
		return Definition{}, errors.New("another assistant skill is already active")
	}
	s.activeName = definition.Name
	return definition, nil
}

// Authorize 检查当前 Skill 是否允许调用指定工具。
func (s *Session) Authorize(name string) error {
	if s.activeName == "" {
		return errors.New("assistant skill is not loaded")
	}
	definition, _ := s.catalog.definition(s.activeName)
	if !slices.Contains(definition.AllowedTools, name) {
		return fmt.Errorf("tool %q is not allowed by assistant skill %q", name, s.activeName)
	}
	return nil
}
