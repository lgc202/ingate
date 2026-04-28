package target

import (
	"fmt"
	"slices"
)

// Registry 保存可用的运行时 target 翻译器
type Registry struct {
	translators map[string]Translator
}

// NewRegistry 创建一个 target 翻译器注册表
func NewRegistry(translators ...Translator) (Registry, error) {
	registry := Registry{
		translators: make(map[string]Translator, len(translators)),
	}
	for _, translator := range translators {
		name := translator.Target()
		if _, ok := registry.translators[name]; ok {
			return Registry{}, fmt.Errorf("duplicate target %q", name)
		}
		registry.translators[name] = translator
	}

	return registry, nil
}

// Get 按名称查找 target 翻译器
func (r Registry) Get(name string) (Translator, bool) {
	translator, ok := r.translators[name]
	return translator, ok
}

// Names 返回已注册 target 名称
func (r Registry) Names() []string {
	names := make([]string, 0, len(r.translators))
	for name := range r.translators {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
