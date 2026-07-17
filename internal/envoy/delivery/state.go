package delivery

import (
	"context"
	"strings"
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/lgc202/ingate/internal/envoy/config"
	"github.com/lgc202/ingate/internal/envoy/lastgood"
	"github.com/lgc202/ingate/internal/envoy/xds"
	"google.golang.org/protobuf/proto"
)

const (
	lastGoodRetryLimit   = 3
	maxErrorSummaryRunes = 512
)

type commandKind uint8

const (
	commandRestore commandKind = iota + 1
	commandSubmit
	commandXDSEvent
	commandACKTimeout
	commandLastGoodRetry
)

type command struct {
	kind commandKind
	ctx  context.Context

	result           config.CompileResult
	compileHasErrors bool
	event            xds.Event

	version     string
	sequence    uint64
	contentHash string
	record      lastgood.Record
	attempt     int

	reply chan<- error
}

type publishedConfig struct {
	version     string
	contentHash string
	config      config.Config
	snapshot    *cachev3.Snapshot
}

type candidateState struct {
	publishedConfig
	sequence      uint64
	requiredTypes []string
	responseSeen  bool
	timer         *time.Timer
}

type persistenceState struct {
	version     string
	contentHash string
	record      lastgood.Record
	attempt     int
	err         string
	timer       *time.Timer
}

type ackProgress struct {
	sent  map[string]bool
	acked map[string]bool
}

type streamState struct {
	nodeID   string
	versions map[string]*ackProgress
}

type runtimeState struct {
	initialized bool
	state       State
	sequence    uint64

	candidate *candidateState
	active    *publishedConfig
	streams   map[int64]*streamState

	lastGoodVersion string
	ackTimedOut     bool
	nackCount       int
	lastNACK        *NACK
	rejected        map[string]bool

	restoreError  string
	rollbackError string
	activeNACK    bool
	persistence   *persistenceState
}

func newRuntimeState() runtimeState {
	return runtimeState{
		state:    StateNoConfig,
		streams:  make(map[int64]*streamState),
		rejected: make(map[string]bool),
	}
}

func (s *runtimeState) refreshState() {
	if s.rollbackError != "" || s.activeNACK {
		s.state = StateDegraded
		return
	}
	if s.candidate != nil {
		s.state = StateWaitingForEnvoy
		if s.candidate.responseSeen {
			s.state = StateWaitingForACK
		}
		return
	}
	if s.restoreError != "" || (s.persistence != nil && s.persistence.err != "") {
		s.state = StateDegraded
		return
	}
	if s.active != nil {
		s.state = StateActive
		return
	}
	s.state = StateNoConfig
}

func (s *runtimeState) snapshot() Status {
	status := Status{
		LastGoodVersion: s.lastGoodVersion,
		ConfigReady:     s.candidate != nil || s.active != nil,
		State:           s.state,
		ConnectedEnvoys: len(s.streams),
		ACKs:            s.ackSummary(),
		NACKs:           NACKSummary{Count: s.nackCount},
		LastNACK:        s.lastNACK,
		ACKTimedOut:     s.ackTimedOut,
		RollbackError:   s.rollbackError,
	}
	if s.candidate != nil {
		status.CandidateVersion = s.candidate.version
	}
	if s.active != nil {
		status.ActiveVersion = s.active.version
	}
	if s.persistence != nil {
		status.PersistenceError = s.persistence.err
		status.PersistenceRetrying = s.persistence.timer != nil
	} else {
		status.PersistenceError = s.restoreError
	}
	return status
}

func (s *runtimeState) ackSummary() ACKSummary {
	var (
		version  string
		required []string
	)
	if s.candidate != nil {
		version = s.candidate.version
		required = s.candidate.requiredTypes
	} else if s.active != nil {
		version = s.active.version
		required = configTypeURLs(s.active.config)
	}
	if version == "" {
		return ACKSummary{}
	}

	received := 0
	for _, stream := range s.streams {
		progress := stream.versions[version]
		if progress == nil {
			continue
		}
		count := 0
		for _, typeURL := range required {
			if progress.acked[typeURL] {
				count++
			}
		}
		if count > received {
			received = count
		}
	}
	return ACKSummary{Required: len(required), Received: received}
}

func (s *runtimeState) stream(streamID int64, nodeID string) (*streamState, bool) {
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

func (s *runtimeState) pruneProgress(versions ...string) {
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

func summarizeError(err error) string {
	if err == nil {
		return ""
	}
	return summarizeMessage(err.Error())
}

func summarizeMessage(message string) string {
	message = strings.Join(strings.Fields(message), " ")
	runes := []rune(message)
	if len(runes) <= maxErrorSummaryRunes {
		return message
	}
	return string(runes[:maxErrorSummaryRunes])
}
