package delivery

import (
	"context"
	"maps"
	"time"

	"github.com/lgc202/ingate/internal/controller/biz/compiler"
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

	result           compiler.Result
	compileHasErrors bool
	event            XDSEvent

	version  string
	sequence uint64

	reply chan<- error
}

type publishedConfig struct {
	version       string
	config        compiler.EnvoyConfig
	resources     []compiler.ResourceGeneration
	policyTargets []compiler.CompiledPolicyTarget
}

type candidateState struct {
	publishedConfig
	sequence             uint64
	requiredTypes        []string
	failurePolicyTargets []compiler.CompiledPolicyTarget
	timer                *time.Timer
}

type responseProgress struct {
	sent map[string]bool
}

type streamState struct {
	nodeID           string
	versions         map[string]*responseProgress
	acceptedVersions map[string]string
}

// deliveryState 只由 Delivery 的命令循环修改，外部读取通过并发安全的 Status 副本完成
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

func (s *deliveryState) status() Status {
	status := Status{LastFailure: s.lastFailure}
	if s.active != nil {
		status.ActiveResources = s.active.resources
		status.ActivePolicyTargets = s.active.policyTargets
	}
	return status.clone()
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
		maps.DeleteFunc(stream.versions, func(version string, _ *responseProgress) bool {
			return !keep[version]
		})
	}
}

func (s *streamState) progress(version string) *responseProgress {
	progress := s.versions[version]
	if progress != nil {
		return progress
	}
	progress = &responseProgress{sent: make(map[string]bool)}
	s.versions[version] = progress
	return progress
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

func cloneResourceGenerations(resources []compiler.ResourceGeneration) []compiler.ResourceGeneration {
	return append([]compiler.ResourceGeneration(nil), resources...)
}

func clonePolicyTargets(targets []compiler.CompiledPolicyTarget) []compiler.CompiledPolicyTarget {
	return append([]compiler.CompiledPolicyTarget(nil), targets...)
}
