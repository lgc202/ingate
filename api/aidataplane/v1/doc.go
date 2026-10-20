// Package aidataplanev1 定义 Controller 向 AI Proxy 分发的独立配置资源。
// 它不包含网络端点或 TLS 参数，也不是面向管理端的业务 API。
package aidataplanev1

// ResourceType 是三类 AI Proxy 配置资源共用的 xDS 类型标识。
const ResourceType = "type.googleapis.com/aidataplane.v1.Resource"

// SharedDataPlaneCluster 标识当前共享 Envoy 与 AI Proxy 所属的数据面组。
const SharedDataPlaneCluster = "ingate-shared-data-plane"
