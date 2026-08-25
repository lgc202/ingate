package skill

import (
	"errors"
	"fmt"
	"strings"
)

// Catalog 保存当前可用的 Skill 定义。
// Catalog 创建后保持只读，可以被并发 Run 共享。
type Catalog struct {
	definitions []Definition
	byName      map[string]int
}

// NewCatalog 校验并创建只读 Skill 目录。
func NewCatalog(definitions []Definition) (*Catalog, error) {
	if len(definitions) == 0 {
		return nil, errors.New("assistant skill catalog is empty")
	}
	catalog := &Catalog{
		definitions: make([]Definition, 0, len(definitions)),
		byName:      make(map[string]int, len(definitions)),
	}
	for _, definition := range definitions {
		definition = normalizeDefinition(cloneDefinition(definition))
		if err := validateDefinition(definition); err != nil {
			return nil, err
		}
		if _, exists := catalog.byName[definition.Name]; exists {
			return nil, fmt.Errorf("duplicate assistant skill %q", definition.Name)
		}
		catalog.byName[definition.Name] = len(catalog.definitions)
		catalog.definitions = append(catalog.definitions, cloneDefinition(definition))
	}
	return catalog, nil
}

// Definitions 返回可供模型选择的 Skill 摘要和执行约束。
func (c *Catalog) Definitions() []Definition {
	definitions := make([]Definition, len(c.definitions))
	for i, definition := range c.definitions {
		definitions[i] = cloneDefinition(definition)
	}
	return definitions
}

func (c *Catalog) definition(name string) (Definition, bool) {
	index, ok := c.byName[name]
	if !ok {
		return Definition{}, false
	}
	return cloneDefinition(c.definitions[index]), true
}

func normalizeDefinition(definition Definition) Definition {
	definition.Name = strings.TrimSpace(definition.Name)
	definition.Description = strings.TrimSpace(definition.Description)
	definition.Instructions = strings.TrimSpace(definition.Instructions)
	for i := range definition.AllowedTools {
		definition.AllowedTools[i] = strings.TrimSpace(definition.AllowedTools[i])
	}
	return definition
}

func validateDefinition(definition Definition) error {
	if definition.Name == "" {
		return errors.New("assistant skill name is required")
	}
	if definition.Description == "" {
		return fmt.Errorf("assistant skill %q description is required", definition.Name)
	}
	if definition.Instructions == "" {
		return fmt.Errorf("assistant skill %q instructions are required", definition.Name)
	}
	if len(definition.AllowedTools) == 0 {
		return fmt.Errorf("assistant skill %q allowed tools are required", definition.Name)
	}
	for _, name := range definition.AllowedTools {
		if name == "" {
			return fmt.Errorf("assistant skill %q contains an empty tool name", definition.Name)
		}
	}
	return nil
}
