package redis

import (
	"errors"
	"fmt"
	"strconv"
)

// Kind 表示 RESP2 返回值类型
type Kind uint8

const (
	// KindSimpleString 表示 RESP simple string
	KindSimpleString Kind = iota + 1
	// KindBulkString 表示 RESP bulk string
	KindBulkString
	// KindInteger 表示 RESP integer
	KindInteger
	// KindArray 表示 RESP array
	KindArray
	// KindNull 表示 RESP nil bulk string 或 nil array
	KindNull
)

// Value 表示一个已经完成边界校验的 RESP2 值
type Value struct {
	Kind    Kind
	Bytes   []byte
	Integer int64
	Values  []Value
}

// RedisError 表示 Redis 返回的 error reply
type RedisError struct {
	Message string
}

// Error 实现 error
func (e *RedisError) Error() string {
	return "redis error: " + e.Message
}

// EncodeCommand 将命令参数编码为 RESP2 bulk-string array
func EncodeCommand(parts ...[]byte) ([]byte, error) {
	if len(parts) == 0 {
		return nil, errors.New("redis command is empty")
	}

	result := make([]byte, 0, 32)
	result = append(result, '*')
	result = strconv.AppendInt(result, int64(len(parts)), 10)
	result = append(result, '\r', '\n')
	for _, part := range parts {
		result = append(result, '$')
		result = strconv.AppendInt(result, int64(len(part)), 10)
		result = append(result, '\r', '\n')
		result = append(result, part...)
		result = append(result, '\r', '\n')
	}
	return result, nil
}

// Decode 解析一个完整 RESP2 值并拒绝尾随数据
func Decode(data []byte) (Value, error) {
	value, next, err := decodeValue(data, 0)
	if err != nil {
		return Value{}, err
	}
	if next != len(data) {
		return Value{}, fmt.Errorf("redis response has %d trailing bytes", len(data)-next)
	}
	return value, nil
}

// DecodeIntegers 解析 Lua 脚本返回的整数数组
func DecodeIntegers(data []byte) ([]int64, error) {
	value, err := Decode(data)
	if err != nil {
		return nil, err
	}
	if value.Kind != KindArray {
		return nil, fmt.Errorf("redis script response kind is %d, want array", value.Kind)
	}

	values := make([]int64, 0, len(value.Values))
	for i, item := range value.Values {
		switch item.Kind {
		case KindInteger:
			values = append(values, item.Integer)
		case KindBulkString, KindSimpleString:
			value, err := strconv.ParseInt(string(item.Bytes), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parse redis script value %d: %w", i, err)
			}
			values = append(values, value)
		default:
			return nil, fmt.Errorf("redis script value %d has kind %d", i, item.Kind)
		}
	}
	return values, nil
}

func decodeValue(data []byte, offset int) (Value, int, error) {
	if offset >= len(data) {
		return Value{}, offset, errors.New("redis response is empty or truncated")
	}

	switch data[offset] {
	case '+':
		line, next, err := readLine(data, offset+1)
		if err != nil {
			return Value{}, offset, err
		}
		return Value{Kind: KindSimpleString, Bytes: append([]byte(nil), line...)}, next, nil
	case '-':
		line, _, err := readLine(data, offset+1)
		if err != nil {
			return Value{}, offset, err
		}
		return Value{}, offset, &RedisError{Message: string(line)}
	case ':':
		line, next, err := readLine(data, offset+1)
		if err != nil {
			return Value{}, offset, err
		}
		value, err := strconv.ParseInt(string(line), 10, 64)
		if err != nil {
			return Value{}, offset, fmt.Errorf("parse redis integer: %w", err)
		}
		return Value{Kind: KindInteger, Integer: value}, next, nil
	case '$':
		return decodeBulkString(data, offset)
	case '*':
		return decodeArray(data, offset)
	default:
		return Value{}, offset, fmt.Errorf("unsupported redis response prefix %q", data[offset])
	}
}

func decodeBulkString(data []byte, offset int) (Value, int, error) {
	line, next, err := readLine(data, offset+1)
	if err != nil {
		return Value{}, offset, err
	}
	size, err := strconv.ParseInt(string(line), 10, 32)
	if err != nil {
		return Value{}, offset, fmt.Errorf("parse redis bulk string size: %w", err)
	}
	if size == -1 {
		return Value{Kind: KindNull}, next, nil
	}
	if size < 0 {
		return Value{}, offset, fmt.Errorf("redis bulk string size is %d", size)
	}
	end := next + int(size)
	if end < next || end+2 > len(data) {
		return Value{}, offset, errors.New("redis bulk string is truncated")
	}
	if data[end] != '\r' || data[end+1] != '\n' {
		return Value{}, offset, errors.New("redis bulk string has invalid terminator")
	}
	return Value{Kind: KindBulkString, Bytes: append([]byte(nil), data[next:end]...)}, end + 2, nil
}

func decodeArray(data []byte, offset int) (Value, int, error) {
	line, next, err := readLine(data, offset+1)
	if err != nil {
		return Value{}, offset, err
	}
	size, err := strconv.ParseInt(string(line), 10, 32)
	if err != nil {
		return Value{}, offset, fmt.Errorf("parse redis array size: %w", err)
	}
	if size == -1 {
		return Value{Kind: KindNull}, next, nil
	}
	if size < 0 {
		return Value{}, offset, fmt.Errorf("redis array size is %d", size)
	}

	values := make([]Value, 0, int(size))
	for range size {
		value, after, err := decodeValue(data, next)
		if err != nil {
			return Value{}, offset, err
		}
		values = append(values, value)
		next = after
	}
	return Value{Kind: KindArray, Values: values}, next, nil
}

func readLine(data []byte, offset int) ([]byte, int, error) {
	for i := offset; i+1 < len(data); i++ {
		if data[i] == '\r' && data[i+1] == '\n' {
			return data[offset:i], i + 2, nil
		}
	}
	return nil, offset, errors.New("redis response line is truncated")
}
