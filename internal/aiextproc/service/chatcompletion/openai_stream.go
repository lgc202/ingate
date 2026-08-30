package chatcompletion

import (
	"bytes"
	"errors"

	"github.com/tidwall/gjson"
)

// OpenAIStream 只缓存尚未结束的 SSE 行，避免把流式响应整体缓冲。
type OpenAIStream struct {
	buffer      []byte
	clientModel string
	metadata    ResponseMetadata
}

// NewOpenAIStream 创建一条 OpenAI 兼容响应流的转换状态
// clientModel 是 AI Route 对外发布的稳定模型名，响应不能泄漏 Service 使用的真实模型名。
func NewOpenAIStream(clientModel string) *OpenAIStream {
	return &OpenAIStream{clientModel: clientModel}
}

// Convert 增量读取 OpenAI SSE，提取运行信息并恢复客户端模型名
// ExtProc chunk 与 SSE 行没有边界关系，因此不完整行必须留到下一个 chunk 再处理。
func (s *OpenAIStream) Convert(chunk []byte, endOfStream bool) ([]byte, ResponseMetadata, bool, error) {
	s.buffer = append(s.buffer, chunk...)
	var converted []byte
	metadataChanged := false
	for {
		lineEnd := bytes.IndexByte(s.buffer, '\n')
		if lineEnd < 0 {
			break
		}
		line := s.buffer[:lineEnd]
		s.buffer = s.buffer[lineEnd+1:]
		output, changed, err := s.convertLine(line)
		if err != nil {
			return nil, ResponseMetadata{}, false, err
		}
		converted = append(converted, output...)
		converted = append(converted, '\n')
		metadataChanged = changed || metadataChanged
	}
	if len(s.buffer) > maxPendingSSEBytes {
		return nil, ResponseMetadata{}, false, errors.New("OpenAI stream event exceeds the size limit")
	}
	if endOfStream && len(s.buffer) > 0 {
		output, changed, err := s.convertLine(s.buffer)
		if err != nil {
			return nil, ResponseMetadata{}, false, err
		}
		converted = append(converted, output...)
		metadataChanged = changed || metadataChanged
		s.buffer = nil
	}
	return converted, s.metadata, metadataChanged, nil
}

func (s *OpenAIStream) convertLine(line []byte) ([]byte, bool, error) {
	carriageReturn := bytes.HasSuffix(line, []byte{'\r'})
	line = bytes.TrimSuffix(line, []byte{'\r'})
	if !bytes.HasPrefix(line, []byte("data:")) {
		if carriageReturn {
			line = append(line, '\r')
		}
		return line, false, nil
	}
	payload := bytes.TrimSpace(line[len("data:"):])
	if bytes.Equal(payload, []byte("[DONE]")) {
		return line, false, nil
	}
	if !gjson.ValidBytes(payload) {
		return nil, false, errors.New("OpenAI stream data must be valid JSON")
	}

	// OpenAI SSE 的 data 内容与非流式响应复用相同的 model、choices 和 usage 路径
	metadata, changed := ObserveOpenAIResponse(payload)
	if changed {
		mergeResponseMetadata(&s.metadata, metadata)
	}
	converted, bodyChanged, err := RewriteOpenAIResponseModel(payload, s.clientModel)
	if err != nil {
		return nil, false, err
	}
	if bodyChanged {
		line = append([]byte("data: "), converted...)
	}
	if carriageReturn {
		line = append(line, '\r')
	}
	return line, changed, nil
}
