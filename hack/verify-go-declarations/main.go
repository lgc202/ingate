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

// Command verify-go-declarations 检查手写 Go 文件的顶层声明组织。
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

const (
	categoryConst declarationCategory = iota
	categoryVar
	categoryType
	categoryExportedFunc
	categoryPrivateFunc
)

type declarationCategory uint8

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

			fileViolations, err := verifyFile(path)
			if err != nil {
				return err
			}
			violations = append(violations, fileViolations...)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("verify Go declarations under %q: %w", root, err)
		}
	}
	return violations, nil
}

func verifyFile(path string) ([]string, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %q: %w", path, err)
	}
	if ast.IsGenerated(file) {
		return nil, nil
	}

	violations := verifyProviderSetFile(path, fileSet, file)
	var highest declarationCategory
	hasDeclaration := false
	for _, declaration := range file.Decls {
		category, ok := classifyDeclaration(declaration)
		if !ok {
			continue
		}
		if hasDeclaration && category < highest {
			position := fileSet.Position(declaration.Pos())
			violations = append(violations, fmt.Sprintf(
				"%s:%d: %s appears after %s declarations",
				path,
				position.Line,
				declarationName(category),
				categoryName(highest),
			))
			continue
		}
		if !hasDeclaration || category > highest {
			highest = category
			hasDeclaration = true
		}
	}
	return violations, nil
}

func verifyProviderSetFile(path string, fileSet *token.FileSet, file *ast.File) []string {
	if filepath.Base(path) == file.Name.Name+".go" {
		return nil
	}
	for _, declaration := range file.Decls {
		variables, ok := declaration.(*ast.GenDecl)
		if !ok || variables.Tok != token.VAR {
			continue
		}
		for _, specification := range variables.Specs {
			value := specification.(*ast.ValueSpec)
			for _, name := range value.Names {
				if name.Name == "ProviderSet" {
					position := fileSet.Position(name.Pos())
					return []string{fmt.Sprintf(
						"%s:%d: ProviderSet must be declared in %s.go",
						path,
						position.Line,
						file.Name.Name,
					)}
				}
			}
		}
	}
	return nil
}

func declarationName(category declarationCategory) string {
	switch category {
	case categoryConst, categoryVar, categoryType:
		return categoryName(category) + " declaration"
	default:
		return categoryName(category)
	}
}

func classifyDeclaration(declaration ast.Decl) (declarationCategory, bool) {
	switch value := declaration.(type) {
	case *ast.GenDecl:
		switch value.Tok {
		case token.CONST:
			return categoryConst, true
		case token.VAR:
			return categoryVar, true
		case token.TYPE:
			return categoryType, true
		default:
			return 0, false
		}
	case *ast.FuncDecl:
		if ast.IsExported(value.Name.Name) {
			return categoryExportedFunc, true
		}
		return categoryPrivateFunc, true
	default:
		return 0, false
	}
}

func categoryName(category declarationCategory) string {
	switch category {
	case categoryConst:
		return "const"
	case categoryVar:
		return "var"
	case categoryType:
		return "type"
	case categoryExportedFunc:
		return "exported function or method"
	case categoryPrivateFunc:
		return "private function or method"
	default:
		return "unknown"
	}
}

func ignoredDirectory(name string) bool {
	return strings.HasPrefix(name, ".") ||
		name == "node_modules" ||
		name == "vendor" ||
		name == "_output"
}
