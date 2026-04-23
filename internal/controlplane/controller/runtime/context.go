package runtime

import (
	externalversions "github.com/lgc202/ingate/pkg/generated/informers/externalversions"

	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"

	"github.com/lgc202/ingate/internal/controlplane/controller/index"
	"github.com/lgc202/ingate/internal/controlplane/controller/shared"
)

type Context struct {
	Clientset       clientset.Interface
	InformerFactory externalversions.SharedInformerFactory
	Index           *index.Index
	GatewayQueue    *shared.GatewayQueue
}

func NewContext(client clientset.Interface, factory externalversions.SharedInformerFactory, idx *index.Index, queue *shared.GatewayQueue) *Context {
	return &Context{
		Clientset:       client,
		InformerFactory: factory,
		Index:           idx,
		GatewayQueue:    queue,
	}
}
