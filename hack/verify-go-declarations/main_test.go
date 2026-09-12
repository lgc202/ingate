package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestConstructorOwner 验证构造函数名称与实际返回类型共同决定所属类型。
func TestConstructorOwner(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "exported constructor",
			source: "type Store struct{}; func NewStore() *Store { return nil }",
			want:   "Store",
		},
		{
			name:   "package constructor",
			source: "type store struct{}; func New() (*store, error) { return nil, nil }",
			want:   "store",
		},
		{
			name:   "private value constructor",
			source: "type strategy struct{}; func newStrategy() strategy { return strategy{} }",
			want:   "strategy",
		},
		{
			name:   "private mixed case type",
			source: "type statusStrategy struct{}; func newStatusStrategy() *statusStrategy { return nil }",
			want:   "statusStrategy",
		},
		{
			name:   "private initialism",
			source: "type apiClient struct{}; func newAPIClient() *apiClient { return nil }",
			want:   "apiClient",
		},
		{
			name:   "generic private constructor",
			source: "type store[T any] struct{}; func newStore[T any]() *store[T] { return nil }",
			want:   "store",
		},
		{
			name:   "generic exported constructor",
			source: "type Store[K comparable, V any] struct{}; func NewStore[K comparable, V any]() *Store[K, V] { return nil }",
			want:   "Store",
		},
		{
			name:   "test helper is not the type constructor",
			source: "type recorder struct{}; func newTestRecorder() *recorder { return nil }",
		},
		{
			name:   "qualified exported factory is not the type constructor",
			source: "type Recorder struct{}; func NewTestRecorder() *Recorder { return nil }",
		},
		{
			name:   "name must match actual return type",
			source: "type Store struct{}; type Reader struct{}; func NewStore() *Reader { return nil }",
		},
		{
			name:   "external return type",
			source: "type Store struct{}; func NewStore() *external.Store { return nil }",
		},
		{
			name:   "no return type",
			source: "type Store struct{}; func NewStore() {}",
		},
		{
			name:   "method is not a constructor",
			source: "type Store struct{}; func (Store) NewStore() *Store { return nil }",
		},
		{
			name:   "ordinary function",
			source: "type store struct{}; func loadStore() *store { return nil }",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), "example.go", "package example\n"+tt.source, 0)
			if err != nil {
				t.Fatalf("parse source %q: %v", tt.source, err)
			}
			types, _ := collectTypeDeclarations(file)
			function := file.Decls[len(file.Decls)-1].(*ast.FuncDecl)
			if got := constructorOwner(function, types); got != tt.want {
				t.Errorf("constructorOwner(%q) = %q, want %q", tt.source, got, tt.want)
			}
		})
	}
}

// TestVerifyFileConstructorOrder 验证私有构造函数的相邻规则，并保留普通辅助函数的声明顺序。
func TestVerifyFileConstructorOrder(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name: "private constructors follow their types",
			source: `package example
type strategy struct{}
func newStrategy() strategy { return strategy{} }
type statusStrategy struct{ strategy }
func newStatusStrategy() statusStrategy { return statusStrategy{} }
func (strategy) Validate() {}
`,
		},
		{
			name: "private constructor separated from type",
			source: `package example
type strategy struct{}
func (strategy) Validate() {}
func newStrategy() strategy { return strategy{} }
`,
			want: []string{
				"constructor newStrategy must immediately follow type strategy",
				"type declaration appears after exported function or method declarations",
			},
		},
		{
			name: "test helper stays with private functions",
			source: `package example
type recorder struct{}
func (recorder) Record() {}
func newTestRecorder() *recorder { return nil }
`,
		},
		{
			name: "unconstructed structs still precede constructed structs",
			source: `package example
type strategy struct{}
func newStrategy() strategy { return strategy{} }
type input struct{}
func (strategy) Validate() {}
`,
			want: []string{"struct input without a constructor must appear before structs with constructors"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "example.go")
			if err := os.WriteFile(path, []byte(tt.source), 0o600); err != nil {
				t.Fatalf("write source %q: %v", tt.source, err)
			}
			violations, err := verifyFile(path)
			if err != nil {
				t.Fatalf("verifyFile(%q): %v", tt.source, err)
			}
			got := make([]string, len(violations))
			for i, violation := range violations {
				_, got[i], _ = strings.Cut(violation, ": ")
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("verifyFile(%q) = %v, want %v", tt.source, got, tt.want)
			}
		})
	}
}
