package delivery

const (
	// EventStreamOpened 表示 SotW stream 已建立。
	EventStreamOpened XDSEventKind = "StreamOpened"
	// EventStreamClosed 表示 SotW stream 已关闭。
	EventStreamClosed XDSEventKind = "StreamClosed"
	// EventResponseSent 表示 SotW response 已交给底层 stream 发送。
	EventResponseSent XDSEventKind = "ResponseSent"
	// EventAcceptedVersionObserved 表示 Envoy 主动报告某类配置当前已接受的版本。
	EventAcceptedVersionObserved XDSEventKind = "AcceptedVersionObserved"
	// EventACK 表示 Envoy 接受了对应 nonce 的配置版本。
	EventACK XDSEventKind = "ACK"
	// EventNACK 表示 Envoy 拒绝了对应 nonce 的配置版本。
	EventNACK XDSEventKind = "NACK"
)

// XDSEventKind 表示 xDS stream 生命周期和响应确认事件类型。
type XDSEventKind string

// XDSEvent 是 xDS 层交给 Delivery 的协议事件。
type XDSEvent struct {
	Kind            XDSEventKind
	StreamID        int64
	NodeID          string
	TypeURL         string
	Version         string
	AcceptedVersion string
}
