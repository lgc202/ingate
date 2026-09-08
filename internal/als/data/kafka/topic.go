package kafka

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kmsg"

	"github.com/lgc202/ingate/internal/als/biz"
)

const minInSyncReplicasConfig = "min.insync.replicas"

// InspectTopic 读取目标 Topic 的最小副本数和 min.insync.replicas。
//
// 请求显式关闭自动创建；Topic 不存在是可判定的拓扑事实，
// 网络、权限和协议错误则返回 error，由调用方决定是否保留旧状态。
func (c *Client) InspectTopic(ctx context.Context) (biz.TopicTopology, error) {
	request := kmsg.NewPtrMetadataRequest()
	request.Topics = []kmsg.MetadataRequestTopic{{Topic: new(c.topic)}}
	request.AllowAutoTopicCreation = false

	response, err := request.RequestWith(ctx, c.kafka)
	if err != nil {
		return biz.TopicTopology{}, fmt.Errorf("describe Kafka topic metadata: %w", err)
	}
	if len(response.Topics) != 1 {
		return biz.TopicTopology{}, fmt.Errorf("describe Kafka topic metadata: got %d topics", len(response.Topics))
	}

	topic := response.Topics[0]
	if topic.Topic == nil || *topic.Topic != c.topic {
		return biz.TopicTopology{}, errors.New("describe Kafka topic metadata: response topic does not match request")
	}
	if topic.ErrorCode != 0 {
		topicErr := kerr.ErrorForCode(topic.ErrorCode)
		if errors.Is(topicErr, kerr.UnknownTopicOrPartition) {
			return biz.TopicTopology{}, nil
		}
		return biz.TopicTopology{}, fmt.Errorf("describe Kafka topic metadata: %w", topicErr)
	}

	replicationFactor, err := minimumReplicationFactor(topic.Partitions)
	if err != nil {
		return biz.TopicTopology{}, err
	}
	minInSyncReplicas, err := c.inspectMinInSyncReplicas(ctx)
	if err != nil {
		return biz.TopicTopology{}, err
	}

	return biz.TopicTopology{
		Exists:            true,
		ReplicationFactor: replicationFactor,
		MinInSyncReplicas: minInSyncReplicas,
	}, nil
}

func (c *Client) inspectMinInSyncReplicas(ctx context.Context) (int, error) {
	request := kmsg.NewPtrDescribeConfigsRequest()
	request.Resources = []kmsg.DescribeConfigsRequestResource{{
		ResourceType: kmsg.ConfigResourceTypeTopic,
		ResourceName: c.topic,
		ConfigNames:  []string{minInSyncReplicasConfig},
	}}

	response, err := request.RequestWith(ctx, c.kafka)
	if err != nil {
		return 0, fmt.Errorf("describe Kafka topic config: %w", err)
	}
	if len(response.Resources) != 1 {
		return 0, fmt.Errorf("describe Kafka topic config: got %d resources", len(response.Resources))
	}

	resource := response.Resources[0]
	if resource.ResourceType != kmsg.ConfigResourceTypeTopic || resource.ResourceName != c.topic {
		return 0, errors.New("describe Kafka topic config: response resource does not match request")
	}
	if resource.ErrorCode != 0 {
		return 0, fmt.Errorf("describe Kafka topic config: %w", kerr.ErrorForCode(resource.ErrorCode))
	}

	for _, config := range resource.Configs {
		if config.Name != minInSyncReplicasConfig || config.Value == nil {
			continue
		}
		value, err := strconv.Atoi(*config.Value)
		if err != nil {
			return 0, fmt.Errorf("parse Kafka topic %s value %q: %w", minInSyncReplicasConfig, *config.Value, err)
		}
		if value <= 0 {
			return 0, fmt.Errorf("kafka topic %s must be greater than zero", minInSyncReplicasConfig)
		}
		return value, nil
	}

	return 0, fmt.Errorf("kafka topic config %q is missing", minInSyncReplicasConfig)
}

func minimumReplicationFactor(partitions []kmsg.MetadataResponseTopicPartition) (int, error) {
	if len(partitions) == 0 {
		return 0, errors.New("kafka topic has no partitions")
	}

	minimum := len(partitions[0].Replicas)
	for _, partition := range partitions {
		if partition.ErrorCode != 0 {
			return 0, fmt.Errorf("describe Kafka topic partition: %w", kerr.ErrorForCode(partition.ErrorCode))
		}
		minimum = min(minimum, len(partition.Replicas))
	}

	return minimum, nil
}
