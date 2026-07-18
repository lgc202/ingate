package delivery

import (
	"context"
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/lgc202/ingate/internal/envoy/config"
	"github.com/lgc202/ingate/internal/envoy/xds"
	"google.golang.org/protobuf/proto"
)

type commandKind uint8

const (
	commandSubmit commandKind = iota + 1
	commandCancelCandidate
	commandXDSEvent
	commandACKTimeout
)

type command struct {
	kind commandKind
	ctx  context.Context

	result           config.CompileResult
	compileHasErrors bool
	event            xds.Event

	version  string
	sequence uint64

	reply chan<- error
}

type publishedConfig struct {
	version   string
	config    config.Config
	snapshot  *cachev3.Snapshot
	resources []config.ResourceGeneration
}

type candidateState struct {
	publishedConfig
	sequence      uint64
	requiredTypes []string
	timer         *time.Timer
}

type ackProgress struct {
	sent  map[string]bool
	acked map[string]bool
}

type streamState struct {
	nodeID           string
	versions         map[string]*ackProgress
	acceptedVersions map[string]string
}

type deliveryState struct {
	sequence uint64

	candidate *candidateState
	active    *publishedConfig
	streams   map[int64]*streamState

	lastFailure *Failure
}

func newDeliveryState() deliveryState {
	return deliveryState{streams: make(map[int64]*streamState)}
}

func (s *deliveryState) snapshot() Status {
	var status Status
	if s.active != nil {
		status.ActiveResources = cloneResourceGenerations(s.active.resources)
	}
	if s.lastFailure != nil {
		failure := *s.lastFailure
		failure.Resources = cloneResourceGenerations(failure.Resources)
		status.LastFailure = &failure
	}
	return status
}

func (s *deliveryState) stream(streamID int64, nodeID string) (*streamState, bool) {
	stream, ok := s.streams[streamID]
	if !ok {
		return nil, false
	}
	if nodeID != "" {
		if stream.nodeID != "" && stream.nodeID != nodeID {
			return nil, false
		}
		stream.nodeID = nodeID
	}
	return stream, true
}

func (s *deliveryState) pruneProgress(versions ...string) {
	keep := make(map[string]bool, len(versions))
	for _, version := range versions {
		if version != "" {
			keep[version] = true
		}
	}
	for _, stream := range s.streams {
		for version := range stream.versions {
			if !keep[version] {
				delete(stream.versions, version)
			}
		}
	}
}

func (s *streamState) progress(version string) *ackProgress {
	progress := s.versions[version]
	if progress != nil {
		return progress
	}
	progress = &ackProgress{
		sent:  make(map[string]bool),
		acked: make(map[string]bool),
	}
	s.versions[version] = progress
	return progress
}

func (s *streamState) fullyACKed(version string, required []string) bool {
	if s.nodeID == "" {
		return false
	}
	progress := s.versions[version]
	if progress == nil {
		return false
	}
	for _, typeURL := range required {
		if !progress.sent[typeURL] || !progress.acked[typeURL] {
			return false
		}
	}
	return true
}

func (s *streamState) fullyAccepted(version string, required []string) bool {
	if s.nodeID == "" {
		return false
	}
	for _, typeURL := range required {
		if s.acceptedVersions[typeURL] != version {
			return false
		}
	}
	return true
}

func (s *streamState) recordAccepted(typeURL, version string) {
	if s.acceptedVersions == nil {
		s.acceptedVersions = make(map[string]string)
	}
	s.acceptedVersions[typeURL] = version
}

func transitionTypeURLs(active *publishedConfig, candidate config.Config) []string {
	required := make(map[string]bool, len(dynamicTypeURLs()))
	for _, typeURL := range configTypeURLs(candidate) {
		required[typeURL] = true
	}
	if active != nil {
		for _, typeURL := range configTypeURLs(active.config) {
			required[typeURL] = true
		}
	}

	result := make([]string, 0, len(required))
	for _, typeURL := range dynamicTypeURLs() {
		if required[typeURL] {
			result = append(result, typeURL)
		}
	}
	return result
}

func configTypeURLs(value config.Config) []string {
	result := []string{resourcev3.ListenerType}
	if len(value.Routes) > 0 {
		result = append(result, resourcev3.RouteType)
	}
	if len(value.Clusters) > 0 {
		result = append(result, resourcev3.ClusterType)
	}
	if len(value.Endpoints) > 0 {
		result = append(result, resourcev3.EndpointType)
	}
	return result
}

func dynamicTypeURLs() []string {
	return []string{
		resourcev3.ListenerType,
		resourcev3.RouteType,
		resourcev3.ClusterType,
		resourcev3.EndpointType,
	}
}

func cloneConfig(value config.Config) config.Config {
	cloned := config.Config{
		Listeners: make([]*listenerv3.Listener, 0, len(value.Listeners)),
		Routes:    make([]*routev3.RouteConfiguration, 0, len(value.Routes)),
		Clusters:  make([]*clusterv3.Cluster, 0, len(value.Clusters)),
		Endpoints: make([]*endpointv3.ClusterLoadAssignment, 0, len(value.Endpoints)),
	}
	for _, listener := range value.Listeners {
		cloned.Listeners = append(cloned.Listeners, proto.CloneOf(listener))
	}
	for _, route := range value.Routes {
		cloned.Routes = append(cloned.Routes, proto.CloneOf(route))
	}
	for _, cluster := range value.Clusters {
		cloned.Clusters = append(cloned.Clusters, proto.CloneOf(cluster))
	}
	for _, endpoint := range value.Endpoints {
		cloned.Endpoints = append(cloned.Endpoints, proto.CloneOf(endpoint))
	}
	return cloned
}

func configsEqual(a, b config.Config) bool {
	if len(a.Listeners) != len(b.Listeners) || len(a.Routes) != len(b.Routes) ||
		len(a.Clusters) != len(b.Clusters) || len(a.Endpoints) != len(b.Endpoints) {
		return false
	}
	for i := range a.Listeners {
		if !proto.Equal(a.Listeners[i], b.Listeners[i]) {
			return false
		}
	}
	for i := range a.Routes {
		if !proto.Equal(a.Routes[i], b.Routes[i]) {
			return false
		}
	}
	for i := range a.Clusters {
		if !proto.Equal(a.Clusters[i], b.Clusters[i]) {
			return false
		}
	}
	for i := range a.Endpoints {
		if !proto.Equal(a.Endpoints[i], b.Endpoints[i]) {
			return false
		}
	}
	return true
}

func cloneResourceGenerations(resources []config.ResourceGeneration) []config.ResourceGeneration {
	return append([]config.ResourceGeneration(nil), resources...)
}
