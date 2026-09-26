package domain

// Session 表示登录态绑定的账户和密码版本。
type Session struct {
	AccountID      uint64
	AccountVersion uint64
}
