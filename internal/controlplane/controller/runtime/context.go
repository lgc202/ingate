package runtime

import (
	"k8s.io/client-go/util/workqueue"

	"github.com/lgc202/ingate/internal/controlplane/controller/index"
	"github.com/lgc202/ingate/internal/controlplane/controller/shared"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
	externalversions "github.com/lgc202/ingate/pkg/generated/informers/externalversions"
)

type Context struct {
	Clientset       clientset.Interface
	InformerFactory externalversions.SharedInformerFactory
	Index           *index.Index
	GatewayQueue    workqueue.TypedRateLimitingInterface[shared.ObjectKey]
}

func NewContext(client clientset.Interface, factory externalversions.SharedInformerFactory, idx *index.Index, queue workqueue.TypedRateLimitingInterface[shared.ObjectKey]) *Context {
	return &Context{
		Clientset:       client,
		InformerFactory: factory,
		Index:           idx,
		GatewayQueue:    queue,
	}
}
