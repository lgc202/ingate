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

type declarationCategory uint8

const (
	categoryConst declarationCategory = iota
	categoryVar
	categoryType
	categoryExportedFunc
	categoryPrivateFunc
)

type constructorDeclaration struct {
	name  string
	index int
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
	typeDeclarations, structDeclarations := collectTypeDeclarations(file)
	constructors := collectConstructors(file, structDeclarations)
	violations = append(violations, verifyAttachedDeclarations(
		path,
		fileSet,
		file,
		typeDeclarations,
		structDeclarations,
		constructors,
	)...)

	var highest declarationCategory
	hasDeclaration := false
	for _, declaration := range file.Decls {
		category, ok := classifyDeclaration(declaration, typeDeclarations, structDeclarations)
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

func collectTypeDeclarations(file *ast.File) (map[string]int, map[string]int) {
	types := make(map[string]int)
	structs := make(map[string]int)
	for index, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.TYPE {
			continue
		}
		for _, specification := range general.Specs {
			typeSpecification := specification.(*ast.TypeSpec)
			types[typeSpecification.Name.Name] = index
			if _, ok := typeSpecification.Type.(*ast.StructType); ok {
				structs[typeSpecification.Name.Name] = index
			}
		}
	}
	return types, structs
}

func collectConstructors(file *ast.File, structs map[string]int) map[string]constructorDeclaration {
	constructors := make(map[string]constructorDeclaration)
	for index, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv != nil || !strings.HasPrefix(function.Name.Name, "New") {
			continue
		}
		owner := constructorOwner(function, structs)
		if owner != "" {
			constructors[owner] = constructorDeclaration{
				name:  function.Name.Name,
				index: index,
			}
		}
	}
	return constructors
}

func verifyAttachedDeclarations(
	path string,
	fileSet *token.FileSet,
	file *ast.File,
	types map[string]int,
	structs map[string]int,
	constructors map[string]constructorDeclaration,
) []string {
	var violations []string
	for index, declaration := range file.Decls {
		owner := constantOwner(declaration)
		if owner == "" {
			continue
		}
		typeIndex, ok := types[owner]
		if !ok {
			continue
		}
		if index == typeIndex+1 {
			continue
		}
		position := fileSet.Position(declaration.Pos())
		violations = append(violations, fmt.Sprintf(
			"%s:%d: const declarations for %s must immediately follow its type declaration",
			path,
			position.Line,
			owner,
		))
	}

	constructedStructSeen := false
	for index, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.TYPE {
			continue
		}
		for _, specification := range general.Specs {
			typeSpecification := specification.(*ast.TypeSpec)
			structIndex, isStruct := structs[typeSpecification.Name.Name]
			if !isStruct || structIndex != index {
				continue
			}
			constructor, hasConstructor := constructors[typeSpecification.Name.Name]
			if !hasConstructor {
				if constructedStructSeen {
					position := fileSet.Position(typeSpecification.Pos())
					violations = append(violations, fmt.Sprintf(
						"%s:%d: struct %s without a constructor must appear before structs with constructors",
						path,
						position.Line,
						typeSpecification.Name.Name,
					))
				}
				continue
			}

			constructedStructSeen = true
			if constructor.index == index+1 {
				continue
			}
			position := fileSet.Position(file.Decls[constructor.index].Pos())
			violations = append(violations, fmt.Sprintf(
				"%s:%d: constructor %s must immediately follow struct %s",
				path,
				position.Line,
				constructor.name,
				typeSpecification.Name.Name,
			))
		}
	}
	return violations
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

func classifyDeclaration(
	declaration ast.Decl,
	types map[string]int,
	structs map[string]int,
) (declarationCategory, bool) {
	switch value := declaration.(type) {
	case *ast.GenDecl:
		switch value.Tok {
		case token.CONST:
			if _, ok := types[constantOwner(value)]; ok {
				return categoryType, true
			}
			return categoryConst, true
		case token.VAR:
			return categoryVar, true
		case token.TYPE:
			return categoryType, true
		default:
			return 0, false
		}
	case *ast.FuncDecl:
		if value.Recv == nil && constructorOwner(value, structs) != "" {
			return categoryType, true
		}
		if ast.IsExported(value.Name.Name) {
			return categoryExportedFunc, true
		}
		return categoryPrivateFunc, true
	default:
		return 0, false
	}
}

func constructorOwner(function *ast.FuncDecl, structs map[string]int) string {
	if function.Name.Name != "New" {
		name := strings.TrimPrefix(function.Name.Name, "New")
		if _, ok := structs[name]; ok {
			return name
		}
		return ""
	}
	if function.Type.Results == nil {
		return ""
	}
	for _, result := range function.Type.Results.List {
		resultType := result.Type
		if pointer, ok := resultType.(*ast.StarExpr); ok {
			resultType = pointer.X
		}
		identifier, ok := resultType.(*ast.Ident)
		if !ok {
			continue
		}
		if _, ok := structs[identifier.Name]; ok {
			return identifier.Name
		}
	}
	return ""
}

func constantOwner(declaration ast.Decl) string {
	general, ok := declaration.(*ast.GenDecl)
	if !ok || general.Tok != token.CONST {
		return ""
	}

	var owner string
	for _, specification := range general.Specs {
		value := specification.(*ast.ValueSpec)
		if value.Type != nil {
			identifier, ok := value.Type.(*ast.Ident)
			if !ok || owner != "" && owner != identifier.Name {
				return ""
			}
			owner = identifier.Name
			continue
		}
		if len(value.Values) == 0 {
			if owner == "" {
				return ""
			}
			continue
		}
		for _, expression := range value.Values {
			convertedOwner := constantConversionOwner(expression)
			if convertedOwner == "" || owner != "" && owner != convertedOwner {
				return ""
			}
			owner = convertedOwner
		}
	}
	return owner
}

func constantConversionOwner(expression ast.Expr) string {
	conversion, ok := expression.(*ast.CallExpr)
	if !ok || len(conversion.Args) != 1 {
		return ""
	}
	identifier, ok := conversion.Fun.(*ast.Ident)
	if !ok {
		return ""
	}
	return identifier.Name
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
