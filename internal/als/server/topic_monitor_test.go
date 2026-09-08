package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/lgc202/ingate/internal/als/biz"
	"github.com/lgc202/ingate/internal/als/conf"
)

type stubTopicInspector struct {
	topology biz.TopicTopology
	err      error
}

func (i *stubTopicInspector) InspectTopic(context.Context) (biz.TopicTopology, error) {
	return i.topology, i.err
}

// TestTopicMonitorChecksBeforeStart 验证 HTTP 和 gRPC 服务启动前已缓存 Topic 契约状态。
func TestTopicMonitorChecksBeforeStart(t *testing.T) {
	inspector := &stubTopicInspector{topology: biz.TopicTopology{
		Exists:            true,
		ReplicationFactor: 3,
		MinInSyncReplicas: 2,
	}}
	contract := biz.NewTopicContract(biz.ReliabilityProduction)
	monitor := NewTopicMonitor(topicMonitorConfig(), inspector, contract, discardServerLogger())

	if err := monitor.BeforeStart(t.Context()); err != nil {
		t.Fatalf("TopicMonitor.BeforeStart() error = %v, want nil", err)
	}
	if status := contract.Status(); !status.Checked || !status.Compliant {
		t.Errorf("TopicContract.Status() = %+v, want checked and compliant", status)
	}
}

// TestTopicMonitorPreservesLastStatus 验证瞬时检查失败不会覆盖最近一次可确定的 Topic 状态。
func TestTopicMonitorPreservesLastStatus(t *testing.T) {
	contract := biz.NewTopicContract(biz.ReliabilityDevelopment)
	want := contract.Update(biz.TopicTopology{
		Exists:            true,
		ReplicationFactor: 1,
		MinInSyncReplicas: 1,
	})
	inspector := &stubTopicInspector{err: errors.New("topic inspection unavailable")}
	monitor := NewTopicMonitor(topicMonitorConfig(), inspector, contract, discardServerLogger())

	if err := monitor.BeforeStart(t.Context()); err != nil {
		t.Fatalf("TopicMonitor.BeforeStart() error = %v, want nil", err)
	}
	if got := contract.Status(); got != want {
		t.Errorf("TopicContract.Status() = %+v after inspection failure, want %+v", got, want)
	}
}

func topicMonitorConfig() *conf.Data_Kafka {
	return &conf.Data_Kafka{TopicCheckTimeout: durationpb.New(time.Second)}
}

func discardServerLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
