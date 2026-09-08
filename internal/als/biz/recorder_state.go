package biz

import "sync"

type writeTarget uint8

const (
	kafkaTarget writeTarget = iota + 1
	queueTarget
)

// recorderState 串行化 Kafka 直写与磁盘队列降级之间的状态迁移。
// 状态锁只保护准入决定和在途计数，不跨越 Kafka 或磁盘 I/O。
type recorderState struct {
	mu sync.Mutex

	// spooling 是一道故障屏障；建立后的新批次只能写入磁盘队列。
	spooling bool
	// kafkaWrites 记录故障屏障前已准入、尚未完成的 Kafka 写入。
	kafkaWrites int
	// queueWrites 记录已准入、尚未完成追加的磁盘队列写入。
	queueWrites int
	// replayPaused 防止永久错误的队首在本进程内无限重试。
	replayPaused bool
	kafkaOK      bool
	queueOK      bool
}

func newRecorderState(hasPending bool) *recorderState {
	return &recorderState{
		spooling: hasPending,
		queueOK:  true,
	}
}

// reserveWrite 在线性化边界内选择写入目标并登记对应的在途操作。
func (s *recorderState) reserveWrite(topicCompliant bool) writeTarget {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.spooling || !topicCompliant {
		s.spooling = true
		s.queueWrites++
		return queueTarget
	}

	s.kafkaWrites++
	return kafkaTarget
}

// finishKafkaWrite 完成一个 Kafka 直写；失败时原子转移为磁盘队列写入。
// 返回 true 表示本次失败建立了新的故障屏障。
func (s *recorderState) finishKafkaWrite(succeeded bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.kafkaWrites--
	if succeeded {
		if !s.spooling {
			s.kafkaOK = true
		}
		return false
	}

	s.kafkaOK = false
	switched := !s.spooling
	s.spooling = true
	s.queueWrites++
	return switched
}

// finishQueueWrite 结束一个已登记的磁盘队列写入。
// 返回 true 表示队列可写状态发生变化。
func (s *recorderState) finishQueueWrite(succeeded bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.queueWrites--
	changed := s.queueOK != succeeded
	s.queueOK = succeeded
	return changed
}

func (s *recorderState) replaySucceeded() {
	s.mu.Lock()
	s.kafkaOK = true
	s.mu.Unlock()
}

func (s *recorderState) replayFailed(permanent bool) {
	s.mu.Lock()
	s.kafkaOK = false
	s.spooling = true
	s.replayPaused = s.replayPaused || permanent
	s.mu.Unlock()
}

func (s *recorderState) queueFailed() {
	s.mu.Lock()
	s.queueOK = false
	s.mu.Unlock()
}

func (s *recorderState) queueSucceeded() {
	s.mu.Lock()
	s.queueOK = true
	s.mu.Unlock()
}

func (s *recorderState) pausePublishing() {
	s.mu.Lock()
	s.spooling = true
	s.mu.Unlock()
}

// resumePublishing 在状态锁内确认恢复条件，使队列排空与后续写入准入线性化。
// Pending 在 Queue 中读取原子快照，不会在状态锁内执行磁盘 I/O。
func (s *recorderState) resumePublishing(topicCompliant bool, queueEmpty func() bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.spooling || !topicCompliant || s.kafkaWrites > 0 || s.queueWrites > 0 ||
		s.replayPaused || !queueEmpty() {
		return false
	}

	s.spooling = false
	return true
}

func (s *recorderState) status(
	topic TopicStatus,
	pendingRecords int64,
	pendingBytes int64,
) RecorderStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	return RecorderStatus{
		Topic:          topic,
		KafkaWritable:  topic.Compliant && s.kafkaOK,
		QueueWritable:  s.queueOK,
		Spooling:       !topic.Compliant || s.spooling,
		PendingRecords: pendingRecords,
		PendingBytes:   pendingBytes,
	}
}
