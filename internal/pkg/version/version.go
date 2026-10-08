// Package version 提供二进制的构建版本信息，不负责命令行或进程生命周期。
package version

import (
	"fmt"
	"runtime"
)

// Info 标识二进制来自哪个源码版本，以及构建时使用的工具链和平台。
type Info struct {
	GitVersion   string `json:"gitVersion"`
	GitCommit    string `json:"gitCommit"`
	GitTreeState string `json:"gitTreeState"`
	BuildDate    string `json:"buildDate"`
	GoVersion    string `json:"goVersion"`
	Compiler     string `json:"compiler"`
	Platform     string `json:"platform"`
}

// 这些字符串只在构建时通过 -ldflags -X 注入，运行时不修改。
var (
	gitVersion   = "devel"
	gitCommit    = "unknown"
	gitTreeState = "unknown"
	buildDate    = "unknown"
)

// Get 返回构建信息；直接 go build 且未注入时保留开发版本和未知字段。
func Get() Info {
	return Info{
		GitVersion:   gitVersion,
		GitCommit:    gitCommit,
		GitTreeState: gitTreeState,
		BuildDate:    buildDate,
		GoVersion:    runtime.Version(),
		Compiler:     runtime.Compiler,
		Platform:     runtime.GOOS + "/" + runtime.GOARCH,
	}
}

// String 返回版本号，不包含其他构建信息。
func (i Info) String() string {
	return i.GitVersion
}

// Text 返回按字段对齐的多行构建信息，不包含末尾换行。
func (i Info) Text() string {
	return fmt.Sprintf(
		"Version:        %s\n"+
			"Git commit:     %s\n"+
			"Git tree state: %s\n"+
			"Build date:     %s\n"+
			"Go version:     %s\n"+
			"Compiler:       %s\n"+
			"Platform:       %s",
		i.GitVersion, i.GitCommit, i.GitTreeState, i.BuildDate,
		i.GoVersion, i.Compiler, i.Platform,
	)
}
