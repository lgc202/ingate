package gateway

import (
	"path"
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

// gatewaySelection 只在 Gateway 投影变化时集中裁决，不依赖 Route 或 Backend。
type gatewaySelection struct {
	gateways  []gatewayv1alpha1.Gateway
	accepted  []*gatewayv1alpha1.Gateway
	conflicts []gatewayConflict
}

// listenerEntry 是一个已接纳入口。只保存自身 Listener，修改兄弟入口不改变其内容。
type listenerEntry struct {
	origin   metav1.ObjectMeta
	listener gatewayv1alpha1.Listener
	index    int
}

// routeParents 不含匹配内容；修改路径、模型条件不会重新计算附着关系。
type routeParents struct {
	key      client.ObjectKey
	parents  []gatewayv1alpha1.ParentReference
	deleting bool
}

type routeAttachment struct {
	route   *routeParents
	entries []string
}

// entryTable 缓存单入口的路由结果；网络内容由 Backend 集合唯一拥有。
// aiErr 只阻止 AI 发布，不影响该入口的普通 HTTP 路由。
type entryTable struct {
	entry        *listenerEntry
	http         dataplane.Host
	ai           aidataplane.Host
	protocols    []aidataplane.Backend
	httpNetworks []string
	aiNetworks   []string
	versions     map[string]compiledVersion
	aiErr        error
}

// compiledVersion 标识入口表实际使用的匹配结果，不是 API Server 的资源版本。
// krt 保留内容相等的旧实例，因此可以用强类型指针核对三类集合是否已经收敛。
type compiledVersion struct {
	http  *httpRoute
	model *modelRoute
	mcp   *mcpRoute
}

func selectGatewayEntries(gateways []*gatewayv1alpha1.Gateway) *gatewaySelection {
	selection := &gatewaySelection{}
	for _, gateway := range gateways {
		selection.gateways = append(selection.gateways, *gateway)
	}

	slices.SortFunc(selection.gateways, func(a, b gatewayv1alpha1.Gateway) int { return compareMeta(a.ObjectMeta, b.ObjectMeta) })

	var result resolution
	result.selectGateways(selection.gateways)
	selection.accepted = result.gateways
	selection.conflicts = result.gatewayConflicts

	return selection
}

// ResourceName 是集中裁决结果的固定集合键。
func (s gatewaySelection) ResourceName() string {
	return "gateway-selection"
}

// ResourceName 与共享协议中 Gateway Listener 的入口名称一致。
func (e *listenerEntry) ResourceName() string {
	return path.Join("gateway", e.origin.Namespace, e.origin.Name, "listener", e.listener.Name)
}

func (e *listenerEntry) gateway() gatewayv1alpha1.Gateway {
	return gatewayv1alpha1.Gateway{
		ObjectMeta: e.origin,
		Spec:       gatewayv1alpha1.GatewaySpec{Listeners: []gatewayv1alpha1.Listener{e.listener}},
	}
}

func (e *listenerEntry) matches(namespace string, parents []gatewayv1alpha1.ParentReference) bool {
	if namespace != e.origin.Namespace || e.listener.Protocol != gatewayv1alpha1.ProtocolHTTP {
		return false
	}

	for _, parent := range parents {
		if parent.Name == e.origin.Name && (parent.SectionName == "" || parent.SectionName == e.listener.Name) {
			return true
		}
	}

	return false
}

// ResourceName 在各 Kind 独立的附着集合中使用同空间资源键。
func (r *routeParents) ResourceName() string {
	return path.Join(r.key.Namespace, r.key.Name)
}

// entriesCurrent 核对父引用查询的入口集合，包含缺失父引用后来出现的情况。
// 这只是发布时的身份与依赖校验，不重新编译匹配、优先级或后端协议。
func (r *routeParents) entriesCurrent(queried map[string]*listenerEntry, byGateway map[string][]*listenerEntry) bool {
	expected := make(map[string]*listenerEntry)
	if !r.deleting {
		for _, parent := range r.parents {
			for _, entry := range byGateway[path.Join(r.key.Namespace, parent.Name)] {
				expected[entry.ResourceName()] = entry
			}
		}
	}

	if len(expected) != len(queried) {
		return false
	}

	for name, entry := range queried {
		current := expected[name]
		if current == nil || current.listener != entry.listener || current.index != entry.index ||
			compareMeta(current.origin, entry.origin) != 0 {
			return false
		}
	}

	return true
}

func (r *routeParents) attachmentsCurrent(entries []string, byGateway map[string][]*listenerEntry) bool {
	expected := make(map[string]bool)
	if !r.deleting {
		for _, parent := range r.parents {
			for _, entry := range byGateway[path.Join(r.key.Namespace, parent.Name)] {
				if entry.matches(r.key.Namespace, []gatewayv1alpha1.ParentReference{parent}) {
					expected[entry.ResourceName()] = true
				}
			}
		}
	}

	if len(expected) != len(entries) {
		return false
	}

	for _, entry := range entries {
		if !expected[entry] {
			return false
		}
	}

	return true
}

// ResourceName 沿用原 Route 的身份。
func (r *routeAttachment) ResourceName() string {
	return r.route.ResourceName()
}

// ResourceName 让入口路由表与其 Listener 使用同一个身份。
func (t *entryTable) ResourceName() string {
	return t.entry.ResourceName()
}

func (v compiledVersion) parentRefs() routeParents {
	switch {
	case v.http != nil:
		return routeParents{key: client.ObjectKey{Namespace: v.http.Namespace, Name: v.http.Name}, parents: v.http.parents}
	case v.model != nil:
		return routeParents{
			key: client.ObjectKey{Namespace: v.model.Namespace, Name: v.model.Name}, parents: v.model.parents,
			deleting: !v.model.DeletionTimestamp.IsZero(),
		}
	default:
		return routeParents{
			key: client.ObjectKey{Namespace: v.mcp.Namespace, Name: v.mcp.Name}, parents: v.mcp.parents,
			deleting: !v.mcp.DeletionTimestamp.IsZero(),
		}
	}
}

func compileEntryTable(entry *listenerEntry, snapshot resources) *entryTable {
	snapshot.gateways.Items = []gatewayv1alpha1.Gateway{entry.gateway()}
	resolved := resolve(snapshot.gateways.Items, snapshot.httpRoutes, snapshot.backends)
	http := compileHTTPListeners(resolved)
	// entry 已经通过集中裁决，其单入口视图必然产生一个 Listener 和 Host。
	table := &entryTable{
		entry:    entry,
		http:     http[0].Hosts[0],
		versions: make(map[string]compiledVersion),
	}
	for _, backend := range resolved.backends {
		table.httpNetworks = append(table.httpNetworks, backend.ResourceName())
	}

	for _, route := range snapshot.httpRoutes {
		if entry.matches(route.Namespace, route.parents) {
			table.versions["http/"+route.ResourceName()] = compiledVersion{http: route}
		}
	}

	for _, route := range snapshot.modelRoutes {
		if route.DeletionTimestamp.IsZero() && entry.matches(route.Namespace, route.parents) {
			table.versions["model/"+route.ResourceName()] = compiledVersion{model: route}
		}
	}

	for _, route := range snapshot.mcpRoutes {
		if route.DeletionTimestamp.IsZero() && entry.matches(route.Namespace, route.parents) {
			table.versions["mcp/"+route.ResourceName()] = compiledVersion{mcp: route}
		}
	}

	ai, err := resolveAI(snapshot)
	if err != nil {
		table.aiErr = err

		return table
	}

	config := ai.buildConfig()
	if len(config.Listeners) != 0 {
		table.ai = config.Listeners[0].Hosts[0]
	}

	table.protocols = config.Backends
	for name := range ai.networkBackends {
		table.aiNetworks = append(table.aiNetworks, name)
	}

	slices.Sort(table.aiNetworks)

	return table
}
