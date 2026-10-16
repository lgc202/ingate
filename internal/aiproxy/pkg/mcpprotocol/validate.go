// Package mcpprotocol 校验首版支持的 MCP Streamable HTTP（2026-07-28）请求信封。
// 网关只校验 JSON-RPC 信封与标准镜像 Header 一致性；
// 不解释方法参数语义、不维护工具 schema、不重建或改写正文，未知 Mcp-Param-* 原样转发。
// 依据：https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http。
package mcpprotocol

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	strictjson "sigs.k8s.io/json"
)

// Version 是首版唯一支持的 MCP 协议版本；声明其他版本或缺失版本头按官方规范拒绝。
const Version = "2026-07-28"

// 官方规范定义的 JSON-RPC 与 MCP 错误码。
const (
	// CodeParseError 表示正文不是合法 JSON。
	CodeParseError = -32700
	// CodeInvalidRequest 表示正文不是合法的 JSON-RPC 请求或通知。
	CodeInvalidRequest = -32600
	// CodeInternalError 表示网关无法完成请求，不公开内部失败原因。
	CodeInternalError = -32603
	// CodeHeaderMismatch 表示镜像 Header 缺失、畸形或与正文不一致。
	CodeHeaderMismatch = -32020
	// CodeUnsupportedProtocolVersion 表示请求声明的协议版本不在支持范围。
	CodeUnsupportedProtocolVersion = -32022
)

const (
	protocolVersionHeader = "MCP-Protocol-Version"
	methodHeader          = "Mcp-Method"
	nameHeader            = "Mcp-Name"
	base64Prefix          = "=?base64?"
	base64Suffix          = "?="
)

// Error 是一次网关校验产生的 JSON-RPC 错误内容，不携带内部实现细节。
// ID 是请求 id 的原样副本；通知或无法解析时为空，写出为 null。
type Error struct {
	Code    int
	Message string
	ID      json.RawMessage
	// Data 是错误附加数据，例如版本拒绝的 supported/requested。
	Data map[string]any
}

// parsedMessage 是一次已解析的 JSON-RPC 信封，只保留校验所需字段。
type parsedMessage struct {
	id          json.RawMessage
	method      string
	isRequest   bool
	bodyVersion string
	paramsName  json.RawMessage
	paramsURI   json.RawMessage
}

// Error 只公开可安全返回的消息，不暴露内部原因。
func (e *Error) Error() string {
	return fmt.Sprintf("MCP protocol error %d: %s", e.Code, e.Message)
}

// WriteError 按 JSON-RPC 2.0 写回一条错误响应，并设置给定的 HTTP 状态。
func WriteError(w http.ResponseWriter, status, code int, message string, id json.RawMessage, data map[string]any) {
	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage("null"),
		"error":   map[string]any{"code": code, "message": message},
	}
	if len(id) != 0 {
		body["id"] = id
	}

	if len(data) != 0 {
		body["error"].(map[string]any)["data"] = data
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// 写出失败时响应已开始，不能再追加业务错误；编码本身只包含可编码字段。
	_ = json.NewEncoder(w).Encode(body)
}

// Validate 校验现代请求的 JSON-RPC 结构与镜像 Header 一致性。
// 版本头必须存在且为 Version，并与正文 params._meta 的协议版本一致；请求还必须带
// 与 method 一致的 Mcp-Method 和适用方法的 Mcp-Name。通知的 Header 要求本修订未定义，
// 只校验版本后原样转发。返回 *Error 时调用方按 JSON-RPC 错误写回。
func Validate(header http.Header, body []byte) error {
	parsed, err := parseMessage(body)
	if err != nil {
		return err
	}

	version, err := singleHeader(header, protocolVersionHeader)
	if err != nil {
		return headerMismatch("invalid MCP-Protocol-Version header", parsed.id)
	}

	if version == "" {
		return headerMismatch("missing MCP-Protocol-Version header", parsed.id)
	}

	if _, err := time.Parse(time.DateOnly, version); err != nil {
		return headerMismatch("invalid MCP-Protocol-Version header", parsed.id)
	}

	if version != Version {
		return &Error{
			Code:    CodeUnsupportedProtocolVersion,
			Message: "Unsupported protocol version",
			ID:      parsed.id,
			Data:    map[string]any{"supported": []string{Version}, "requested": version},
		}
	}

	if parsed.isRequest && parsed.bodyVersion != version {
		return headerMismatch("MCP-Protocol-Version header does not match body protocol version", parsed.id)
	}

	if !parsed.isRequest {
		return nil
	}

	method, err := singleHeader(header, methodHeader)
	if err != nil {
		return headerMismatch("invalid Mcp-Method header", parsed.id)
	}

	if method == "" {
		return headerMismatch("missing Mcp-Method header", parsed.id)
	}

	if method != parsed.method {
		return headerMismatch("Mcp-Method header does not match body method", parsed.id)
	}

	expectedField, required := messageName(parsed)
	if !required {
		return nil
	}

	raw, err := singleHeader(header, nameHeader)
	if err != nil {
		return headerMismatch("invalid Mcp-Name header", parsed.id)
	}

	if raw == "" {
		return headerMismatch("missing Mcp-Name header", parsed.id)
	}

	name, ok := decodeHeaderValue(raw)
	if !ok {
		return headerMismatch("invalid Mcp-Name header encoding", parsed.id)
	}

	var expectedName *string
	if err := json.Unmarshal(expectedField, &expectedName); err != nil || expectedName == nil {
		return headerMismatch("missing or invalid body name", parsed.id)
	}

	if name != *expectedName {
		return headerMismatch("Mcp-Name header does not match body name", parsed.id)
	}

	return nil
}

