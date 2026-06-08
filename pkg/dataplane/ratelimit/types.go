package ratelimit

// Algorithm 表示数据面限流算法
type Algorithm string

const (
	// AlgorithmFixedWindow 表示固定窗口限流
	AlgorithmFixedWindow Algorithm = "FixedWindow"
	// AlgorithmSlidingWindow 表示滑动窗口限流
	AlgorithmSlidingWindow Algorithm = "SlidingWindow"
	// AlgorithmTokenBucket 表示令牌桶限流
	AlgorithmTokenBucket Algorithm = "TokenBucket"
)

// RedisMode 表示 Redis 部署模式
type RedisMode string

const (
	// RedisModeStandalone 表示单实例 Redis
	RedisModeStandalone RedisMode = "Standalone"
	// RedisModeSentinel 表示 Redis Sentinel
	RedisModeSentinel RedisMode = "Sentinel"
	// RedisModeCluster 表示 Redis Cluster
	RedisModeCluster RedisMode = "Cluster"
)

// ErrorCode 表示单条限流检查失败时的稳定错误类别
type ErrorCode string

const (
	// ErrorCodeInvalidRequest 表示插件发出的单条检查请求缺少必要字段或参数非法
	ErrorCodeInvalidRequest ErrorCode = "InvalidRequest"
	// ErrorCodeUnsupportedAlgorithm 表示 dataplane 不支持请求指定的限流算法
	ErrorCodeUnsupportedAlgorithm ErrorCode = "UnsupportedAlgorithm"
	// ErrorCodeTimeout 表示 Redis 或外部执行超时
	ErrorCodeTimeout ErrorCode = "Timeout"
	// ErrorCodeRedisError 表示 Redis 连接或脚本执行失败
	ErrorCodeRedisError ErrorCode = "RedisError"
)

// CheckRequest 表示一次请求需要执行的一组限流检查
type CheckRequest struct {
	Checks []Check `json:"checks"`
}

// Check 表示一条需要由数据面服务完成的全局限流检查
//
// dataplane 会保持输入和输出一一对应：只要 HTTP 请求成功解析，单条检查失败也会返回
// 对应 Result，并通过 ErrorCode / Error 表达失败原因。插件随后根据自己的
// failurePolicy 决定 fail-open 或 fail-close。
type Check struct {
	PolicyName    string     `json:"policyName"`
	RuleName      string     `json:"ruleName"`
	RedisKey      string     `json:"redisKey"`
	RedisStore    RedisStore `json:"redisStore"`
	Algorithm     Algorithm  `json:"algorithm"`
	Limit         Limit      `json:"limit"`
	TimeoutMillis int        `json:"timeoutMillis,omitempty"`
}

// RedisStore 表示数据面服务连接 Redis 所需的运行时配置
//
// Password 表示已解析后的运行时密码。PasswordRef 预留给后续 secret resolver；
// 当前 dataplane 不会自行根据 PasswordRef 读取控制面 secret。
type RedisStore struct {
	ID                   string    `json:"id"`
	Mode                 RedisMode `json:"mode"`
	Address              string    `json:"address,omitempty"`
	Addresses            []string  `json:"addresses,omitempty"`
	DB                   int       `json:"db,omitempty"`
	TLS                  bool      `json:"tls,omitempty"`
	TLSServerName        string    `json:"tlsServerName,omitempty"`
	Username             string    `json:"username,omitempty"`
	Password             string    `json:"password,omitempty"`
	PasswordRef          string    `json:"passwordRef,omitempty"`
	ConnectTimeoutMillis int       `json:"connectTimeoutMillis,omitempty"`
	CommandTimeoutMillis int       `json:"commandTimeoutMillis,omitempty"`
	PoolSize             int       `json:"poolSize,omitempty"`
	MinIdleConns         int       `json:"minIdleConns,omitempty"`
	SentinelMaster       string    `json:"sentinelMaster,omitempty"`
}

// Limit 表示限流算法输入参数
type Limit struct {
	Requests      int `json:"requests"`
	WindowSeconds int `json:"windowSeconds"`
	Burst         int `json:"burst,omitempty"`
}

// CheckResponse 表示数据面服务返回的一组限流检查结果
type CheckResponse struct {
	Results []Result `json:"results"`
}

// Result 表示单条限流检查结果
//
// ErrorCode 为空表示检查已完成；Error 是面向日志和诊断的补充信息，插件不应依赖
// Error 文本做稳定分支。
type Result struct {
	PolicyName        string    `json:"policyName"`
	RuleName          string    `json:"ruleName"`
	Allowed           bool      `json:"allowed"`
	Current           int       `json:"current"`
	Limit             int       `json:"limit"`
	Remaining         int       `json:"remaining"`
	ResetSeconds      int       `json:"resetSeconds"`
	RetryAfterSeconds int       `json:"retryAfterSeconds,omitempty"`
	ErrorCode         ErrorCode `json:"errorCode,omitempty"`
	Error             string    `json:"error,omitempty"`
}
