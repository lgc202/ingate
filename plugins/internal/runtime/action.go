// Package runtime 提供内置插件共享的轻量运行时抽象
package runtime

// ActionKind 表示插件对当前 HTTP 请求的处理动作
type ActionKind string

const (
	ActionContinue ActionKind = "Continue"
	ActionPause    ActionKind = "Pause"
	ActionRespond  ActionKind = "Respond"
)

// Action 是插件领域结果和 Proxy-Wasm SDK 动作之间的稳定边界
type Action struct {
	Kind       ActionKind
	StatusCode int
	Headers    map[string]string
	Body       string
}

// Continue 表示继续处理当前请求
func Continue() Action {
	return Action{Kind: ActionContinue}
}

// Pause 表示暂停当前请求，等待异步 hostcall 回调
func Pause() Action {
	return Action{Kind: ActionPause}
}

// Respond 表示直接返回响应并终止当前请求
func Respond(statusCode int, headers map[string]string, body string) Action {
	return Action{
		Kind:       ActionRespond,
		StatusCode: statusCode,
		Headers:    headers,
		Body:       body,
	}
}
