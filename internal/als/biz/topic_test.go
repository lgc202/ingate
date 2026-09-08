package biz

import "testing"

// TestTopicContract 验证开发和生产模式使用各自的 Topic 副本约束。
func TestTopicContract(t *testing.T) {
	tests := []struct {
		name      string
		mode      ReliabilityMode
		topology  TopicTopology
		compliant bool
	}{
		{
			name: "single broker in development",
			mode: ReliabilityDevelopment,
			topology: TopicTopology{
				Exists:            true,
				ReplicationFactor: 1,
				MinInSyncReplicas: 1,
			},
			compliant: true,
		},
		{
			name: "single broker in production",
			mode: ReliabilityProduction,
			topology: TopicTopology{
				Exists:            true,
				ReplicationFactor: 1,
				MinInSyncReplicas: 1,
			},
		},
		{
			name: "minimum ISR exceeds replicas",
			mode: ReliabilityDevelopment,
			topology: TopicTopology{
				Exists:            true,
				ReplicationFactor: 1,
				MinInSyncReplicas: 2,
			},
		},
		{
			name: "replicated topic in production",
			mode: ReliabilityProduction,
			topology: TopicTopology{
				Exists:            true,
				ReplicationFactor: 3,
				MinInSyncReplicas: 2,
			},
			compliant: true,
		},
		{
			name: "missing topic",
			mode: ReliabilityDevelopment,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			contract := NewTopicContract(test.mode)
			status := contract.Update(test.topology)

			if !status.Checked || status.Compliant != test.compliant {
				t.Errorf("TopicContract.Update() = %+v, want checked and compliant = %t", status, test.compliant)
			}
			if cached := contract.Status(); cached != status {
				t.Errorf("TopicContract.Status() = %+v, want %+v", cached, status)
			}
		})
	}
}
