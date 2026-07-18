package wasm

import "testing"

func TestParseRouteConfigName(t *testing.T) {
	const routeName = "ingate-route/gateway-1/route-1/chat/POST/ai/config-1"

	identity, configID, ok := ParseRouteConfigName("ingate-route", "ai", routeName)
	if !ok {
		t.Fatal("ParseRouteConfigName(AI route) ok = false, want true")
	}
	if identity.GatewayName != "gateway-1" || identity.RouteName != "route-1" || identity.RuleName != "chat" {
		t.Errorf("ParseRouteConfigName(AI route) identity = %#v, want gateway-1/route-1/chat", identity)
	}
	if configID != "config-1" {
		t.Errorf("ParseRouteConfigName(AI route) configID = %q, want %q", configID, "config-1")
	}

	if _, _, ok := ParseRouteConfigName("ingate-route", "ai", "ingate-route/gateway-1/route-1/chat/POST"); ok {
		t.Error("ParseRouteConfigName(ordinary route) ok = true, want false")
	}
}

func TestParseRouteNameKeepsPolicyIdentityForAIRoute(t *testing.T) {
	identity, ok := ParseRouteName("ingate-route", "ingate-route/gateway-1/route-1/chat/POST/ai/config-1")
	if !ok {
		t.Fatal("ParseRouteName(AI route) ok = false, want true")
	}
	if identity.GatewayName != "gateway-1" || identity.RouteName != "route-1" || identity.RuleName != "chat" {
		t.Errorf("ParseRouteName(AI route) identity = %#v, want gateway-1/route-1/chat", identity)
	}
}
