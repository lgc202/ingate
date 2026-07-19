package sse

import (
	"bytes"
	"errors"
	"strconv"
)

// MaxBufferedBytes 是 Decoder 为尚未完成的 SSE 单行和事件保留的最大字节数
const MaxBufferedBytes = 1 << 20

var (
	// ErrClosed 表示 Decoder 已完成，不能继续写入数据
	ErrClosed = errors.New("SSE decoder is closed")
	// ErrBufferLimit 表示尚未完成的 SSE 单行或事件超过缓冲上限
	ErrBufferLimit = errors.New("SSE decoder buffer limit exceeded")
)

// Event 表示一个完成的 SSE 事件
type Event struct {
	Type  string
	Data  []byte
	ID    string
	Retry *int
}

// Decoder 按任意网络分块增量解析 SSE，不受 bufio.Scanner 64 KiB 限制，但限制单个未完成事件的缓冲大小
type Decoder struct {
	buffer  []byte
	event   Event
	data    []byte
	lastID  string
	hasData bool
	closed  bool
}

// Push 写入一个网络分块并返回其中已经完整结束的事件
func (d *Decoder) Push(chunk []byte) ([]Event, error) {
	if d.closed {
		return nil, ErrClosed
	}
	if len(chunk) == 0 {
		return nil, nil
	}

	var events []Event
	for len(chunk) > 0 {
		available := MaxBufferedBytes - d.bufferedBytes()
		if available <= 0 {
			d.reset()
			d.closed = true
			return nil, ErrBufferLimit
		}
		count := min(len(chunk), available)
		d.buffer = append(d.buffer, chunk[:count]...)
		chunk = chunk[count:]

		completed, err := d.consume(false)
		if err != nil {
			return nil, err
		}
		events = append(events, completed...)
	}
	return events, nil
}

// Finish 完成解析，并把没有以空行结尾的最后一个事件交给调用方
func (d *Decoder) Finish() ([]Event, error) {
	if d.closed {
		return nil, ErrClosed
	}
	d.closed = true
	return d.consume(true)
}

// EncodeData 把一个数据载荷编码成标准 SSE data 事件
func EncodeData(data []byte) []byte {
	lines := bytes.Split(data, []byte{'\n'})
	var output bytes.Buffer
	for _, line := range lines {
		output.WriteString("data: ")
		output.Write(line)
		output.WriteByte('\n')
	}
	output.WriteByte('\n')
	return output.Bytes()
}

func (d *Decoder) consume(final bool) ([]Event, error) {
	var events []Event
	for {
		line, ok := d.nextLine(final)
		if !ok {
			break
		}

		event, err := d.consumeLine(line)
		if err != nil {
			return nil, err
		}
		if event != nil {
			events = append(events, *event)
		}
	}

	if final && d.hasData {
		events = append(events, d.finishEvent())
	}
	return events, nil
}

func (d *Decoder) nextLine(final bool) ([]byte, bool) {
	for i, value := range d.buffer {
		switch value {
		case '\n':
			line := append([]byte(nil), d.buffer[:i]...)
			d.discard(i + 1)
			return line, true
		case '\r':
			if i+1 == len(d.buffer) && !final {
				return nil, false
			}
			line := append([]byte(nil), d.buffer[:i]...)
			consumed := i + 1
			if consumed < len(d.buffer) && d.buffer[consumed] == '\n' {
				consumed++
			}
			d.discard(consumed)
			return line, true
		}
	}

	if final && len(d.buffer) > 0 {
		line := append([]byte(nil), d.buffer...)
		d.buffer = nil
		return line, true
	}
	return nil, false
}

func (d *Decoder) consumeLine(line []byte) (*Event, error) {
	if len(line) == 0 {
		if !d.hasData {
			d.event = Event{}
			return nil, nil
		}
		event := d.finishEvent()
		return &event, nil
	}
	if line[0] == ':' {
		return nil, nil
	}

	field, value, found := bytes.Cut(line, []byte{':'})
	if !found {
		value = nil
	} else if len(value) > 0 && value[0] == ' ' {
		value = value[1:]
	}

	switch string(field) {
	case "event":
		d.event.Type = string(value)
	case "data":
		if d.hasData {
			d.data = append(d.data, '\n')
		}
		d.data = append(d.data, value...)
		d.hasData = true
	case "id":
		if !bytes.ContainsRune(value, 0) {
			d.lastID = string(value)
		}
	case "retry":
		retry, err := strconv.Atoi(string(value))
		if err == nil && retry >= 0 {
			d.event.Retry = &retry
		}
	}
	return nil, nil
}

func (d *Decoder) finishEvent() Event {
	d.event.Data = d.data
	d.event.ID = d.lastID
	event := d.event
	d.event = Event{}
	d.data = nil
	d.hasData = false
	return event
}

func (d *Decoder) bufferedBytes() int {
	return len(d.buffer) + len(d.data) + len(d.event.Type) + len(d.lastID)
}

func (d *Decoder) reset() {
	d.buffer = nil
	d.event = Event{}
	d.data = nil
	d.lastID = ""
	d.hasData = false
}

func (d *Decoder) discard(count int) {
	d.buffer = d.buffer[count:]
	if len(d.buffer) == 0 {
		d.buffer = nil
	}
}
