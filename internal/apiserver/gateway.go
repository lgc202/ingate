package apiserver

import (
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"

	gatewayregistry "github.com/lgc202/ingate/internal/apiserver/registry/gateway"
	gatewayapi "github.com/lgc202/ingate/pkg/apis/gateway"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

// gatewayNegotiatedSerializer 保留上游版本转换和流式编码，仅排除业务类型未实现的 Protobuf。
// 核心组仍使用完整 CodecFactory，Namespace、ConfigMap 的原生编码不受影响。
type gatewayNegotiatedSerializer struct {
	serializer.CodecFactory
}

func (s gatewayNegotiatedSerializer) SupportedMediaTypes() []runtime.SerializerInfo {
	return slices.DeleteFunc(slices.Clone(s.CodecFactory.SupportedMediaTypes()), func(info runtime.SerializerInfo) bool {
		return info.MediaType == runtime.ContentTypeProtobuf
	})
}

func installGatewayAPIGroup(
	server *genericapiserver.GenericAPIServer,
	scheme *runtime.Scheme,
	optionsGetter generic.RESTOptionsGetter,
) error {
	storage, err := gatewayregistry.NewStorage(scheme, optionsGetter)
	if err != nil {
		return fmt.Errorf("create %s API group storage: %w", gatewayapi.GroupName, err)
	}

	apiGroupInfo := gatewayAPIGroupInfo(scheme, storage)
	if err := server.InstallAPIGroup(&apiGroupInfo); err != nil {
		for _, resourceStorage := range storage {
			resourceStorage.Destroy()
		}

		return fmt.Errorf("install %s API group: %w", gatewayapi.GroupName, err)
	}

	return nil
}

func gatewayAPIGroupInfo(
	scheme *runtime.Scheme,
	storage map[string]rest.Storage,
) genericapiserver.APIGroupInfo {
	codecs := serializer.NewCodecFactory(scheme)
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(
		gatewayapi.GroupName,
		scheme,
		runtime.NewParameterCodec(scheme),
		codecs,
	)
	apiGroupInfo.NegotiatedSerializer = gatewayNegotiatedSerializer{CodecFactory: codecs}
	apiGroupInfo.VersionedResourcesStorageMap[gatewayv1alpha1.Version] = storage

	return apiGroupInfo
}
