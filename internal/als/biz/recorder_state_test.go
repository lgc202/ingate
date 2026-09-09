package biz

import "testing"

// TestRecorderStateWaitsForInFlightWrites 验证所有故障屏障前后的在途写入完成后才能恢复直写。
func TestRecorderStateWaitsForInFlightWrites(t *testing.T) {
	state := newRecorderState(false)
	if target := state.reserveWriteTarget(true); target != kafkaTarget {
		t.Fatalf("recorderState.reserveWriteTarget() = %v, want Kafka", target)
	}
	if !state.failKafkaWrite() {
		t.Fatal("recorderState.failKafkaWrite(first failure) = false, want true")
	}
	if target := state.reserveWriteTarget(true); target != queueTarget {
		t.Fatalf("recorderState.reserveWriteTarget() after failure = %v, want queue", target)
	}

	if state.resumeKafkaWrites(true, func() bool { return true }) {
		t.Fatal("recorderState.resumeKafkaWrites() = true with queue writes in flight, want false")
	}

	state.finishQueueWrite(nil)
	state.finishQueueWrite(nil)
	if !state.resumeKafkaWrites(true, func() bool { return true }) {
		t.Fatal("recorderState.resumeKafkaWrites() = false after queue drain, want true")
	}
}

// TestRecorderStateCompletesPreBarrierWrites 验证故障屏障前已准入的 Kafka 写入按各自结果完成。
func TestRecorderStateCompletesPreBarrierWrites(t *testing.T) {
	state := newRecorderState(false)
	for i := range 2 {
		if target := state.reserveWriteTarget(true); target != kafkaTarget {
			t.Fatalf("recorderState.reserveWriteTarget() call %d = %v, want Kafka", i+1, target)
		}
	}

	if !state.failKafkaWrite() {
		t.Fatal("recorderState.failKafkaWrite(first failure) = false, want true")
	}
	if state.failKafkaWrite() {
		t.Fatal("recorderState.failKafkaWrite(second failure) = true, want false")
	}
	if target := state.reserveWriteTarget(true); target != queueTarget {
		t.Fatalf("recorderState.reserveWriteTarget() beyond barrier = %v, want queue", target)
	}

	state.finishQueueWrite(nil)
	state.finishQueueWrite(nil)
	state.finishQueueWrite(nil)
	if !state.resumeKafkaWrites(true, func() bool { return true }) {
		t.Fatal("recorderState.resumeKafkaWrites() = false after all pre-barrier writes completed")
	}
}

// TestRecorderStateRequiresRecoveryConditions 验证 Topic、空队列和永久错误共同约束恢复直写。
func TestRecorderStateRequiresRecoveryConditions(t *testing.T) {
	state := newRecorderState(true)
	if state.resumeKafkaWrites(true, func() bool { return false }) {
		t.Fatal("recorderState.resumeKafkaWrites() = true with pending records, want false")
	}
	if state.resumeKafkaWrites(false, func() bool { return true }) {
		t.Fatal("recorderState.resumeKafkaWrites() = true with a noncompliant topic, want false")
	}

	state.replayFailed(PublishPermanent)
	if state.resumeKafkaWrites(true, func() bool { return true }) {
		t.Fatal("recorderState.resumeKafkaWrites() = true after permanent replay failure, want false")
	}
}

// TestRecorderStateSerializesRecovery 验证恢复判断与后续写入准入使用同一线性化边界。
func TestRecorderStateSerializesRecovery(t *testing.T) {
	state := newRecorderState(true)
	checkingQueue := make(chan struct{})
	continueRecovery := make(chan struct{})
	recovered := make(chan bool, 1)

	go func() {
		recovered <- state.resumeKafkaWrites(true, func() bool {
			close(checkingQueue)
			<-continueRecovery
			return true
		})
	}()
	<-checkingQueue

	target := make(chan writeTarget, 1)
	go func() {
		target <- state.reserveWriteTarget(true)
	}()

	close(continueRecovery)
	if !<-recovered {
		t.Fatal("recorderState.resumeKafkaWrites() = false, want true")
	}
	if got := <-target; got != kafkaTarget {
		t.Fatalf("recorderState.reserveWriteTarget() after recovery = %v, want Kafka", got)
	}
	state.completeKafkaWrite()
}