// parseMessage 复用项目已有的严格 JSON 解码器，在一次解码中检查信封字段和重复成员。
// 未知业务字段仍由后端解释；id 保留原始字节，不经过浮点转换。
func parseMessage(body []byte) (parsedMessage, error) {
	if !utf8.Valid(body) {
		return parsedMessage{}, &Error{Code: CodeParseError, Message: "invalid UTF-8 JSON payload"}
	}

	var envelope struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
		Params  *struct {
			Name json.RawMessage `json:"name"`
			URI  json.RawMessage `json:"uri"`
			Meta struct {
				Version string `json:"io.modelcontextprotocol/protocolVersion"`
			} `json:"_meta"`
		} `json:"params"`
	}
	strictErrors, err := strictjson.UnmarshalStrict(body, &envelope, strictjson.DisallowDuplicateFields)
	if err != nil {
		code := CodeInvalidRequest
		if ok, _ := strictjson.SyntaxErrorOffset(err); ok {
			code = CodeParseError
		}

		return parsedMessage{}, &Error{Code: code, Message: "invalid JSON-RPC message"}
	}

	if len(strictErrors) != 0 || envelope.JSONRPC != "2.0" || envelope.Method == "" || len(envelope.Result) != 0 || len(envelope.Error) != 0 {
		return parsedMessage{}, &Error{Code: CodeInvalidRequest, Message: "invalid JSON-RPC request or notification"}
	}

	id := envelope.ID
	if len(id) != 0 && id[0] != '"' {
		// MCP 的数字 id 必须是整数；不接受 null、布尔、对象、数组或小数。
		number := strings.TrimPrefix(string(id), "-")
		if number == "" || strings.IndexFunc(number, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return parsedMessage{}, &Error{Code: CodeInvalidRequest, Message: "invalid JSON-RPC request id"}
		}
	}

	parsed := parsedMessage{id: id, method: envelope.Method, isRequest: len(id) != 0}
	if envelope.Params != nil {
		parsed.paramsName = envelope.Params.Name
		parsed.paramsURI = envelope.Params.URI
		parsed.bodyVersion = envelope.Params.Meta.Version
	}

	return parsed, nil
}

// messageName 返回适用方法要求与 Mcp-Name 一致的正文值；其他方法不要求该 Header。
func messageName(parsed parsedMessage) (json.RawMessage, bool) {
	switch parsed.method {
	case "tools/call", "prompts/get":
		return parsed.paramsName, true
	case "resources/read":
		return parsed.paramsURI, true
	default:
		return nil, false
	}
}

// singleHeader 读取单个 Header 值；缺失返回空字符串，重复或空白值返回错误。
func singleHeader(header http.Header, name string) (string, error) {
	values := header.Values(name)
	if len(values) == 0 {
		return "", nil
	}

	if len(values) != 1 {
		return "", errors.New("duplicate header")
	}

	value := values[0]
	if value == "" || value != strings.TrimSpace(value) {
		return "", errors.New("empty header")
	}

	for _, r := range value {
		if r != '\t' && (r < 0x20 || r > 0x7e) {
			return "", errors.New("invalid header characters")
		}
	}

	return value, nil
}

// decodeHeaderValue 解码 Base64 sentinel；普通值原样返回，非法编码返回 false。
func decodeHeaderValue(value string) (string, bool) {
	if !strings.HasPrefix(value, base64Prefix) || !strings.HasSuffix(value, base64Suffix) {
		return value, true
	}

	encoded, ok := strings.CutSuffix(strings.TrimPrefix(value, base64Prefix), base64Suffix)
	if !ok {
		return "", false
	}

	decoded, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || !utf8.Valid(decoded) {
		return "", false
	}

	return string(decoded), true
}

func headerMismatch(message string, id json.RawMessage) *Error {
	return &Error{Code: CodeHeaderMismatch, Message: message, ID: id}
}
