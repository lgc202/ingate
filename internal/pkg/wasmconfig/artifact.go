// Package wasmconfig 定义 Wasm 制品各信任边界共享的稳定约束。
package wasmconfig

import (
	"crypto/sha256"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/uuid"
	"golang.org/x/mod/semver"
)

const (
	// MaxArtifactURLBytes 限制 Wasm 制品地址的持久化大小。
	MaxArtifactURLBytes = 4_096
	// MaxVersionBytes 限制插件语义版本的存储大小。
	MaxVersionBytes = 128
	// MaxRootIDBytes 限制 Proxy-Wasm Root Context 标识的存储大小。
	MaxRootIDBytes    = 256
	pluginIDNamespace = "https://ingate.io/wasm-plugins/"
)

// PluginID 返回插件包在一个配置域内唯一且稳定的资源 ID。
func PluginID(packageName string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(pluginIDNamespace+packageName)).String()
}

// NormalizeVersion 返回不带 v 前缀的规范插件语义版本。
func NormalizeVersion(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "v")
}

// IsValidVersion 判断 value 是否为不带 v 前缀的规范语义版本。
func IsValidVersion(value string) bool {
	return value != "" &&
		len(value) <= MaxVersionBytes &&
		value == NormalizeVersion(value) &&
		semver.IsValid("v"+value)
}

// IsValidRootID 判断 value 是否可安全写入 Proxy-Wasm Root Context 配置。
func IsValidRootID(value string) bool {
	if len(value) > MaxRootIDBytes || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

// IsValidArtifactURL 判断 value 是否为可直接拉取的 HTTP(S) 或 OCI 制品地址。
func IsValidArtifactURL(value string) bool {
	if value == "" ||
		len(value) > MaxArtifactURLBytes ||
		strings.TrimSpace(value) != value {
		return false
	}

	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return false
	}
	switch parsed.Scheme {
	case "http", "https":
		return parsed.Fragment == ""
	case "oci":
		if parsed.RawQuery != "" ||
			parsed.Fragment != "" ||
			strings.Trim(parsed.Path, "/") == "" {
			return false
		}
		_, err := name.ParseReference(
			strings.TrimPrefix(value, "oci://"),
			name.StrictValidation,
		)
		return err == nil
	default:
		return false
	}
}

// IsValidSHA256Digest 判断 value 是否为规范的小写 SHA-256 十六进制摘要。
func IsValidSHA256Digest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	for i := range value {
		character := value[i]
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				return false
			}
		}
	}
	return true
}
