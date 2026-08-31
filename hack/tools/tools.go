//go:build tools

// Package tools 固定只在代码生成阶段使用的 Go 工具依赖。
package tools

import _ "k8s.io/code-generator"
