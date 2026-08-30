package requestrecord

// MaxEncodedBytes 限制一条 RequestRecord protobuf 的最大编码长度。
//
// 请求记录不包含 Header 和 Body，64 KiB 足以容纳排障字段，并为 Kafka 默认的
// 单条消息上限保留充足余量，避免永久无法投递的记录阻塞本地队列。
const MaxEncodedBytes = 64 << 10
