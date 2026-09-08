package kafka

import (
	"testing"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kmsg"
)

// TestMinimumReplicationFactor 验证 Topic 契约采用所有分区中的最弱副本配置。
func TestMinimumReplicationFactor(t *testing.T) {
	partitions := []kmsg.MetadataResponseTopicPartition{
		{Replicas: []int32{1, 2, 3}},
		{Replicas: []int32{2, 3}},
	}

	factor, err := minimumReplicationFactor(partitions)
	if err != nil {
		t.Fatalf("minimumReplicationFactor() error = %v, want nil", err)
	}
	if factor != 2 {
		t.Errorf("minimumReplicationFactor() = %d, want 2", factor)
	}
}

// TestMinimumReplicationFactorRejectsIncompleteMetadata 验证缺失分区或分区错误不会被误判为有效拓扑。
func TestMinimumReplicationFactorRejectsIncompleteMetadata(t *testing.T) {
	tests := []struct {
		name       string
		partitions []kmsg.MetadataResponseTopicPartition
	}{
		{name: "no partitions"},
		{
			name: "partition error",
			partitions: []kmsg.MetadataResponseTopicPartition{{
				ErrorCode: kerr.LeaderNotAvailable.Code,
			}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := minimumReplicationFactor(test.partitions); err == nil {
				t.Fatal("minimumReplicationFactor() error = nil, want non-nil")
			}
		})
	}
}
