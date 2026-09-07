// Package telemetry 提供 Ingate 进程共享的身份、日志和 Trace 基础能力。
package telemetry

import (
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"

	"github.com/lgc202/ingate/internal/pkg/version"
)

const namespace = "ingate"

// Identity 描述一个进程实例在日志和遥测信号中的稳定身份。
type Identity struct {
	// Namespace 将同一组织下的服务归入统一命名空间。
	Namespace string
	// Name 是进程提供的服务名称。
	Name string
	// InstanceID 在每次进程启动时生成，用于区分同一主机上的多个实例。
	InstanceID string
	// Version 是当前构建版本。
	Version string
	// Environment 是实例所在的部署环境。
	Environment string
	// Hostname 是运行实例的主机名。
	Hostname string
}

// NewIdentity 创建当前进程实例的遥测身份。
// environment 为空时读取 INGATE_ENVIRONMENT，仍为空则标记为 unknown。
func NewIdentity(name, environment string) (Identity, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return Identity{}, fmt.Errorf("read hostname: %w", err)
	}
	environment = strings.TrimSpace(environment)
	if environment == "" {
		environment = strings.TrimSpace(os.Getenv("INGATE_ENVIRONMENT"))
	}
	if environment == "" {
		environment = "unknown"
	}
	return Identity{
		Namespace:   namespace,
		Name:        name,
		InstanceID:  uuid.NewString(),
		Version:     version.String(),
		Environment: environment,
		Hostname:    hostname,
	}, nil
}

// Resource 创建与进程身份一致的 OpenTelemetry Resource。
func (i Identity) Resource() *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNamespace(i.Namespace),
		semconv.ServiceName(i.Name),
		semconv.ServiceInstanceID(i.InstanceID),
		semconv.ServiceVersion(i.Version),
		semconv.DeploymentEnvironmentNameKey.String(i.Environment),
		semconv.HostName(i.Hostname),
	)
}
