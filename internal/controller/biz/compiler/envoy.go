package compiler

import (
	"cmp"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"slices"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"google.golang.org/protobuf/proto"
)

const versionPrefix = "ingate/"

type versionRecord struct {
	typeURL string
	name    string
	message proto.Message
}

func (c EnvoyConfig) validate() error {
	for _, listener := range c.Listeners {
		if err := listener.ValidateAll(); err != nil {
			return fmt.Errorf("validate listener %q: %w", listener.GetName(), err)
		}
	}
	for _, route := range c.Routes {
		if err := route.ValidateAll(); err != nil {
			return fmt.Errorf("validate route %q: %w", route.GetName(), err)
		}
	}
	for _, cluster := range c.Clusters {
		if err := cluster.ValidateAll(); err != nil {
			return fmt.Errorf("validate cluster %q: %w", cluster.GetName(), err)
		}
	}
	for _, endpoint := range c.Endpoints {
		if err := endpoint.ValidateAll(); err != nil {
			return fmt.Errorf("validate endpoint %q: %w", endpoint.GetClusterName(), err)
		}
	}
	return nil
}

func (c EnvoyConfig) version() (string, error) {
	// 版本只由排序后的资源类型、名字和确定性 protobuf bytes 决定，不依赖输入集合顺序
	records := make([]versionRecord, 0, len(c.Listeners)+len(c.Routes)+len(c.Clusters)+len(c.Endpoints))
	for _, listener := range c.Listeners {
		records = append(records, versionRecord{
			typeURL: resourcev3.ListenerType,
			name:    listener.GetName(),
			message: listener,
		})
	}
	for _, route := range c.Routes {
		records = append(records, versionRecord{
			typeURL: resourcev3.RouteType,
			name:    route.GetName(),
			message: route,
		})
	}
	for _, cluster := range c.Clusters {
		records = append(records, versionRecord{
			typeURL: resourcev3.ClusterType,
			name:    cluster.GetName(),
			message: cluster,
		})
	}
	for _, endpoint := range c.Endpoints {
		records = append(records, versionRecord{
			typeURL: resourcev3.EndpointType,
			name:    endpoint.GetClusterName(),
			message: endpoint,
		})
	}
	slices.SortFunc(records, func(a, b versionRecord) int {
		if a.typeURL != b.typeURL {
			return cmp.Compare(a.typeURL, b.typeURL)
		}
		return cmp.Compare(a.name, b.name)
	})

	data := make([]byte, 0)
	marshal := proto.MarshalOptions{Deterministic: true}
	for _, record := range records {
		payload, err := marshal.Marshal(record.message)
		if err != nil {
			return "", fmt.Errorf("marshal %s %q: %w", record.typeURL, record.name, err)
		}
		data = appendVersionField(data, []byte(record.typeURL))
		data = appendVersionField(data, []byte(record.name))
		data = appendVersionField(data, payload)
	}

	sum := sha256.Sum256(data)
	return versionPrefix + hex.EncodeToString(sum[:]), nil
}

func appendVersionField(data, field []byte) []byte {
	data = binary.AppendUvarint(data, uint64(len(field)))
	return append(data, field...)
}

func adsConfigSource() *corev3.ConfigSource {
	return &corev3.ConfigSource{
		ConfigSourceSpecifier: &corev3.ConfigSource_Ads{
			Ads: &corev3.AggregatedConfigSource{},
		},
		ResourceApiVersion: corev3.ApiVersion_V3,
	}
}
