package v1_test

import (
	"errors"
	"strings"
	"testing"

	"buf.build/go/protovalidate"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
)

// TestRequestNamespaceContract 固定所有管理请求的空间约束，避免机械生成后漏掉某个入口。
func TestRequestNamespaceContract(t *testing.T) {
	requests := []proto.Message{
		&adminv1.CreateGatewayRequest{},
		&adminv1.GetGatewayRequest{},
		&adminv1.ListGatewaysRequest{},
		&adminv1.UpdateGatewayRequest{},
		&adminv1.DeleteGatewayRequest{},
		&adminv1.CreateBackendRequest{},
		&adminv1.GetBackendRequest{},
		&adminv1.ListBackendsRequest{},
		&adminv1.UpdateBackendRequest{},
		&adminv1.DeleteBackendRequest{},
		&adminv1.CreateHTTPRouteRequest{},
		&adminv1.GetHTTPRouteRequest{},
		&adminv1.ListHTTPRoutesRequest{},
		&adminv1.UpdateHTTPRouteRequest{},
		&adminv1.DeleteHTTPRouteRequest{},
		&adminv1.CreateAIBackendRequest{},
		&adminv1.GetAIBackendRequest{},
		&adminv1.ListAIBackendsRequest{},
		&adminv1.UpdateAIBackendRequest{},
		&adminv1.DeleteAIBackendRequest{},
		&adminv1.CreateModelRouteRequest{},
		&adminv1.GetModelRouteRequest{},
		&adminv1.ListModelRoutesRequest{},
		&adminv1.UpdateModelRouteRequest{},
		&adminv1.DeleteModelRouteRequest{},
		&adminv1.CreateMCPRouteRequest{},
		&adminv1.GetMCPRouteRequest{},
		&adminv1.ListMCPRoutesRequest{},
		&adminv1.UpdateMCPRouteRequest{},
		&adminv1.DeleteMCPRouteRequest{},
	}
	namespaces := []string{
		"team-a", "a", strings.Repeat("a", 63),
		"", "Team-A", "team.a", "-team", "team-", strings.Repeat("a", 64),
	}
	for _, request := range requests {
		t.Run(string(request.ProtoReflect().Descriptor().Name()), func(t *testing.T) {
			message := request.ProtoReflect()
			field := message.Descriptor().Fields().ByName("namespace")
			if field == nil {
				t.Fatal("request descriptor lacks namespace, want explicit namespace field")
			}

			// 只比较 namespace 字段规则；其它必填字段有各资源自己的协议测试。
			filter := protovalidate.WithFilter(protovalidate.FilterFunc(func(
				_ protoreflect.Message,
				descriptor protoreflect.Descriptor,
			) bool {
				return descriptor == field
			}))
			for _, namespace := range namespaces {
				message.Set(field, protoreflect.ValueOfString(namespace))
				err := protovalidate.Validate(request, filter)
				wantValid := namespace != "" && len(apivalidation.ValidateNamespaceName(namespace, false)) == 0
				if (err == nil) != wantValid {
					t.Errorf("Validate(%T, namespace=%q) error = %v, want valid=%t", request, namespace, err, wantValid)
				}

				if err != nil {
					if _, ok := errors.AsType[*protovalidate.ValidationError](err); !ok {
						t.Errorf("Validate(%T, namespace=%q) error = %v, want contract violation", request, namespace, err)
					}
				}
			}
		})
	}
}
