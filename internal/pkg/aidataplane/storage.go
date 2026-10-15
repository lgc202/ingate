package aidataplane

const (
	// ConfigMapName 是共享 AI 配置的固定对象名；Namespace 由部署显式提供。
	ConfigMapName = "ingate-shared-data-plane-ai"
	// ConfigMapDataKey 是 ConfigMap.data 中存放完整配置 JSON 的键。
	ConfigMapDataKey = "config.json"
)
