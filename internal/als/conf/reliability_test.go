package conf

import (
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
)

// TestValidateDataReliabilityMode 验证开发和生产模式对本地队列的不同约束。
func TestValidateDataReliabilityMode(t *testing.T) {
	tests := []struct {
		name      string
		mode      Data_ReliabilityMode
		queueSync bool
		wantErr   bool
	}{
		{name: "unspecified mode", wantErr: true},
		{name: "development without sync", mode: Data_DEVELOPMENT},
		{name: "production with sync", mode: Data_PRODUCTION, queueSync: true},
		{name: "production without sync", mode: Data_PRODUCTION, wantErr: true},
		{name: "unknown mode", mode: Data_ReliabilityMode(9), queueSync: true, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := validDataConfig()
			config.ReliabilityMode = test.mode
			config.DiskQueue.Sync = test.queueSync
			err := validateData(config)
			if got := err != nil; got != test.wantErr {
				t.Errorf("validateData(%s) error = %v, want error presence = %t", test.name, err, test.wantErr)
			}
		})
	}
}

// TestValidateDataTopicCheckTimeout 验证 Topic 检查必须有明确的超时边界。
func TestValidateDataTopicCheckTimeout(t *testing.T) {
	config := validDataConfig()
	config.Kafka.TopicCheckTimeout = nil
	if err := validateData(config); err == nil {
		t.Error("validateData(config without topic check timeout) error = nil, want non-nil")
	}
}

// TestValidateDiskQueueReplayBackoff 验证回放退避必须是有效且有序的时间边界。
func TestValidateDiskQueueReplayBackoff(t *testing.T) {
	tests := []struct {
		name string
		min  *durationpb.Duration
		max  *durationpb.Duration
	}{
		{name: "missing minimum", max: durationpb.New(time.Second)},
		{name: "zero minimum", min: durationpb.New(0), max: durationpb.New(time.Second)},
		{name: "missing maximum", min: durationpb.New(time.Second)},
		{name: "maximum below minimum", min: durationpb.New(2 * time.Second), max: durationpb.New(time.Second)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := validDataConfig()
			config.DiskQueue.ReplayMinBackoff = test.min
			config.DiskQueue.ReplayMaxBackoff = test.max
			if err := validateData(config); err == nil {
				t.Error("validateData(config with invalid replay backoff) error = nil, want non-nil")
			}
		})
	}
}

func validDataConfig() *Data {
	return &Data{
		Kafka: &Data_Kafka{
			Brokers:           []string{"kafka:9092"},
			Topic:             "request-records",
			WriteTimeout:      durationpb.New(5 * time.Second),
			DialTimeout:       durationpb.New(3 * time.Second),
			TopicCheckTimeout: durationpb.New(5 * time.Second),
		},
		DiskQueue: &Data_DiskQueue{
			Path:             "/tmp/ingate-als-test",
			SegmentBytes:     1024,
			ReplayBatchSize:  10,
			ReplayMinBackoff: durationpb.New(time.Second),
			ReplayMaxBackoff: durationpb.New(30 * time.Second),
			MaxBytes:         2048,
		},
	}
}
