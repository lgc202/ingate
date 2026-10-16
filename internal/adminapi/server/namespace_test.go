package server_test

import (
	"net/http"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	clienttesting "k8s.io/client-go/testing"
)

// TestResourceNamespaceHTTPBoundary 核对生成的路径绑定、请求校验及原有入口移除。
// 后端为 fake Client；它验证管理请求的空间传递，不替代 API Server 存储测试。
func TestResourceNamespaceHTTPBoundary(t *testing.T) {
	tests := []struct {
		path string
		body string
	}{
		{path: "gateways", body: `{"listeners":[{"name":"http","protocol":"LISTENER_PROTOCOL_HTTP","port":8080}]}`},
		{path: "backends", body: `{"endpoints":[{"address":"example.com","port":8080,"weight":1}]}`},
		{path: "http-routes", body: `{"parentRefs":[{"name":"edge"}],"rules":[{"backendRefs":[{"name":"network"}]}]}`},
		{path: "ai-backends", body: `{"backendRef":{"name":"network"},"schema":{"name":"OpenAI"}}`},
		{path: "model-routes", body: `{"parentRefs":[{"name":"edge"}],"rules":[{"backendRefs":[{"name":"models"}]}]}`},
		{path: "mcp-routes", body: `{"parentRefs":[{"name":"edge"}],"backendRefs":[{"name":"network"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			httpServer, _, client := newTestServers()
			// 原生 HTTP 绑定以路径为目标，不允许 body 切换到其它空间。
			body := `{"namespace":"team-b",` + tt.body[1:]
			path := "/api/admin/v1/namespaces/team-a/" + tt.path
			response := requestResource(httpServer, http.MethodPost, path, body)
			requireHTTPStatus(t, response, http.StatusOK)
			actions := client.Actions()
			if len(actions) != 1 {
				t.Fatalf("POST %s API actions = %v, want one create", path, actions)
			}

			create, ok := actions[0].(clienttesting.CreateAction)
			if !ok {
				t.Fatalf("POST %s action = %T, want CreateAction", path, actions[0])
			}

			object, err := meta.Accessor(create.GetObject())
			if err != nil {
				t.Fatalf("POST %s object metadata error = %v, want nil", path, err)
			}

			if create.GetNamespace() != "team-a" || object.GetNamespace() != "team-a" {
				t.Errorf("POST %s action/object namespace = %q/%q, want team-a/team-a", path, create.GetNamespace(), object.GetNamespace())
			}

			client.ClearActions()

			for _, namespace := range []string{"Team-A", "team.a", "-team"} {
				response := requestResource(httpServer, http.MethodPost, "/api/admin/v1/namespaces/"+namespace+"/"+tt.path, tt.body)
				requireHTTPStatus(t, response, http.StatusBadRequest)
			}

			for _, request := range []struct{ method, suffix string }{
				{method: http.MethodPost},
				{method: http.MethodGet},
				{method: http.MethodGet, suffix: "/resource"},
				{method: http.MethodPut, suffix: "/resource"},
				{method: http.MethodDelete, suffix: "/resource"},
			} {
				response := requestResource(httpServer, request.method, "/api/admin/v1/"+tt.path+request.suffix, tt.body)
				requireHTTPStatus(t, response, http.StatusNotFound)
			}

			if actions := client.Actions(); len(actions) != 0 {
				t.Errorf("invalid namespace or legacy %s requests API actions = %v, want none", tt.path, actions)
			}
		})
	}
}
