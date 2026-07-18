// Package xds 提供基于 go-control-plane SotW Snapshot Cache 的 ADS 基础能力
package xds

import "context"

// EventKind 表示 xDS stream 生命周期和响应确认事件类型
type EventKind string

const (
	// EventStreamOpened 表示 SotW stream 已建立
	EventStreamOpened EventKind = "StreamOpened"
	// EventStreamClosed 表示 SotW stream 已关闭
	EventStreamClosed EventKind = "StreamClosed"
	// EventResponseSent 表示 SotW response 已交给底层 stream 发送
	EventResponseSent EventKind = "ResponseSent"
	// EventAcceptedVersionObserved 表示 Envoy 主动报告某类配置当前已接受的版本
	EventAcceptedVersionObserved EventKind = "AcceptedVersionObserved"
	// EventACK 表示 Envoy 接受了对应 nonce 的配置版本
	EventACK EventKind = "ACK"
	// EventNACK 表示 Envoy 拒绝了对应 nonce 的配置版本
	EventNACK EventKind = "NACK"
)

// Event 表示 xDS 层交给配置 Delivery 和状态模块的协议事件
type Event struct {
	Kind            EventKind
	StreamID        int64
	NodeID          string
	TypeURL         string
	Version         string
	AcceptedVersion string
}

// EventSink 同步处理一个 xDS 事件
type EventSink func(context.Context, Event) error
