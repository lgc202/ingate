package service

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	accesslogdata "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	accesslogservice "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	"github.com/lgc202/ingate/internal/als/biz"
	"github.com/lgc202/ingate/internal/als/conf"
	"github.com/lgc202/ingate/internal/als/data/diskqueue"
	alsmetrics "github.com/lgc202/ingate/internal/als/metrics"
	"github.com/lgc202/ingate/internal/pkg/requestrecord"
)

type acceptingPublisher struct{}

type accessLogStream struct {
	grpc.ServerStream
	ctx      context.Context
	message  *accesslogservice.StreamAccessLogsMessage
	received bool
}

func (acceptingPublisher) Publish(_ context.Context, records []*alsv1.RequestRecord) biz.PublishResult {
	return biz.PublishResult{Confirmed: len(records)}
}

func (s *accessLogStream) Context() context.Context {
	return s.ctx
}

func (s *accessLogStream) Recv() (*accesslogservice.StreamAccessLogsMessage, error) {
	if s.received {
		return nil, io.EOF
	}
	s.received = true
	return s.message, nil
}

func (*accessLogStream) SendAndClose(*accesslogservice.StreamAccessLogsResponse) error {
	return nil
}

// TestStreamAccessLogsClosesWhenWALIsFull 验证 WAL 拒绝批次时流以 Unavailable 结束。
func TestStreamAccessLogsClosesWhenWALIsFull(t *testing.T) {
	segmentBytes := int64(1 << 10)
	capacityBytes := segmentBytes*2 + 1
	queue, err := diskqueue.NewQueue(&conf.Data_DiskQueue{
		Path:          t.TempDir(),
		SegmentBytes:  segmentBytes,
		CapacityBytes: &capacityBytes,
		Sync:          true,
	})
	if err != nil {
		t.Fatalf("diskqueue.NewQueue() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if err := queue.Close(); err != nil {
			t.Errorf("Queue.Close() error = %v, want nil", err)
		}
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	recorder := biz.NewRecorder(
		acceptingPublisher{},
		biz.NewTopicContract(biz.ReliabilityDevelopment),
		queue,
		logger,
	)
	stream := &accessLogStream{
		ctx:     t.Context(),
		message: validAccessLogMessage(),
	}

	err = NewService(recorder, alsmetrics.NewEventCollector(), logger).StreamAccessLogs(stream)
	if code := status.Code(err); code != codes.Unavailable {
		t.Fatalf("Service.StreamAccessLogs(full WAL) code = %s, want %s", code, codes.Unavailable)
	}
	if records, bytes := queue.Pending(); records != 0 || bytes != 0 {
		t.Errorf("Queue.Pending() after rejected stream = (%d, %d), want (0, 0)", records, bytes)
	}
}

// TestStreamAccessLogsCountsRecordsBeforeIdentityValidation 验证身份错误不会漏掉协议入口计数。
func TestStreamAccessLogsCountsRecordsBeforeIdentityValidation(t *testing.T) {
	message := validAccessLogMessage()
	message.Identifier = nil
	events := alsmetrics.NewEventCollector()
	stream := &accessLogStream{ctx: t.Context(), message: message}
	service := NewService(
		newServiceRecorder(t),
		events,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	if code := status.Code(service.StreamAccessLogs(stream)); code != codes.InvalidArgument {
		t.Fatalf("Service.StreamAccessLogs(invalid identity) code = %s, want %s", code, codes.InvalidArgument)
	}

	err := testutil.CollectAndCompare(
		events,
		strings.NewReader(`# HELP ingate_als_records_received_total Request records received at the ALS protocol boundary.
# TYPE ingate_als_records_received_total counter
ingate_als_records_received_total 1
`),
		"ingate_als_records_received_total",
	)
	if err != nil {
		t.Fatalf("CollectAndCompare() error = %v, want nil", err)
	}
}

func newServiceRecorder(t *testing.T) *biz.Recorder {
	t.Helper()

	capacityBytes := int64(4 << 10)
	queue, err := diskqueue.NewQueue(&conf.Data_DiskQueue{
		Path:          t.TempDir(),
		SegmentBytes:  1 << 10,
		CapacityBytes: &capacityBytes,
		Sync:          true,
	})
	if err != nil {
		t.Fatalf("diskqueue.NewQueue() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if err := queue.Close(); err != nil {
			t.Errorf("Queue.Close() error = %v, want nil", err)
		}
	})

	topic := biz.NewTopicContract(biz.ReliabilityDevelopment)
	topic.Update(biz.TopicTopology{Exists: true, ReplicationFactor: 1, MinInSyncReplicas: 1})
	return biz.NewRecorder(
		acceptingPublisher{},
		topic,
		queue,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
}

func validAccessLogMessage() *accesslogservice.StreamAccessLogsMessage {
	entry := &accesslogdata.HTTPAccessLogEntry{
		CommonProperties: &accesslogdata.AccessLogCommon{
			StartTime: timestamppb.Now(),
			StreamId:  "stream-1",
		},
		ProtocolVersion: accesslogdata.HTTPAccessLogEntry_HTTP11,
		Request: &accesslogdata.HTTPRequestProperties{
			RequestMethod: corev3.RequestMethod_GET,
			Authority:     "example.com",
			Path:          "/health",
			RequestId:     "request-1",
		},
		Response: &accesslogdata.HTTPResponseProperties{
			ResponseCode: wrapperspb.UInt32(200),
		},
	}
	return &accesslogservice.StreamAccessLogsMessage{
		Identifier: &accesslogservice.StreamAccessLogsMessage_Identifier{
			Node:    &corev3.Node{Id: "envoy-1"},
			LogName: requestrecord.StreamName,
		},
		LogEntries: &accesslogservice.StreamAccessLogsMessage_HttpLogs{
			HttpLogs: &accesslogservice.StreamAccessLogsMessage_HTTPAccessLogEntries{
				LogEntry: []*accesslogdata.HTTPAccessLogEntry{entry},
			},
		},
	}
}
