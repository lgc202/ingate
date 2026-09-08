package biz

import "testing"

// TestRecorderStateWaitsForQueuedWrites 验证已准入的队列写入完成前不会恢复 Kafka 直写。
func TestRecorderStateWaitsForQueuedWrites(t *testing.T) {
	state := newRecorderState(false)
	if !state.reserveQueueWrite(false) {
		t.Fatal("recorderState.reserveQueueWrite() = false, want true")
	}

	if state.resumePublishing(true, func() bool { return true }) {
		t.Fatal("recorderState.resumePublishing() = true with a queue write in flight, want false")
	}

	state.completeQueueWrite(true)
	if !state.resumePublishing(true, func() bool { return true }) {
		t.Fatal("recorderState.resumePublishing() = false after queue drain, want true")
	}
}

// TestRecorderStateRequiresEmptyQueue 验证 Topic 合规且队列排空后才能恢复 Kafka 直写。
func TestRecorderStateRequiresEmptyQueue(t *testing.T) {
	state := newRecorderState(true)
	if state.resumePublishing(true, func() bool { return false }) {
		t.Fatal("recorderState.resumePublishing() = true with pending records, want false")
	}
	if state.resumePublishing(false, func() bool { return true }) {
		t.Fatal("recorderState.resumePublishing() = true with a noncompliant topic, want false")
	}
	if !state.resumePublishing(true, func() bool { return true }) {
		t.Fatal("recorderState.resumePublishing() = false with an empty queue and compliant topic, want true")
	}
}

// TestRecorderStateSerializesRecovery 验证恢复判断与后续写入准入使用同一线性化边界。
func TestRecorderStateSerializesRecovery(t *testing.T) {
	state := newRecorderState(true)
	checkingQueue := make(chan struct{})
	continueRecovery := make(chan struct{})
	recovered := make(chan bool, 1)

	go func() {
		recovered <- state.resumePublishing(true, func() bool {
			close(checkingQueue)
			<-continueRecovery
			return true
		})
	}()
	<-checkingQueue

	queued := make(chan bool, 1)
	go func() {
		queued <- state.reserveQueueWrite(true)
	}()

	close(continueRecovery)
	if !<-recovered {
		t.Fatal("recorderState.resumePublishing() = false, want true")
	}
	if <-queued {
		t.Fatal("recorderState.reserveQueueWrite() = true after recovery, want false")
	}
}
