package requestrecord

const (
	// StreamName 是 Controller 和 ALS 共享的 Envoy gRPC 访问日志流名称。
	StreamName = "ingate"
	// ContentTypeHeader 是 Kafka 请求记录的编码类型 Header。
	ContentTypeHeader = "content-type"
	// MessageTypeHeader 是 Kafka 请求记录的消息类型 Header。
	MessageTypeHeader = "message-type"
	// ContentType 是 Kafka 请求记录的 protobuf 编码类型。
	ContentType = "application/x-protobuf"
	// MessageType 是 Kafka 请求记录的 protobuf 全限定类型。
	MessageType = "ingate.als.v1.RequestRecord"
)

// MaxEncodedBytes 限制一条 RequestRecord protobuf 的最大编码长度。
//
// 请求记录不包含 Header 和 Body，64 KiB 足以容纳排障字段，并为 Kafka 默认的
// 单条消息上限保留充足余量，避免永久无法投递的记录阻塞本地队列。
const MaxEncodedBytes = 64 << 10
