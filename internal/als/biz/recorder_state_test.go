package biz

import "testing"

// TestRecorderStateWaitsForInFlightWrites 验证所有故障屏障前后的在途写入完成后才能恢复直写。
func TestRecorderStateWaitsForInFlightWrites(t *testing.T) {
	state := newRecorderState(false)
	if target := state.reserveWrite(true); target != kafkaTarget {
		t.Fatalf("recorderState.reserveWrite() = %v, want Kafka", target)
	}
	if !state.finishKafkaWrite(false) {
		t.Fatal("recorderState.finishKafkaWrite(first failure) = false, want true")
	}
	if target := state.reserveWrite(true); target != queueTarget {
		t.Fatalf("recorderState.reserveWrite() after failure = %v, want queue", target)
	}

	if state.resumePublishing(true, func() bool { return true }) {
		t.Fatal("recorderState.resumePublishing() = true with queue writes in flight, want false")
	}

	state.finishQueueWrite(true)
	state.finishQueueWrite(true)
	if !state.resumePublishing(true, func() bool { return true }) {
		t.Fatal("recorderState.resumePublishing() = false after queue drain, want true")
	}
}

// TestRecorderStateCompletesPreBarrierWrites 验证故障屏障前已准入的 Kafka 写入按各自结果完成。
func TestRecorderStateCompletesPreBarrierWrites(t *testing.T) {
	state := newRecorderState(false)
	for i := range 2 {
		if target := state.reserveWrite(true); target != kafkaTarget {
			t.Fatalf("recorderState.reserveWrite() call %d = %v, want Kafka", i+1, target)
		}
	}

	if !state.finishKafkaWrite(false) {
		t.Fatal("recorderState.finishKafkaWrite(first failure) = false, want true")
	}
	if state.finishKafkaWrite(false) {
		t.Fatal("recorderState.finishKafkaWrite(second failure) = true, want false")
	}
	if target := state.reserveWrite(true); target != queueTarget {
		t.Fatalf("recorderState.reserveWrite() beyond barrier = %v, want queue", target)
	}

	state.finishQueueWrite(true)
	state.finishQueueWrite(true)
	state.finishQueueWrite(true)
	if !state.resumePublishing(true, func() bool { return true }) {
		t.Fatal("recorderState.resumePublishing() = false after all pre-barrier writes completed")
	}
}

// TestRecorderStateRequiresRecoveryConditions 验证 Topic、空队列和永久错误共同约束恢复直写。
func TestRecorderStateRequiresRecoveryConditions(t *testing.T) {
	state := newRecorderState(true)
	if state.resumePublishing(true, func() bool { return false }) {
		t.Fatal("recorderState.resumePublishing() = true with pending records, want false")
	}
	if state.resumePublishing(false, func() bool { return true }) {
		t.Fatal("recorderState.resumePublishing() = true with a noncompliant topic, want false")
	}

	state.replayFailed(true)
	if state.resumePublishing(true, func() bool { return true }) {
		t.Fatal("recorderState.resumePublishing() = true after permanent replay failure, want false")
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

	target := make(chan writeTarget, 1)
	go func() {
		target <- state.reserveWrite(true)
	}()

	close(continueRecovery)
	if !<-recovered {
		t.Fatal("recorderState.resumePublishing() = false, want true")
	}
	if got := <-target; got != kafkaTarget {
		t.Fatalf("recorderState.reserveWrite() after recovery = %v, want Kafka", got)
	}
	state.finishKafkaWrite(true)
}
