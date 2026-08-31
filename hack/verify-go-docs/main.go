// Copyright 2026 Ingate Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command verify-go-docs 检查手写 Go 代码的包与导出声明文档。
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

type packageDocumentation struct {
	name       string
	documented bool
}

func main() {
	roots := os.Args[1:]
	if len(roots) == 0 {
		roots = []string{"."}
	}

	violations, err := verifyRoots(roots)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, violation := range violations {
		fmt.Fprintln(os.Stderr, violation)
	}
	if len(violations) != 0 {
		os.Exit(1)
	}
}

func verifyRoots(roots []string) ([]string, error) {
	var violations []string
	packages := make(map[string]*packageDocumentation)
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if path != root && ignoredDirectory(entry.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}

			fileViolations, packageName, documented, generated, err := verifyFile(path)
			if err != nil {
				return err
			}
			if generated {
				return nil
			}
			violations = append(violations, fileViolations...)
			directory := filepath.Dir(path)
			info := packages[directory]
			if info == nil {
				info = &packageDocumentation{name: packageName}
				packages[directory] = info
			}
			info.documented = info.documented || documented
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("verify Go documentation under %q: %w", root, err)
		}
	}

	directories := make([]string, 0, len(packages))
	for directory := range packages {
		directories = append(directories, directory)
	}
	slices.Sort(directories)
	for _, directory := range directories {
		info := packages[directory]
		if !info.documented {
			violations = append(violations, fmt.Sprintf(
				"%s: package %s has no package comment",
				directory,
				info.name,
			))
		}
	}
	return violations, nil
}

func verifyFile(path string) ([]string, string, bool, bool, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, parser.ParseComments)
	if err != nil {
		return nil, "", false, false, fmt.Errorf("parse %q: %w", path, err)
	}
	if ast.IsGenerated(file) {
		return nil, file.Name.Name, false, true, nil
	}

	violations := checkDirectiveComments(fileSet, path, file.Comments)
	if file.Doc != nil {
		violations = append(violations, checkComment(
			fileSet,
			path,
			"Package "+file.Name.Name,
			file.Package,
			file.Doc,
			file.Name.Name == "main",
		)...)
	}
	for _, declaration := range file.Decls {
		violations = append(violations, checkDeclaration(fileSet, path, declaration)...)
	}
	return violations, file.Name.Name, file.Doc != nil, false, nil
}

func checkDirectiveComments(
	fileSet *token.FileSet,
	path string,
	groups []*ast.CommentGroup,
) []string {
	var violations []string
	for _, group := range groups {
		for _, comment := range group.List {
			text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
			if !strings.HasPrefix(text, "+") || !hasTerminalPunctuation(text) {
				continue
			}
			violations = append(violations, fmt.Sprintf(
				"%s:%d: generator directive has terminal punctuation",
				path,
				fileSet.Position(comment.Pos()).Line,
			))
		}
	}
	return violations
}

func checkDeclaration(fileSet *token.FileSet, path string, declaration ast.Decl) []string {
	var violations []string
	switch value := declaration.(type) {
	case *ast.FuncDecl:
		if ast.IsExported(value.Name.Name) && exportedReceiver(value) {
			violations = append(violations, checkComment(
				fileSet,
				path,
				value.Name.Name,
				value.Pos(),
				value.Doc,
				false,
			)...)
		}
	case *ast.GenDecl:
		for _, raw := range value.Specs {
			switch spec := raw.(type) {
			case *ast.TypeSpec:
				if ast.IsExported(spec.Name.Name) {
					violations = append(violations, checkComment(
						fileSet,
						path,
						spec.Name.Name,
						spec.Pos(),
						declarationComment(spec.Doc, value.Doc),
						false,
					)...)
				}
			case *ast.ValueSpec:
				for _, name := range spec.Names {
					if ast.IsExported(name.Name) {
						violations = append(violations, checkComment(
							fileSet,
							path,
							name.Name,
							spec.Pos(),
							declarationComment(spec.Doc, value.Doc),
							false,
						)...)
					}
				}
			}
		}
	}
	return violations
}

func checkComment(
	fileSet *token.FileSet,
	path string,
	name string,
	position token.Pos,
	comment *ast.CommentGroup,
	commandAllowed bool,
) []string {
	line := fileSet.Position(position).Line
	if comment == nil {
		return []string{fmt.Sprintf("%s:%d: %s has no doc comment", path, line, name)}
	}

	text := documentationText(comment)
	validPrefix := strings.HasPrefix(text, name)
	if commandAllowed {
		validPrefix = validPrefix || strings.HasPrefix(text, "Command ")
	}
	var violations []string
	if !validPrefix {
		violations = append(violations, fmt.Sprintf(
			"%s:%d: %s doc comment starts with %q",
			path,
			line,
			name,
			firstLine(text),
		))
	}
	summary, _, hasDetails := strings.Cut(text, "\n\n")
	if hasDetails && !hasTerminalPunctuation(summary) {
		violations = append(violations, fmt.Sprintf(
			"%s:%d: %s doc comment summary is not a complete sentence",
			path,
			line,
			name,
		))
	}
	if !hasTerminalPunctuation(text) {
		violations = append(violations, fmt.Sprintf(
			"%s:%d: %s doc comment is not a complete sentence",
			path,
			line,
			name,
		))
	}
	return violations
}

func documentationText(comment *ast.CommentGroup) string {
	lines := strings.Split(comment.Text(), "\n")
	documentation := lines[:0]
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "+") {
			continue
		}
		documentation = append(documentation, line)
	}
	return strings.TrimSpace(strings.Join(documentation, "\n"))
}

func declarationComment(preferred, fallback *ast.CommentGroup) *ast.CommentGroup {
	if preferred != nil {
		return preferred
	}
	return fallback
}

func exportedReceiver(function *ast.FuncDecl) bool {
	if function.Recv == nil {
		return true
	}
	receiver := function.Recv.List[0].Type
	if pointer, ok := receiver.(*ast.StarExpr); ok {
		receiver = pointer.X
	}
	if index, ok := receiver.(*ast.IndexExpr); ok {
		receiver = index.X
	}
	if indexList, ok := receiver.(*ast.IndexListExpr); ok {
		receiver = indexList.X
	}
	identifier, ok := receiver.(*ast.Ident)
	return ok && ast.IsExported(identifier.Name)
}

func firstLine(value string) string {
	line, _, _ := strings.Cut(value, "\n")
	return line
}

func hasTerminalPunctuation(value string) bool {
	var last rune
	for _, current := range strings.TrimSpace(value) {
		if !unicode.IsSpace(current) {
			last = current
		}
	}
	switch last {
	case '.', '。', '!', '！', '?', '？':
		return true
	default:
		return false
	}
}

func ignoredDirectory(name string) bool {
	return strings.HasPrefix(name, ".") ||
		name == "node_modules" ||
		name == "vendor" ||
		name == "_output"
}
