// Package version 提供 Ingate 二进制的构建版本信息
package version

import (
	"fmt"
	"runtime"
	"strings"
)

var (
	// 这些变量由构建脚本通过 -ldflags 注入，本地直接 go run 时保留可识别的默认值
	gitVersion   = "v0.0.0-unknown"
	gitCommit    = "unknown"
	gitTreeState = ""
	buildDate    = "unknown"
)

// String 返回适合日志和服务元数据使用的简短版本号
func String() string {
	if gitTreeState == "dirty" {
		return gitVersion + "-dirty"
	}
	return gitVersion
}

// Text 返回适合 --version 输出的完整构建信息
func Text() string {
	rows := [][2]string{
		{"gitVersion:", gitVersion},
		{"gitCommit:", gitCommit},
	}
	if gitTreeState != "" {
		rows = append(rows, [2]string{"gitTreeState:", gitTreeState})
	}
	rows = append(rows,
		[2]string{"buildDate:", buildDate},
		[2]string{"goVersion:", runtime.Version()},
		[2]string{"compiler:", runtime.Compiler},
		[2]string{"platform:", runtime.GOOS + "/" + runtime.GOARCH},
	)

	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("%-13s %s", row[0], row[1]))
	}
	return strings.Join(lines, "\n")
}
