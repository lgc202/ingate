package extproc

import (
	"context"
	"errors"
	"fmt"

	grpcmetadata "google.golang.org/grpc/metadata"

	"github.com/lgc202/ingate/internal/pkg/aiproxyconfig"
)

// decodeRouteConfig 读取 Controller 写入 per-route ExtProc 配置的 gRPC metadata
func decodeRouteConfig(ctx context.Context) (aiproxyconfig.Config, error) {
	metadata, ok := grpcmetadata.FromIncomingContext(ctx)
	if !ok {
		return aiproxyconfig.Config{}, errors.New("ExtProc gRPC metadata is missing")
	}
	values := metadata.Get(aiproxyconfig.GRPCMetadataKey)
	if len(values) != 1 {
		return aiproxyconfig.Config{}, fmt.Errorf("ExtProc gRPC metadata %q must contain one value", aiproxyconfig.GRPCMetadataKey)
	}
	return aiproxyconfig.Decode(values[0])
}
