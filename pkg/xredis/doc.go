// Package xredis 封装项目通用 Redis 连接能力
//
// 这个包面向基础设施复用，只处理 Redis 连接配置、部署模式、连接池复用和生命周期管理。
// 限流、缓存、事件投递等业务语义应由上层包实现，并把自己的领域配置转换成 Config。
package xredis
