package envoy

import (
	"testing"

	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"google.golang.org/protobuf/proto"
)

func TestIngressTranslationReusesUnaffectedPorts(t *testing.T) {
	var emptyHTTP httpTranslation
	http := compileTestHTTP(t, &emptyHTTP, twoPortConfig())
	orders := entryKey{routeName: "route/8080", hostName: "gateway/orders"}
	operations := entryKey{routeName: "route/9090", hostName: "gateway/operations"}
	entries := map[entryKey][]*routev3.Route{
		orders:     {newAIIngressRoute(8080, orders.hostName, "/mcp", "mcp")},
		operations: {newAIIngressRoute(9090, operations.hostName, "/ops", "mcp")},
	}
	var empty ingressTranslation

	first := empty.merge(http.resources.routes, entries)

	// AI 适配器可能提供等值的新消息；比较内容而非地址，避免重复合入。
	entries[orders][0] = proto.Clone(entries[orders][0]).(*routev3.Route)

	again := first.merge(http.resources.routes, entries)

	if again.byName["route/8080"].resource != first.byName["route/8080"].resource ||
		again.byName["route/9090"].resource != first.byName["route/9090"].resource {
		t.Error("merge(equal AI entries) rebuilt public RDS")
	}

	entries[orders] = []*routev3.Route{newAIIngressRoute(8080, orders.hostName, "/v2/mcp", "mcp")}

	changed := again.merge(http.resources.routes, entries)

	if changed.byName["route/8080"].resource == again.byName["route/8080"].resource ||
		changed.byName["route/9090"].resource != again.byName["route/9090"].resource {
		t.Error("merge(changed 8080 AI entry) did not remain scoped to route/8080")
	}

	if got := first.byName["route/8080"].resource.VirtualHosts[0].Routes[0].Match.GetPath(); got != "/mcp" {
		t.Errorf("merge(changed entry) modified previous RDS path = %q, want /mcp", got)
	}

	if got := http.routes[8080].resource.VirtualHosts[0].Routes[0].Name; got != "orders" {
		t.Errorf("merge(AI entry) modified HTTP source route = %q, want orders", got)
	}

	delete(entries, orders)

	removed := changed.merge(http.resources.routes, entries)

	if removed.byName["route/8080"].resource != http.routes[8080].resource ||
		removed.byName["route/9090"].resource != changed.byName["route/9090"].resource {
		t.Error("merge(removed AI entry) did not restore the base HTTP RDS or preserve the other port")
	}

	entries[entryKey{routeName: "route/8080", hostName: "absent"}] = []*routev3.Route{newAIIngressRoute(8080, "absent", "/ignored", "mcp")}

	unmatched := removed.merge(http.resources.routes, entries)

	if unmatched.byName["route/8080"].resource != removed.byName["route/8080"].resource ||
		len(unmatched.byName["route/8080"].entries) != 0 {
		t.Error("merge(unmatched Host) attached an entry or rebuilt unrelated HTTP RDS")
	}

	deleted := unmatched.merge(http.resources.routes[1:], entries)
	if len(deleted.byName) != 1 || deleted.byName["route/8080"].resource != nil {
		t.Error("merge(deleted public port) retained its cached RDS")
	}
}
