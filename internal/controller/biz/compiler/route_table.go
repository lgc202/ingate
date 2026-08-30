package compiler

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/lgc202/ingate/internal/pkg/extauthz"
)

const (
	envoyRouteNamePrefix  = "ingate-route"
	virtualHostNamePrefix = "ingate-vhost"
)

// routeAttachment 表示一条 Route 已成功展开到某个 Gateway Listener。
// 内置治理策略只根据成功挂载结果生成执行索引。
type routeAttachment struct {
	listenerKey listenerKey
	gatewayID   string
	routeID     string
	routes      []*routev3.Route
}

// routeTable 维护 Listener 和 Domain 下的 Route、匹配占用关系及治理策略附件。
type routeTable struct {
	entriesByListener   map[listenerKey]map[string][]routeEntry
	entriesByMatchClass map[listenerKey]map[string]map[routeMatchClass][]routeEntry
	attachmentIndex     map[policyRouteKey]int
	attachments         []routeAttachment
}

func newRouteTable(listenerCount int) *routeTable {
	return &routeTable{
		entriesByListener: make(map[listenerKey]map[string][]routeEntry, listenerCount),
		entriesByMatchClass: make(
			map[listenerKey]map[string]map[routeMatchClass][]routeEntry,
			listenerCount,
		),
		attachmentIndex: make(map[policyRouteKey]int),
	}
}

func (t *routeTable) addEntry(
	key listenerKey,
	domain string,
	gatewayID string,
	entry routeEntry,
	accessConfig *anypb.Any,
) (string, bool) {
	if t.entriesByListener[key] == nil {
		t.entriesByListener[key] = make(map[string][]routeEntry)
	}
	if t.entriesByMatchClass[key] == nil {
		t.entriesByMatchClass[key] = make(map[string]map[routeMatchClass][]routeEntry)
	}
	if t.entriesByMatchClass[key][domain] == nil {
		t.entriesByMatchClass[key][domain] = make(map[routeMatchClass][]routeEntry)
	}

	matchClass := routeMatchClass{
		path:        entry.path,
		exactPath:   entry.exactPath,
		method:      entry.method,
		headerCount: entry.headerCount,
	}
	for _, existing := range t.entriesByMatchClass[key][domain][matchClass] {
		if routeHeaderMatchesOverlap(existing.matchHeaders, entry.matchHeaders) {
			return existing.routeID, true
		}
	}

	current := entry
	current.route = proto.Clone(entry.route).(*routev3.Route)
	if accessConfig != nil {
		if current.route.TypedPerFilterConfig == nil {
			current.route.TypedPerFilterConfig = make(map[string]*anypb.Any)
		}
		current.route.TypedPerFilterConfig[extauthz.FilterName] = accessConfig
	}
	current.route.Name = envoyRouteName(gatewayID, entry.routeID, entry.method, entry.variant)

	t.entriesByMatchClass[key][domain][matchClass] = append(
		t.entriesByMatchClass[key][domain][matchClass],
		current,
	)
	t.entriesByListener[key][domain] = append(t.entriesByListener[key][domain], current)

	attachmentKey := policyRouteKey{
		listenerKey: key,
		gatewayID:   gatewayID,
		routeID:     entry.routeID,
	}
	index, exists := t.attachmentIndex[attachmentKey]
	if !exists {
		index = len(t.attachments)
		t.attachmentIndex[attachmentKey] = index
		t.attachments = append(t.attachments, routeAttachment{
			listenerKey: key,
			gatewayID:   gatewayID,
			routeID:     entry.routeID,
		})
	}
	t.attachments[index].routes = append(t.attachments[index].routes, current.route)
	return "", false
}

func (t *routeTable) configurations(
	listenerGroups map[listenerKey]*listenerGroup,
) []*routev3.RouteConfiguration {
	configs := make([]*routev3.RouteConfiguration, 0, len(listenerGroups))
	for _, key := range sortedListenerKeys(listenerGroups) {
		virtualHosts := make([]*routev3.VirtualHost, 0, len(t.entriesByListener[key]))
		for _, domain := range slices.Sorted(maps.Keys(t.entriesByListener[key])) {
			entries := t.entriesByListener[key][domain]
			slices.SortFunc(entries, compareRouteEntries)
			routes := make([]*routev3.Route, 0, len(entries))
			for _, entry := range entries {
				routes = append(routes, entry.route)
			}
			virtualHosts = append(virtualHosts, &routev3.VirtualHost{
				Name:    virtualHostName(key, domain),
				Domains: []string{domain},
				Routes:  routes,
			})
		}
		configs = append(configs, &routev3.RouteConfiguration{
			Name:         routeConfigName(key),
			VirtualHosts: virtualHosts,
		})
	}
	return configs
}

func (t *routeTable) sortedAttachments() []routeAttachment {
	attachments := slices.Clone(t.attachments)
	slices.SortFunc(attachments, compareRouteAttachments)
	return attachments
}

func compareRouteAttachments(a, b routeAttachment) int {
	if result := compareListenerKeys(a.listenerKey, b.listenerKey); result != 0 {
		return result
	}
	if result := cmp.Compare(a.gatewayID, b.gatewayID); result != 0 {
		return result
	}
	return cmp.Compare(a.routeID, b.routeID)
}

func envoyRouteName(gatewayID, routeID, method, variant string) string {
	name := fmt.Sprintf("%s/%s/%s", envoyRouteNamePrefix, gatewayID, routeID)
	if method != "" {
		name += "/" + strings.ToLower(method)
	}
	if variant != "" {
		name += "/" + variant
	}
	return name
}

func virtualHostName(key listenerKey, domain string) string {
	return fmt.Sprintf("%s/%s/%s", virtualHostNamePrefix, listenerName(key), domain)
}
