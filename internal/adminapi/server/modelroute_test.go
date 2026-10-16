package server_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
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

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestCreateModelRouteReturnsSavedStateHTTP(t *testing.T) {
	httpServer, _, client := newTestServers()
	createdAt := time.Date(2026, 10, 12, 1, 2, 3, 0, time.UTC)
	client.PrependReactor("create", "modelroutes", func(action clienttesting.Action) (bool, runtime.Object, error) {
		route := action.(clienttesting.CreateAction).GetObject().(*gatewayv1alpha1.ModelRoute).DeepCopy()
		gatewayv1alpha1.SetDefaultsModelRoute(route)
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
	clientAPI := adminv1.NewModelRouteServiceHTTPClient(httpClient)
	request := &adminv1.CreateModelRouteRequest{
		Namespace:   "team-a",
		DisplayName: "模型路由",
		ParentRefs: []*adminv1.ParentReference{
			{Name: "edge", SectionName: "z"},
			{Name: "edge", SectionName: "a"},
		},
		Rules: []*adminv1.ModelRouteRule{
			{
				Name: "z-rule",
				Matches: []*adminv1.ModelRouteMatch{
					{Headers: []*adminv1.HTTPHeaderMatch{{Name: "X-Ai-Eg-Model", Value: " Model-Z "}}},
					{},
				},
				BackendRefs: []*adminv1.AIBackendReference{{Name: "models-z", ModelNameOverride: "provider-model"}},
			},
			{Name: "a-rule", BackendRefs: []*adminv1.AIBackendReference{{Name: "models-a"}}},
		},
	}
	reply, err := clientAPI.CreateModelRoute(t.Context(), request)
	if err != nil {
		t.Fatalf("CreateModelRoute(HTTP round trip) error = %v, want nil", err)
	}

	name := reply.GetRoute().GetName()
	if !strings.HasPrefix(name, "modelroute-") {
		t.Errorf("CreateModelRoute().name = %q, want generated modelroute ID", name)
	}

	want := &adminv1.ModelRoute{
		Namespace:   "team-a",
		Name:        name,
		DisplayName: request.DisplayName,
		ParentRefs:  request.ParentRefs,
		Rules: []*adminv1.ModelRouteRule{
			{
				Name: "z-rule",
				Matches: []*adminv1.ModelRouteMatch{
					{Headers: []*adminv1.HTTPHeaderMatch{{
						Name:  "X-Ai-Eg-Model",
						Type:  "Exact",
						Value: " Model-Z ",
					}}},
					{},
				},
				BackendRefs: request.Rules[0].BackendRefs,
			},
			request.Rules[1],
		},
		CreateTime: timestamppb.New(createdAt),
		Version:    "9",
	}
	if diff := cmp.Diff(want, reply.GetRoute(), protocmp.Transform()); diff != "" {
		t.Errorf("CreateModelRoute(saved defaults, order and catch-all) mismatch (-want +got):\n%s", diff)
	}
}

func TestModelRouteUpdateAndDeleteHTTP(t *testing.T) {
	seed := &gatewayv1alpha1.ModelRoute{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "team-a",
			Name:            "models",
			UID:             types.UID("route-instance"),
			ResourceVersion: "7",
			Labels:          map[string]string{"team": "core"},
			Finalizers:      []string{"gateway.ingate.io/cleanup"},
		},
		Spec: gatewayv1alpha1.ModelRouteSpec{
			DisplayName: "旧名称",
			ParentRefs:  []gatewayv1alpha1.ParentReference{{Name: "edge", SectionName: "http"}},
			Rules: []gatewayv1alpha1.ModelRouteRule{{
				Name:        "old-rule",
				BackendRefs: []gatewayv1alpha1.AIBackendReference{{Name: "old-backend", ModelNameOverride: "old-model"}},
			}},
		},
		Status: gatewayv1alpha1.ModelRouteStatus{Parents: []gatewayv1alpha1.RouteParentStatus{{ControllerName: "gateway.ingate.io/test"}}},
	}
	httpServer, client := crudTestServer(t, seed)
	stale := `{"version":"stale","parentRefs":[{"name":"other"}],"rules":[{"matches":[{}],"backendRefs":[{"name":"other-models"}]}]}`
	response := requestResource(httpServer, http.MethodPut, "/api/admin/v1/namespaces/team-a/model-routes/models", stale)
	requireHTTPStatus(t, response, http.StatusConflict)
	stored, err := client.GatewayV1alpha1().ModelRoutes("team-a").Get(t.Context(), "models", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(ModelRoute after stale update) error = %v, want nil", err)
	}

	if diff := cmp.Diff(seed, stored); diff != "" {
		t.Errorf("UpdateModelRoute(stale) changed state (-want +got):\n%s", diff)
	}

	body := `{"version":"7","parentRefs":[{"name":"other"}],"rules":[{"matches":[{}],"backendRefs":[{"name":"other-models"}]}]}`
	response = requestResource(httpServer, http.MethodPut, "/api/admin/v1/namespaces/team-a/model-routes/models", body)
	requireHTTPStatus(t, response, http.StatusOK)
	stored, err = client.GatewayV1alpha1().ModelRoutes("team-a").Get(t.Context(), "models", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(updated ModelRoute) error = %v, want nil", err)
	}

	want := seed.DeepCopy()
	want.Spec = gatewayv1alpha1.ModelRouteSpec{
		ParentRefs: []gatewayv1alpha1.ParentReference{{Name: "other"}},
		Rules: []gatewayv1alpha1.ModelRouteRule{{
			Matches:     []gatewayv1alpha1.ModelRouteMatch{{}},
			BackendRefs: []gatewayv1alpha1.AIBackendReference{{Name: "other-models"}},
		}},
	}
	if diff := cmp.Diff(want, stored); diff != "" {
		t.Errorf("UpdateModelRoute(full PUT, metadata/status preserved) mismatch (-want +got):\n%s", diff)
	}

	var reply adminv1.UpdateModelRouteReply
	decodeReply(t, response, &reply)
	wantReply := &adminv1.ModelRoute{
		Namespace:  "team-a",
		Name:       "models",
		ParentRefs: []*adminv1.ParentReference{{Name: "other"}},
		Rules: []*adminv1.ModelRouteRule{{
			Matches:     []*adminv1.ModelRouteMatch{{}},
			BackendRefs: []*adminv1.AIBackendReference{{Name: "other-models"}},
		}},
		CreateTime: timestamppb.New(seed.CreationTimestamp.Time),
		Version:    "7",
	}
	if diff := cmp.Diff(wantReply, reply.GetRoute(), protocmp.Transform()); diff != "" {
		t.Errorf("UpdateModelRoute(cleared fields) reply mismatch (-want +got):\n%s", diff)
	}

	response = requestResource(httpServer, http.MethodDelete, "/api/admin/v1/namespaces/team-a/model-routes/models", "")
	requireHTTPStatus(t, response, http.StatusOK)
	if _, err := client.GatewayV1alpha1().ModelRoutes("team-a").Get(t.Context(), "models", metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Errorf("Get(deleted ModelRoute) error = %v, want NotFound", err)
	}

	response = requestResource(httpServer, http.MethodDelete, "/api/admin/v1/namespaces/team-a/model-routes/models", "")
	requireHTTPStatus(t, response, http.StatusNotFound)
}

func TestModelRouteRequestValidationHTTP(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
	}{
		{name: "missing fields", body: `{}`},
		{name: "missing parent", body: `{"rules":[{"backendRefs":[{"name":"models"}]}]}`},
		{name: "missing backend", body: `{"parentRefs":[{"name":"edge"}],"rules":[{}]}`},
		{name: "wrong header", body: `{"parentRefs":[{"name":"edge"}],"rules":[{"matches":[{"headers":[{"name":"other","value":"model"}]}],"backendRefs":[{"name":"models"}]}]}`},
		{name: "regex", body: `{"parentRefs":[{"name":"edge"}],"rules":[{"matches":[{"headers":[{"name":"x-ai-eg-model","type":"RegularExpression","value":"model"}]}],"backendRefs":[{"name":"models"}]}]}`},
		{name: "empty model", body: `{"parentRefs":[{"name":"edge"}],"rules":[{"matches":[{"headers":[{"name":"x-ai-eg-model","value":""}]}],"backendRefs":[{"name":"models"}]}]}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			httpServer, _, client := newTestServers()
			response := requestResource(httpServer, http.MethodPost, "/api/admin/v1/namespaces/team-a/model-routes", tt.body)
			requireHTTPStatus(t, response, http.StatusBadRequest)
			if actions := client.Actions(); len(actions) != 0 {
				t.Errorf("CreateModelRoute(%s) API actions = %v, want none", tt.name, actions)
			}
		})
	}
}

func TestModelRouteAuthenticationHTTP(t *testing.T) {
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
		{method: http.MethodPost, path: "/api/admin/v1/namespaces/team-a/model-routes", body: `{}`},
		{method: http.MethodGet, path: "/api/admin/v1/namespaces/team-a/model-routes/models"},
		{method: http.MethodGet, path: "/api/admin/v1/namespaces/team-a/model-routes"},
		{method: http.MethodPut, path: "/api/admin/v1/namespaces/team-a/model-routes/models", body: `{}`},
		{method: http.MethodDelete, path: "/api/admin/v1/namespaces/team-a/model-routes/models"},
	} {
		response := requestResource(httpServer, tt.method, tt.path, tt.body)
		requireHTTPStatus(t, response, http.StatusUnauthorized)
	}

	if calls != 5 || len(client.Actions()) != 0 {
		t.Errorf("ModelRoute unauthenticated requests = %d auth calls and %v API actions, want 5 and none", calls, client.Actions())
	}
}

func TestModelRouteQueryHTTP(t *testing.T) {
	httpServer, client, query := newQueryTestServer(t)
	createdAt := time.Date(2026, 10, 12, 1, 2, 3, 0, time.UTC)
	parentsJSON := `[{"name":"edge","sectionName":"z"},{"name":"edge","sectionName":"a"}]`
	rulesJSON := `[{"name":"z-rule","matches":[{"headers":[{"name":"X-Ai-Eg-Model","type":"Exact","value":" Model-Z "}]},{}],"backendRefs":[{"name":"models-z","modelNameOverride":"provider-model"}]},{"name":"a-rule","matches":null,"backendRefs":[{"name":"models-a"}]}]`
	columns := []string{"namespace", "name", "display_name", "parents", "rules", "version", "created_at"}
	query.ExpectQuery("FROM `modelroutes` WHERE namespace = \\? AND name =").WithArgs("team-a", "models", 1).
		WillReturnRows(sqlmock.NewRows(columns).AddRow("team-a", "models", "模型路由", parentsJSON, rulesJSON, "9", createdAt))
	response := requestResource(httpServer, http.MethodGet, "/api/admin/v1/namespaces/team-a/model-routes/models", "")
	requireHTTPStatus(t, response, http.StatusOK)
	var getReply adminv1.GetModelRouteReply
	decodeReply(t, response, &getReply)
	want := &adminv1.ModelRoute{
		Namespace:   "team-a",
		Name:        "models",
		DisplayName: "模型路由",
		ParentRefs: []*adminv1.ParentReference{
			{Name: "edge", SectionName: "z"},
			{Name: "edge", SectionName: "a"},
		},
		Rules: []*adminv1.ModelRouteRule{
			{
				Name: "z-rule",
				Matches: []*adminv1.ModelRouteMatch{
					{Headers: []*adminv1.HTTPHeaderMatch{{
						Name:  "X-Ai-Eg-Model",
						Type:  "Exact",
						Value: " Model-Z ",
					}}},
					{},
				},
				BackendRefs: []*adminv1.AIBackendReference{{Name: "models-z", ModelNameOverride: "provider-model"}},
			},
			{Name: "a-rule", BackendRefs: []*adminv1.AIBackendReference{{Name: "models-a"}}},
		},
		CreateTime: timestamppb.New(createdAt),
		Version:    "9",
	}
	if diff := cmp.Diff(want, getReply.GetRoute(), protocmp.Transform()); diff != "" {
		t.Errorf("GetModelRoute(models) mismatch (-want +got):\n%s", diff)
	}

	condition := "JSON_CONTAINS(parents, JSON_OBJECT('name', ?))"
	query.ExpectBegin()
	query.ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `modelroutes` WHERE namespace = ? AND "+condition)).
		WithArgs("team-a", "edge").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	query.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `modelroutes` WHERE namespace = ? AND "+condition+" ORDER BY name LIMIT ?")).
		WithArgs("team-a", "edge", 50).WillReturnRows(sqlmock.NewRows(columns).AddRow("team-a", "models", "模型路由", parentsJSON, rulesJSON, "9", createdAt))
	query.ExpectCommit()
	response = requestResource(httpServer, http.MethodGet, "/api/admin/v1/namespaces/team-a/model-routes?gatewayName=edge", "")
	requireHTTPStatus(t, response, http.StatusOK)
	var listReply adminv1.ListModelRoutesReply
	decodeReply(t, response, &listReply)
	wantPage := &adminv1.ListModelRoutesReply{Routes: []*adminv1.ModelRoute{want}, TotalCount: 1}
	if diff := cmp.Diff(wantPage, &listReply, protocmp.Transform()); diff != "" {
		t.Errorf("ListModelRoutes(default page, Gateway filter) mismatch (-want +got):\n%s", diff)
	}

	query.ExpectQuery("FROM `modelroutes` WHERE namespace = \\? AND name =").WithArgs("team-a", "missing", 1).
		WillReturnRows(sqlmock.NewRows(columns))
	response = requestResource(httpServer, http.MethodGet, "/api/admin/v1/namespaces/team-a/model-routes/missing", "")
	requireHTTPStatus(t, response, http.StatusNotFound)
	if actions := client.Actions(); len(actions) != 0 {
		t.Errorf("Get/ListModelRoutes() API actions = %v, want query model only", actions)
	}
}

func TestModelRouteGRPC(t *testing.T) {
	connection, backend := newTestGRPCConnection(t)
	client := adminv1.NewModelRouteServiceClient(connection)
	if _, err := client.CreateModelRoute(t.Context(), &adminv1.CreateModelRouteRequest{Namespace: "team-a"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("CreateModelRoute(empty) error = %v, want InvalidArgument", err)
	}

	if actions := backend.Actions(); len(actions) != 0 {
		t.Errorf("CreateModelRoute(empty) API actions = %v, want none", actions)
	}

	request := &adminv1.CreateModelRouteRequest{
		Namespace:   "team-a",
		DisplayName: "模型路由",
		ParentRefs:  []*adminv1.ParentReference{{Name: "edge"}},
		Rules:       []*adminv1.ModelRouteRule{{BackendRefs: []*adminv1.AIBackendReference{{Name: "models"}}}},
	}
	first, err := client.CreateModelRoute(t.Context(), request)
	if err != nil {
		t.Fatalf("CreateModelRoute(first) error = %v, want nil", err)
	}

	second, err := client.CreateModelRoute(t.Context(), request)
	if err != nil {
		t.Fatalf("CreateModelRoute(duplicate display name) error = %v, want nil", err)
	}

	if first.GetRoute().GetName() == second.GetRoute().GetName() || second.GetRoute().GetDisplayName() != "模型路由" {
		t.Errorf("CreateModelRoute(duplicate display name) = %v, want distinct ID and same display name", second)
	}

	resource := gatewayv1alpha1.SchemeGroupVersion.WithResource("modelroutes").GroupResource()
	backend.PrependReactor("create", "modelroutes", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewAlreadyExists(resource, "generated-id")
	})
	if _, err := client.CreateModelRoute(t.Context(), request); status.Code(err) != codes.Aborted || kratoserrors.Reason(err) != "MODEL_ROUTE_ALREADY_EXISTS" {
		t.Errorf("CreateModelRoute(identity conflict) = %v, want Aborted/MODEL_ROUTE_ALREADY_EXISTS", err)
	}

	backend.PrependReactor("create", "modelroutes", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewInvalid(schema.GroupKind{Group: gatewayv1alpha1.GroupName, Kind: "ModelRoute"}, "models", field.ErrorList{
			field.Duplicate(field.NewPath("spec", "parentRefs").Index(1), "edge"),
		})
	})
	if _, err := client.CreateModelRoute(t.Context(), request); status.Code(err) != codes.InvalidArgument || kratoserrors.Reason(err) != "MODEL_ROUTE_INVALID_ARGUMENT" {
		t.Errorf("CreateModelRoute(API validation error) = %v, want InvalidArgument/MODEL_ROUTE_INVALID_ARGUMENT", err)
	}

	backend.PrependReactor("create", "modelroutes", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("private storage failure")
	})
	if _, err := client.CreateModelRoute(t.Context(), request); status.Code(err) != codes.Internal || status.Convert(err).Message() != "服务器内部错误" {
		t.Errorf("CreateModelRoute(storage failure) = %v, want Internal with safe public message", err)
	}
}
