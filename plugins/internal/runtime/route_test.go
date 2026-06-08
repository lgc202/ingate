package runtime

import "testing"

type indexedRoute struct {
	key  RouteKey
	name string
}

func TestRouteIndexFindsRouteByKey(t *testing.T) {
	index := NewRouteIndex([]indexedRoute{
		{
			key:  RouteKey{GatewayName: "gw", RouteName: "users", RuleName: "primary"},
			name: "users-primary",
		},
	}, func(route indexedRoute) RouteKey {
		return route.key
	})

	route, ok := index.Get(RouteKey{GatewayName: "gw", RouteName: "users", RuleName: "primary"})
	if !ok {
		t.Fatal("Get() ok = false, want true")
	}
	if route.name != "users-primary" {
		t.Fatalf("route.name = %q, want users-primary", route.name)
	}
}

func TestRouteIndexReturnsFalseForUnknownRoute(t *testing.T) {
	index := NewRouteIndex([]indexedRoute{}, func(route indexedRoute) RouteKey {
		return route.key
	})

	_, ok := index.Get(RouteKey{GatewayName: "gw", RouteName: "missing"})
	if ok {
		t.Fatal("Get() ok = true, want false")
	}
}
