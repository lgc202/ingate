// Package skill 负责把随二进制发布的 SKILL.md 转换为模型执行期目录。
package skill

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"go.yaml.in/yaml/v3"
)

const builtinPattern = "builtin/*/SKILL.md"

//go:embed builtin/*/SKILL.md
var builtinFiles embed.FS

type metadata struct {
	Name         string `yaml:"name"`
	Description  string `yaml:"description"`
	AllowedTools string `yaml:"allowed-tools"`
}

// LoadBuiltin 从编译进二进制的 SKILL.md 创建 Skill 目录。
func LoadBuiltin() (*Catalog, error) {
	filenames, err := fs.Glob(builtinFiles, builtinPattern)
	if err != nil {
		return nil, fmt.Errorf("find builtin assistant skills: %w", err)
	}
	if len(filenames) == 0 {
		return nil, errors.New("no builtin assistant skills found")
	}

	definitions := make([]Definition, 0, len(filenames))
	for _, filename := range filenames {
		definition, err := loadDefinition(filename)
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, definition)
	}
	catalog, err := NewCatalog(definitions)
	if err != nil {
		return nil, fmt.Errorf("create builtin assistant skill catalog: %w", err)
	}
	return catalog, nil
}

func loadDefinition(filename string) (Definition, error) {
	content, err := builtinFiles.ReadFile(filename)
	if err != nil {
		return Definition{}, fmt.Errorf("read assistant skill %q: %w", filename, err)
	}
	definition, err := parse(content)
	if err != nil {
		return Definition{}, fmt.Errorf("parse assistant skill %q: %w", filename, err)
	}
	if directory := path.Base(path.Dir(filename)); directory != definition.Name {
		return Definition{}, fmt.Errorf(
			"assistant skill directory %q does not match name %q",
			directory,
			definition.Name,
		)
	}
	return definition, nil
}

func parse(content []byte) (Definition, error) {
	const delimiter = "---\n"
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	if !strings.HasPrefix(text, delimiter) {
		return Definition{}, errors.New("missing YAML front matter")
	}
	frontMatter, instructions, ok := strings.Cut(strings.TrimPrefix(text, delimiter), "\n"+delimiter)
	if !ok {
		return Definition{}, errors.New("unterminated YAML front matter")
	}

	var values metadata
	if err := yaml.Unmarshal([]byte(frontMatter), &values); err != nil {
		return Definition{}, fmt.Errorf("decode YAML front matter: %w", err)
	}
	return Definition{
		Name:         values.Name,
		Description:  values.Description,
		Instructions: instructions,
		AllowedTools: strings.Fields(values.AllowedTools),
	}, nil
}
