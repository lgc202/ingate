// Package httpheader 提供 HTTP Header 的通用校验能力
package httpheader

// ValidValue 判断值是否可以安全写入单个 HTTP Header
func ValidValue(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < 0x20 || value[i] == 0x7f {
			return false
		}
	}
	return true
}
