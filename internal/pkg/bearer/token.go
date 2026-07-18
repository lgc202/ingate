// Package bearer 提供 Authorization Bearer Token 的共享校验
package bearer

// ValidToken 返回 value 是否符合 RFC 6750 b64token 字符范围
func ValidToken(value string) bool {
	if value == "" {
		return false
	}

	padding := false
	tokenChars := 0
	for i := 0; i < len(value); i++ {
		char := value[i]
		if char == '=' {
			if tokenChars == 0 {
				return false
			}
			padding = true
			continue
		}
		if padding || !isTokenChar(char) {
			return false
		}
		tokenChars++
	}
	return tokenChars > 0
}

func isTokenChar(char byte) bool {
	return char >= 'a' && char <= 'z' ||
		char >= 'A' && char <= 'Z' ||
		char >= '0' && char <= '9' ||
		char == '-' || char == '.' || char == '_' || char == '~' || char == '+' || char == '/'
}
