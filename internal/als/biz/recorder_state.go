package biz

import "sync"

// recorderState 串行化 Kafka 直写与磁盘队列降级之间的状态迁移。
// spooling 和 activeQueueWrites 必须在同一把锁内判断和修改，
// 否则回放线程可能在已获准的队列写入真正落盘前恢复 Kafka 直写。
type recorderState struct {
	mu sync.Mutex

	spooling          bool
	activeQueueWrites int
	kafkaOK           bool
	queueOK           bool
}

func newRecorderState(hasPending bool) *recorderState {
	return &recorderState{
		spooling: hasPending,
		queueOK:  true,
	}
}

// reserveQueueWrite 在当前写入必须进入磁盘队列时登记一个写入者。
// 返回 true 后，调用方必须调用 completeQueueWrite 完成配对。
func (s *recorderState) reserveQueueWrite(topicCompliant bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.spooling && topicCompliant {
		return false
	}

	s.spooling = true
	s.activeQueueWrites++
	return true
}

func (s *recorderState) publishSucceeded() {
	s.mu.Lock()
	s.kafkaOK = true
	s.mu.Unlock()
}

// reserveFallbackWrite 将 Kafka 失败的批次切换到磁盘队列并登记一个写入者。
// 返回 true 表示本次调用使 Recorder 首次进入降级状态。
func (s *recorderState) reserveFallbackWrite() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.kafkaOK = false
	switched := !s.spooling
	s.spooling = true
	s.activeQueueWrites++

	return switched
}

// completeQueueWrite 结束一个已登记的磁盘队列写入。
// 返回 true 表示队列可写状态发生变化。
func (s *recorderState) completeQueueWrite(succeeded bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.activeQueueWrites--
	changed := s.queueOK != succeeded
	s.queueOK = succeeded

	return changed
}

func (s *recorderState) replayFailed() {
	s.mu.Lock()
	s.kafkaOK = false
	s.spooling = true
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

// resumePublishing 在状态锁内确认队列仍为空，使得这次检查与恢复直写线性化。
// 已经获得队列准入的写入会计入 activeQueueWrites，因此不会在落盘前被跳过。
func (s *recorderState) resumePublishing(topicCompliant bool, queueEmpty func() bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.spooling || !topicCompliant || s.activeQueueWrites > 0 || !queueEmpty() {
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
