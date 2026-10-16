package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJSONContractsRejectUnknownConstraints(t *testing.T) {
	for _, resource := range []struct {
		path         string
		body         string
		unknownField string
	}{
		{
			path:         "ai-backends",
			body:         `{"backendRef":{"name":"network"},"schema":{"name":"OpenAI"}}`,
			unknownField: `,"unknownPolicy":true`,
		},
		{
			path:         "model-routes",
			body:         `{"parentRefs":[{"name":"edge"}],"rules":[{"matches":[{}],"backendRefs":[{"name":"models"}]}]}`,
			unknownField: `,"unknownModelMatch":"private-model"`,
		},
		{
			path:         "mcp-routes",
			body:         `{"parentRefs":[{"name":"edge"}],"backendRefs":[{"name":"network"}],"path":"/mcp"}`,
			unknownField: `,"headers":[{"name":"Authorization","value":"private-token"}]`,
		},
	} {
		for _, mediaType := range []string{"application/json", "application/protojson", "text/json"} {
			t.Run(resource.path+"/"+mediaType, func(t *testing.T) {
				server, _, client := newTestServers()
				url := "/api/admin/v1/namespaces/team-a/" + resource.path
				post := func(body string) *httptest.ResponseRecorder {
					request := httptest.NewRequest(http.MethodPost, url, strings.NewReader(body))
					request.Header.Set("Content-Type", mediaType)
					response := httptest.NewRecorder()
					server.ServeHTTP(response, request)

					return response
				}

				// 先证明除未知约束外的契约有效，避免因缺少必填字段而假通过。
				valid := post(resource.body)
				if valid.Code != http.StatusOK {
					t.Fatalf("valid %s = %d %s, want 200", url, valid.Code, valid.Body)
				}

				client.ClearActions()
				body := strings.TrimSuffix(resource.body, "}") + resource.unknownField + "}"
				response := post(body)

				if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "请求参数不合法") {
					t.Errorf("unknown constraint %s = %d %s, want 400/Chinese contract error", url, response.Code, response.Body)
				}

				if actions := client.Actions(); len(actions) != 0 {
					t.Errorf("unknown constraint %s made %d API calls, want 0", url, len(actions))
				}
			})
		}
	}
}
