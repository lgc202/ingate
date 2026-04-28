// Package runtime 定义编译结果交给运行时 target 的数据结构
package runtime

// RuntimeSnapshot 表示一个运行时 target 可消费的编译配置快照
type RuntimeSnapshot struct {
	Target  string `json:"target"`
	Gateway string `json:"gateway"`
	Version string `json:"version"`
	Config  any    `json:"config"`
}
