// Package domain 定义 Admin API 在协议和数据库表示之间使用的业务状态。
package domain

// Account 表示管理员账户及其认证状态。
type Account struct {
	ID             uint64
	Username       string
	PasswordHash   string
	SessionVersion uint64
}
