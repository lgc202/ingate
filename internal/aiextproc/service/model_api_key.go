package service

// ModelAPIKeySource 提供当前已同步的模型 Service API Key
// 接口位于消费方，具体的数据同步方式不会进入请求处理流程
type ModelAPIKeySource interface {
	APIKey(serviceID string) (string, error)
}
