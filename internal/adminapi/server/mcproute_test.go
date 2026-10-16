package server_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	kratosmiddleware "github.com/go-kratos/kratos/v3/middleware"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestCreateMCPRouteReturnsSavedStateHTTP(t *testing.T) {
	for _, tt := range []struct {
		name         string
		path         *string
		backendPath  *string
		wantPath     string
		wantUpstream string
	}{
		{name: "omitted paths", wantPath: "/mcp", wantUpstream: "/mcp"},
		{
			name:         "independent upstream default",
			path:         ptr.To("/tools/mcp/"),
			wantPath:     "/tools/mcp/",
			wantUpstream: "/mcp",
		},
		{
			name:         "explicit paths",
			path:         ptr.To("/工具/%2F/"),
			backendPath:  ptr.To("/"),
			wantPath:     "/工具/%2F/",
			wantUpstream: "/",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			httpServer, _, client := newTestServers()
			createdAt := time.Date(2026, 10, 12, 1, 2, 3, 0, time.UTC)
			client.PrependReactor("create", "mcproutes", func(action clienttesting.Action) (bool, runtime.Object, error) {
				route := action.(clienttesting.CreateAction).GetObject().(*gatewayv1alpha1.MCPRoute).DeepCopy()
				if diff := cmp.Diff(tt.path, route.Spec.Path); diff != "" {
					t.Errorf("CreateMCPRoute changed entry path before API defaulting (-want +got):\n%s", diff)
				}

				if diff := cmp.Diff(tt.backendPath, route.Spec.BackendRefs[0].Path); diff != "" {
					t.Errorf("CreateMCPRoute changed upstream path before API defaulting (-want +got):\n%s", diff)
				}

				gatewayv1alpha1.SetDefaultsMCPRoute(route)
				route.ResourceVersion = "9"
				route.CreationTimestamp = metav1.NewTime(createdAt)

				return true, route, nil
			})
			server := httptest.NewServer(httpServer)
			t.Cleanup(server.Close)
			httpClient, err := kratoshttp.NewClient(t.Context(), kratoshttp.WithEndpoint(server.URL))
			if err != nil {
				t.Fatalf("http.NewClient() error = %v, want nil", err)
			}

			t.Cleanup(func() {
				if err := httpClient.Close(); err != nil {
					t.Errorf("Client.Close() error = %v, want nil", err)
				}
			})
			request := &adminv1.CreateMCPRouteRequest{
				Namespace:   "team-a",
				DisplayName: "工具路由",
				ParentRefs: []*adminv1.ParentReference{
					{Name: "edge", SectionName: "z"},
					{Name: "edge", SectionName: "a"},
				},
				Path:        tt.path,
				BackendRefs: []*adminv1.MCPBackendReference{{Name: "network", Path: tt.backendPath}},
			}
			reply, err := adminv1.NewMCPRouteServiceHTTPClient(httpClient).CreateMCPRoute(t.Context(), request)
			if err != nil {
				t.Fatalf("CreateMCPRoute(HTTP round trip) error = %v, want nil", err)
			}

			name := reply.GetRoute().GetName()
			if !strings.HasPrefix(name, "mcproute-") {
				t.Errorf("CreateMCPRoute().name = %q, want generated mcproute ID", name)
			}

			want := &adminv1.MCPRoute{
				Namespace:   "team-a",
				Name:        name,
				DisplayName: request.DisplayName,
				ParentRefs:  request.ParentRefs,
				Path:        ptr.To(tt.wantPath),
				BackendRefs: []*adminv1.MCPBackendReference{{Name: "network", Path: ptr.To(tt.wantUpstream)}},
				CreateTime:  timestamppb.New(createdAt),
				Version:     "9",
			}
			if diff := cmp.Diff(want, reply.GetRoute(), protocmp.Transform()); diff != "" {
				t.Errorf("CreateMCPRoute(saved paths and parents) mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMCPRouteUpdateAndDeleteHTTP(t *testing.T) {
	seed := &gatewayv1alpha1.MCPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "team-a",
			Name:            "tools",
			UID:             types.UID("route-instance"),
			ResourceVersion: "7",
			Labels:          map[string]string{"team": "core"},
			Finalizers:      []string{"gateway.ingate.io/cleanup"},
		},
		Spec: gatewayv1alpha1.MCPRouteSpec{
			DisplayName: "旧名称",
			ParentRefs:  []gatewayv1alpha1.ParentReference{{Name: "edge", SectionName: "http"}},
			Path:        ptr.To("/old-entry"),
			BackendRefs: []gatewayv1alpha1.MCPBackendReference{{Name: "old-backend", Path: ptr.To("/old-upstream")}},
		},
		Status: gatewayv1alpha1.MCPRouteStatus{Parents: []gatewayv1alpha1.RouteParentStatus{{ControllerName: "gateway.ingate.io/test"}}},
	}
	httpServer, client := crudTestServer(t, seed)
	client.PrependReactor("update", "mcproutes", func(action clienttesting.Action) (bool, runtime.Object, error) {
		gatewayv1alpha1.SetDefaultsMCPRoute(action.(clienttesting.UpdateAction).GetObject().(*gatewayv1alpha1.MCPRoute))
		return false, nil, nil
	})
	stale := `{"version":"stale","parentRefs":[{"name":"other"}],"backendRefs":[{"name":"other-backend"}]}`
	response := requestResource(httpServer, http.MethodPut, "/api/admin/v1/namespaces/team-a/mcp-routes/tools", stale)
	requireHTTPStatus(t, response, http.StatusConflict)
	stored, err := client.GatewayV1alpha1().MCPRoutes("team-a").Get(t.Context(), "tools", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(MCPRoute after stale update) error = %v, want nil", err)
	}

	if diff := cmp.Diff(seed, stored); diff != "" {
		t.Errorf("UpdateMCPRoute(stale) changed state (-want +got):\n%s", diff)
	}

	body := `{"version":"7","parentRefs":[{"name":"other"}],"backendRefs":[{"name":"other-backend"}]}`
	response = requestResource(httpServer, http.MethodPut, "/api/admin/v1/namespaces/team-a/mcp-routes/tools", body)
	requireHTTPStatus(t, response, http.StatusOK)
	stored, err = client.GatewayV1alpha1().MCPRoutes("team-a").Get(t.Context(), "tools", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(updated MCPRoute) error = %v, want nil", err)
	}

	want := seed.DeepCopy()
	want.Spec = gatewayv1alpha1.MCPRouteSpec{
		ParentRefs:  []gatewayv1alpha1.ParentReference{{Name: "other"}},
		Path:        ptr.To("/mcp"),
		BackendRefs: []gatewayv1alpha1.MCPBackendReference{{Name: "other-backend", Path: ptr.To("/mcp")}},
	}
	if diff := cmp.Diff(want, stored); diff != "" {
		t.Errorf("UpdateMCPRoute(full PUT, metadata/status preserved) mismatch (-want +got):\n%s", diff)
	}

	var reply adminv1.UpdateMCPRouteReply
	decodeReply(t, response, &reply)
	wantReply := &adminv1.MCPRoute{
		Namespace:   "team-a",
		Name:        "tools",
		ParentRefs:  []*adminv1.ParentReference{{Name: "other"}},
		Path:        ptr.To("/mcp"),
		BackendRefs: []*adminv1.MCPBackendReference{{Name: "other-backend", Path: ptr.To("/mcp")}},
		CreateTime:  timestamppb.New(seed.CreationTimestamp.Time),
		Version:     "7",
	}
	if diff := cmp.Diff(wantReply, reply.GetRoute(), protocmp.Transform()); diff != "" {
		t.Errorf("UpdateMCPRoute(cleared name, reset paths) reply mismatch (-want +got):\n%s", diff)
	}

	response = requestResource(httpServer, http.MethodDelete, "/api/admin/v1/namespaces/team-a/mcp-routes/tools", "")
	requireHTTPStatus(t, response, http.StatusOK)
	if _, err := client.GatewayV1alpha1().MCPRoutes("team-a").Get(t.Context(), "tools", metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Errorf("Get(deleted MCPRoute) error = %v, want NotFound", err)
	}

	response = requestResource(httpServer, http.MethodDelete, "/api/admin/v1/namespaces/team-a/mcp-routes/tools", "")
	requireHTTPStatus(t, response, http.StatusNotFound)
}

func TestMCPRouteRequestValidationHTTP(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
	}{
		{name: "missing fields", body: `{}`},
		{name: "missing parent", body: `{"backendRefs":[{"name":"network"}]}`},
		{name: "missing backend", body: `{"parentRefs":[{"name":"edge"}]}`},
		{name: "missing backend name", body: `{"parentRefs":[{"name":"edge"}],"backendRefs":[{}]}`},
		{name: "multiple backends", body: `{"parentRefs":[{"name":"edge"}],"backendRefs":[{"name":"first"},{"name":"second"}]}`},
		{name: "empty entry path", body: `{"parentRefs":[{"name":"edge"}],"path":"","backendRefs":[{"name":"network"}]}`},
		{name: "empty upstream path", body: `{"parentRefs":[{"name":"edge"}],"backendRefs":[{"name":"network","path":""}]}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			httpServer, _, client := newTestServers()
			response := requestResource(httpServer, http.MethodPost, "/api/admin/v1/namespaces/team-a/mcp-routes", tt.body)
			requireHTTPStatus(t, response, http.StatusBadRequest)
			if actions := client.Actions(); len(actions) != 0 {
				t.Errorf("CreateMCPRoute(%s) API actions = %v, want none", tt.name, actions)
			}
		})
	}
}

func TestMCPRouteAuthenticationHTTP(t *testing.T) {
	calls := 0
	deny := func(kratosmiddleware.Handler) kratosmiddleware.Handler {
		return func(context.Context, any) (any, error) {
			calls++
			return nil, kratoserrors.Unauthorized("UNAUTHENTICATED", "未登录")
		}
	}
	httpServer, _, client := newTestServersWithAuthentication(deny)
	for _, tt := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/admin/v1/namespaces/team-a/mcp-routes", body: `{}`},
		{method: http.MethodGet, path: "/api/admin/v1/namespaces/team-a/mcp-routes/tools"},
		{method: http.MethodGet, path: "/api/admin/v1/namespaces/team-a/mcp-routes"},
		{method: http.MethodPut, path: "/api/admin/v1/namespaces/team-a/mcp-routes/tools", body: `{}`},
		{method: http.MethodDelete, path: "/api/admin/v1/namespaces/team-a/mcp-routes/tools"},
	} {
		response := requestResource(httpServer, tt.method, tt.path, tt.body)
		requireHTTPStatus(t, response, http.StatusUnauthorized)
	}

	if calls != 5 || len(client.Actions()) != 0 {
		t.Errorf("MCPRoute unauthenticated requests = %d auth calls and %v API actions, want 5 and none", calls, client.Actions())
	}
}

func TestMCPRouteQueryHTTP(t *testing.T) {
	httpServer, client, query := newQueryTestServer(t)
	createdAt := time.Date(2026, 10, 12, 1, 2, 3, 0, time.UTC)
	parentsJSON := `[{"name":"edge","sectionName":"z"},{"name":"edge","sectionName":"a"}]`
	columns := []string{"namespace", "name", "display_name", "parents", "path", "backend_name", "backend_path", "version", "created_at"}
	query.ExpectQuery("FROM `mcproutes` WHERE namespace = \\? AND name =").WithArgs("team-a", "tools", 1).
		WillReturnRows(sqlmock.NewRows(columns).AddRow("team-a", "tools", "工具路由", parentsJSON, "/工具/%2F/", "network", "/", "9", createdAt))
	response := requestResource(httpServer, http.MethodGet, "/api/admin/v1/namespaces/team-a/mcp-routes/tools", "")
	requireHTTPStatus(t, response, http.StatusOK)
	var getReply adminv1.GetMCPRouteReply
	decodeReply(t, response, &getReply)
	want := &adminv1.MCPRoute{
		Namespace:   "team-a",
		Name:        "tools",
		DisplayName: "工具路由",
		ParentRefs: []*adminv1.ParentReference{
			{Name: "edge", SectionName: "z"},
			{Name: "edge", SectionName: "a"},
		},
		Path:        ptr.To("/工具/%2F/"),
		BackendRefs: []*adminv1.MCPBackendReference{{Name: "network", Path: ptr.To("/")}},
		CreateTime:  timestamppb.New(createdAt),
		Version:     "9",
	}
	if diff := cmp.Diff(want, getReply.GetRoute(), protocmp.Transform()); diff != "" {
		t.Errorf("GetMCPRoute(tools) mismatch (-want +got):\n%s", diff)
	}

	condition := "JSON_CONTAINS(parents, JSON_OBJECT('name', ?))"
	query.ExpectBegin()
	query.ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `mcproutes` WHERE namespace = ? AND "+condition)).
		WithArgs("team-a", "edge").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	query.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `mcproutes` WHERE namespace = ? AND "+condition+" ORDER BY name LIMIT ?")).
		WithArgs("team-a", "edge", 50).WillReturnRows(sqlmock.NewRows(columns).AddRow("team-a", "tools", "工具路由", parentsJSON, "/工具/%2F/", "network", "/", "9", createdAt))
	query.ExpectCommit()
	response = requestResource(httpServer, http.MethodGet, "/api/admin/v1/namespaces/team-a/mcp-routes?gatewayName=edge", "")
	requireHTTPStatus(t, response, http.StatusOK)
	var listReply adminv1.ListMCPRoutesReply
	decodeReply(t, response, &listReply)
	if diff := cmp.Diff(&adminv1.ListMCPRoutesReply{Routes: []*adminv1.MCPRoute{want}, TotalCount: 1}, &listReply, protocmp.Transform()); diff != "" {
		t.Errorf("ListMCPRoutes(default page, Gateway filter) mismatch (-want +got):\n%s", diff)
	}

	query.ExpectQuery("FROM `mcproutes` WHERE namespace = \\? AND name =").WithArgs("team-a", "missing", 1).WillReturnRows(sqlmock.NewRows(columns))
	response = requestResource(httpServer, http.MethodGet, "/api/admin/v1/namespaces/team-a/mcp-routes/missing", "")
	requireHTTPStatus(t, response, http.StatusNotFound)
	if actions := client.Actions(); len(actions) != 0 {
		t.Errorf("Get/ListMCPRoutes() API actions = %v, want query model only", actions)
	}
}

func TestMCPRouteGRPC(t *testing.T) {
	connection, backend := newTestGRPCConnection(t)
	client := adminv1.NewMCPRouteServiceClient(connection)
	if _, err := client.CreateMCPRoute(t.Context(), &adminv1.CreateMCPRouteRequest{Namespace: "team-a"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("CreateMCPRoute(empty) error = %v, want InvalidArgument", err)
	}

	if actions := backend.Actions(); len(actions) != 0 {
		t.Errorf("CreateMCPRoute(empty) API actions = %v, want none", actions)
	}

	request := &adminv1.CreateMCPRouteRequest{
		Namespace:   "team-a",
		DisplayName: "工具路由",
		ParentRefs:  []*adminv1.ParentReference{{Name: "edge"}},
		BackendRefs: []*adminv1.MCPBackendReference{{Name: "network"}},
	}
	first, err := client.CreateMCPRoute(t.Context(), request)
	if err != nil {
		t.Fatalf("CreateMCPRoute(first) error = %v, want nil", err)
	}

	second, err := client.CreateMCPRoute(t.Context(), request)
	if err != nil {
		t.Fatalf("CreateMCPRoute(duplicate display name) error = %v, want nil", err)
	}

	if first.GetRoute().GetName() == second.GetRoute().GetName() || second.GetRoute().GetDisplayName() != "工具路由" {
		t.Errorf("CreateMCPRoute(duplicate display name) = %v, want distinct ID and same display name", second)
	}

	resource := gatewayv1alpha1.SchemeGroupVersion.WithResource("mcproutes").GroupResource()
	backend.PrependReactor("create", "mcproutes", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewAlreadyExists(resource, "generated-id")
	})
	if _, err := client.CreateMCPRoute(t.Context(), request); status.Code(err) != codes.Aborted || kratoserrors.Reason(err) != "MCP_ROUTE_ALREADY_EXISTS" {
		t.Errorf("CreateMCPRoute(identity conflict) = %v, want Aborted/MCP_ROUTE_ALREADY_EXISTS", err)
	}

	backend.PrependReactor("create", "mcproutes", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewInvalid(schema.GroupKind{Group: gatewayv1alpha1.GroupName, Kind: "MCPRoute"}, "tools", field.ErrorList{
			field.Invalid(field.NewPath("spec", "path"), "relative", "must be absolute"),
		})
	})
	if _, err := client.CreateMCPRoute(t.Context(), request); status.Code(err) != codes.InvalidArgument || kratoserrors.Reason(err) != "MCP_ROUTE_INVALID_ARGUMENT" {
		t.Errorf("CreateMCPRoute(API validation error) = %v, want InvalidArgument/MCP_ROUTE_INVALID_ARGUMENT", err)
	}

	backend.PrependReactor("create", "mcproutes", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("private storage failure")
	})
	if _, err := client.CreateMCPRoute(t.Context(), request); status.Code(err) != codes.Internal || status.Convert(err).Message() != "服务器内部错误" {
		t.Errorf("CreateMCPRoute(storage failure) = %v, want Internal with safe public message", err)
	}
}

func TestMCPRoutePathLengthGRPC(t *testing.T) {
	for _, tt := range []struct {
		name string
		path string
		want codes.Code
	}{
		{name: "unicode at limit", path: "/" + strings.Repeat("工", 1023), want: codes.OK},
		{name: "unicode over limit", path: "/" + strings.Repeat("工", 1024), want: codes.InvalidArgument},
		{name: "ascii over limit", path: "/" + strings.Repeat("a", 1024), want: codes.InvalidArgument},
		{name: "explicit empty", want: codes.InvalidArgument},
	} {
		for _, upstream := range []bool{false, true} {
			t.Run(tt.name+"/upstream="+strconv.FormatBool(upstream), func(t *testing.T) {
				connection, backend := newTestGRPCConnection(t)
				request := &adminv1.CreateMCPRouteRequest{
					Namespace:   "team-a",
					ParentRefs:  []*adminv1.ParentReference{{Name: "edge"}},
					BackendRefs: []*adminv1.MCPBackendReference{{Name: "network"}},
				}
				if upstream {
					request.BackendRefs[0].Path = ptr.To(tt.path)
				} else {
					request.Path = ptr.To(tt.path)
				}

				_, err := adminv1.NewMCPRouteServiceClient(connection).CreateMCPRoute(t.Context(), request)
				if status.Code(err) != tt.want {
					t.Errorf("CreateMCPRoute(path length) error = %v, want %v", err, tt.want)
				}

				if tt.want != codes.OK && len(backend.Actions()) != 0 {
					t.Errorf("CreateMCPRoute(invalid path) API actions = %v, want none", backend.Actions())
				}
			})
		}
	}
}
