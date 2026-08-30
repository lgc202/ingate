// Package controlplaneauth 定义控制面内部认证凭据的稳定约束。
package controlplaneauth

const (
	// MinBearerTokenBytes 是控制面 Bearer Token 的最小长度。
	MinBearerTokenBytes = 32
	// MaxBearerTokenBytes 是控制面 Bearer Token 的最大长度。
	MaxBearerTokenBytes = 256
)

// IsValidBearerToken 判断 value 是否符合 RFC 6750 b64token 字符集和长度约束。
func IsValidBearerToken(value string) bool {
	if len(value) < MinBearerTokenBytes || len(value) > MaxBearerTokenBytes {
		return false
	}
	padding := false
	for i := range value {
		character := value[i]
		if character == '=' {
			if i == 0 {
				return false
			}
			padding = true
			continue
		}
		if padding || !isBearerTokenCharacter(character) {
			return false
		}
	}
	return true
}

func isBearerTokenCharacter(character byte) bool {
	switch {
	case character >= 'a' && character <= 'z':
		return true
	case character >= 'A' && character <= 'Z':
		return true
	case character >= '0' && character <= '9':
		return true
	case character == '-',
		character == '.',
		character == '_',
		character == '~',
		character == '+',
		character == '/':
		return true
	default:
		return false
	}
}
