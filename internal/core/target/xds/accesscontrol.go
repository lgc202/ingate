package xds

import pluginacl "github.com/lgc202/ingate/pkg/plugin/acl"

// AccessControlConfig 表示 xDS target 内部保留的内置访问控制插件编译结果
type AccessControlConfig struct {
	Bindings []pluginacl.Binding `json:"bindings,omitempty"`
}
