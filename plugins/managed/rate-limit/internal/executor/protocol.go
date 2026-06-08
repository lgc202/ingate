// Package executor 定义 Wasm 插件调用内置限流执行器的 JSON 协议
package executor

const SchemaVersionV1 = "v1"

// CheckRequest 表示一次请求需要执行的一组全局限流检查
type CheckRequest struct {
	SchemaVersion string  `json:"schemaVersion"`
	Checks        []Check `json:"checks"`
}

// Check 表示一条需要由执行器完成的 Redis 限流检查
type Check struct {
	PolicyName    string     `json:"policyName"`
	RuleName      string     `json:"ruleName"`
	RedisKey      string     `json:"redisKey"`
	RedisStore    RedisStore `json:"redisStore"`
	Algorithm     string     `json:"algorithm"`
	Limit         Limit      `json:"limit"`
	TimeoutMillis int        `json:"timeoutMillis,omitempty"`
}

// RedisStore 表示执行器连接 Redis 所需的运行时配置
type RedisStore struct {
	ID                   string   `json:"id"`
	Mode                 string   `json:"mode"`
	Address              string   `json:"address,omitempty"`
	Addresses            []string `json:"addresses,omitempty"`
	DB                   int      `json:"db,omitempty"`
	TLS                  bool     `json:"tls,omitempty"`
	TLSServerName        string   `json:"tlsServerName,omitempty"`
	Username             string   `json:"username,omitempty"`
	PasswordRef          string   `json:"passwordRef,omitempty"`
	ConnectTimeoutMillis int      `json:"connectTimeoutMillis,omitempty"`
	CommandTimeoutMillis int      `json:"commandTimeoutMillis,omitempty"`
	PoolSize             int      `json:"poolSize,omitempty"`
	MinIdleConns         int      `json:"minIdleConns,omitempty"`
	SentinelMaster       string   `json:"sentinelMaster,omitempty"`
}

// Limit 表示限流算法参数
type Limit struct {
	Requests      int `json:"requests"`
	WindowSeconds int `json:"windowSeconds"`
	Burst         int `json:"burst,omitempty"`
}

// CheckResponse 表示执行器返回的一组限流检查结果
type CheckResponse struct {
	SchemaVersion string   `json:"schemaVersion"`
	Results       []Result `json:"results"`
}

// Result 表示单条限流检查结果
type Result struct {
	PolicyName        string `json:"policyName"`
	RuleName          string `json:"ruleName"`
	Allowed           bool   `json:"allowed"`
	Current           int    `json:"current"`
	Limit             int    `json:"limit"`
	Remaining         int    `json:"remaining"`
	ResetSeconds      int    `json:"resetSeconds"`
	RetryAfterSeconds int    `json:"retryAfterSeconds,omitempty"`
	Error             string `json:"error,omitempty"`
}
