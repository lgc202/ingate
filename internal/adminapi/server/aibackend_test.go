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
	clienttesting "k8s.io/client-go/testing"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestCreateAIBackendReturnsSavedState(t *testing.T) {
	httpServer, _, client := newTestServers()
	createdAt := time.Date(2026, 10, 12, 1, 2, 3, 0, time.UTC)
	client.PrependReactor("create", "aibackends", func(action clienttesting.Action) (bool, runtime.Object, error) {
		backend := action.(clienttesting.CreateAction).GetObject().(*gatewayv1alpha1.AIBackend).DeepCopy()
		gatewayv1alpha1.SetDefaultsAIBackend(backend)
		backend.ResourceVersion = "9"
		backend.CreationTimestamp = metav1.NewTime(createdAt)
		return true, backend, nil
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
	clientAPI := adminv1.NewAIBackendServiceHTTPClient(httpClient)
	reply, err := clientAPI.CreateAIBackend(t.Context(), &adminv1.CreateAIBackendRequest{
		Namespace:   "team-a",
		DisplayName: "模型服务",
		BackendRef:  &adminv1.BackendReference{Name: "network"},
		Schema:      &adminv1.APISchema{Name: "OpenAI"},
	})
	if err != nil {
		t.Fatalf("CreateAIBackend(saved state HTTP round trip) error = %v, want nil", err)
	}
	name := reply.GetBackend().GetName()
	if !strings.HasPrefix(name, "aibackend-") {
		t.Errorf("CreateAIBackend().name = %q, want generated aibackend ID", name)
	}
	want := &adminv1.AIBackend{
		Namespace:   "team-a",
		Name:        name,
		DisplayName: "模型服务",
		BackendRef:  &adminv1.BackendReference{Name: "network"},
		Schema:      &adminv1.APISchema{Name: "OpenAI", Prefix: "/v1"},
		CreateTime:  timestamppb.New(createdAt),
		Version:     "9",
	}
	if diff := cmp.Diff(want, reply.GetBackend(), protocmp.Transform()); diff != "" {
		t.Errorf("CreateAIBackend(saved state) mismatch (-want +got):\n%s", diff)
	}
}

func TestAIBackendUpdateAndDeleteHTTP(t *testing.T) {
	seed := &gatewayv1alpha1.AIBackend{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "team-a",
			Name:            "models",
			UID:             types.UID("model-instance"),
			ResourceVersion: "7",
			Labels:          map[string]string{"team": "core"},
			Finalizers:      []string{"gateway.ingate.io/cleanup"},
		},
		Spec: gatewayv1alpha1.AIBackendSpec{
			DisplayName: "旧名称",
			BackendRef:  gatewayv1alpha1.BackendReference{Name: "network"},
			Schema:      gatewayv1alpha1.APISchema{Name: gatewayv1alpha1.APISchemaOpenAI, Prefix: "/v1"},
		},
		Status: gatewayv1alpha1.AIBackendStatus{Conditions: []metav1.Condition{{Type: "Accepted", Status: metav1.ConditionTrue}}},
	}
	httpServer, client := crudTestServer(t, seed)
	stale := `{"version":"stale","backendRef":{"name":"other-network"},"schema":{"name":"OpenAI","prefix":"/compat/v1"}}`
	response := requestResource(httpServer, http.MethodPut, "/api/admin/v1/namespaces/team-a/ai-backends/models", stale)
	requireHTTPStatus(t, response, http.StatusConflict)
	stored, err := client.GatewayV1alpha1().AIBackends("team-a").Get(t.Context(), "models", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(AIBackend after stale update) error = %v, want nil", err)
	}
	if diff := cmp.Diff(seed, stored); diff != "" {
		t.Errorf("UpdateAIBackend(stale) changed state (-want +got):\n%s", diff)
	}

	body := `{"version":"7","backendRef":{"name":"other-network"},"schema":{"name":"OpenAI","prefix":"/compat/v1"}}`
	response = requestResource(httpServer, http.MethodPut, "/api/admin/v1/namespaces/team-a/ai-backends/models", body)
	requireHTTPStatus(t, response, http.StatusOK)
	var reply adminv1.UpdateAIBackendReply
	decodeReply(t, response, &reply)
	stored, err = client.GatewayV1alpha1().AIBackends("team-a").Get(t.Context(), "models", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(updated AIBackend) error = %v, want nil", err)
	}
	want := seed.DeepCopy()
	want.Spec = gatewayv1alpha1.AIBackendSpec{
		BackendRef: gatewayv1alpha1.BackendReference{Name: "other-network"},
		Schema:     gatewayv1alpha1.APISchema{Name: gatewayv1alpha1.APISchemaOpenAI, Prefix: "/compat/v1"},
	}
	if diff := cmp.Diff(want, stored); diff != "" {
		t.Errorf("UpdateAIBackend(full spec, metadata/status preserved) mismatch (-want +got):\n%s", diff)
	}
	if got := reply.GetBackend(); got.GetDisplayName() != "" || got.GetBackendRef().GetName() != "other-network" || got.GetSchema().GetPrefix() != "/compat/v1" {
		t.Errorf("UpdateAIBackend().backend = %v, want cleared display name and changed reference/prefix", got)
	}

	response = requestResource(httpServer, http.MethodDelete, "/api/admin/v1/namespaces/team-a/ai-backends/models", "")
	requireHTTPStatus(t, response, http.StatusOK)
	if _, err := client.GatewayV1alpha1().AIBackends("team-a").Get(t.Context(), "models", metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Errorf("Get(deleted AIBackend) error = %v, want NotFound", err)
	}
	response = requestResource(httpServer, http.MethodDelete, "/api/admin/v1/namespaces/team-a/ai-backends/models", "")
	requireHTTPStatus(t, response, http.StatusNotFound)
}

func TestAIBackendRequestValidationHTTP(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
	}{
		{name: "missing fields", body: `{}`},
		{name: "missing reference", body: `{"schema":{"name":"OpenAI"}}`},
		{name: "empty reference", body: `{"backendRef":{},"schema":{"name":"OpenAI"}}`},
		{name: "missing schema", body: `{"backendRef":{"name":"network"}}`},
		{name: "unsupported schema", body: `{"backendRef":{"name":"network"},"schema":{"name":"Anthropic"}}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			httpServer, _, client := newTestServers()
			response := requestResource(httpServer, http.MethodPost, "/api/admin/v1/namespaces/team-a/ai-backends", tt.body)
			requireHTTPStatus(t, response, http.StatusBadRequest)
			if actions := client.Actions(); len(actions) != 0 {
				t.Errorf("CreateAIBackend(%s) API actions = %v, want none", tt.name, actions)
			}
		})
	}
}

func TestAIBackendAuthenticationHTTP(t *testing.T) {
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
		{method: http.MethodPost, path: "/api/admin/v1/namespaces/team-a/ai-backends", body: `{}`},
		{method: http.MethodGet, path: "/api/admin/v1/namespaces/team-a/ai-backends/models"},
		{method: http.MethodGet, path: "/api/admin/v1/namespaces/team-a/ai-backends"},
		{method: http.MethodPut, path: "/api/admin/v1/namespaces/team-a/ai-backends/models", body: `{}`},
		{method: http.MethodDelete, path: "/api/admin/v1/namespaces/team-a/ai-backends/models"},
	} {
		response := requestResource(httpServer, tt.method, tt.path, tt.body)
		requireHTTPStatus(t, response, http.StatusUnauthorized)
	}
	if calls != 5 || len(client.Actions()) != 0 {
		t.Errorf("AIBackend unauthenticated requests = %d auth calls and %v API actions, want 5 and none", calls, client.Actions())
	}
}

func TestAIBackendQueryHTTP(t *testing.T) {
	httpServer, client, query := newQueryTestServer(t)
	createdAt := time.Date(2026, 10, 12, 1, 2, 3, 0, time.UTC)
	columns := []string{"namespace", "name", "display_name", "backend_name", "schema_name", "schema_prefix", "version", "created_at"}
	query.ExpectQuery("FROM `aibackends` WHERE namespace = \\? AND name =").WithArgs("team-a", "models", 1).
		WillReturnRows(sqlmock.NewRows(columns).AddRow("team-a", "models", "模型服务", "network", "OpenAI", "/v1", "9", createdAt))
	response := requestResource(httpServer, http.MethodGet, "/api/admin/v1/namespaces/team-a/ai-backends/models", "")
	requireHTTPStatus(t, response, http.StatusOK)
	var getReply adminv1.GetAIBackendReply
	decodeReply(t, response, &getReply)
	want := &adminv1.AIBackend{
		Namespace:   "team-a",
		Name:        "models",
		DisplayName: "模型服务",
		BackendRef:  &adminv1.BackendReference{Name: "network"},
		Schema:      &adminv1.APISchema{Name: "OpenAI", Prefix: "/v1"},
		CreateTime:  timestamppb.New(createdAt),
		Version:     "9",
	}
	if diff := cmp.Diff(want, getReply.GetBackend(), protocmp.Transform()); diff != "" {
		t.Errorf("GetAIBackend(models) mismatch (-want +got):\n%s", diff)
	}

	query.ExpectBegin()
	query.ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `aibackends` WHERE namespace = ?")).WithArgs("team-a").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	query.ExpectQuery("FROM `aibackends` WHERE namespace = \\? ORDER BY name LIMIT").WithArgs("team-a", 50).
		WillReturnRows(sqlmock.NewRows(columns).AddRow("team-a", "models", "模型服务", "network", "OpenAI", "/v1", "9", createdAt))
	query.ExpectCommit()
	response = requestResource(httpServer, http.MethodGet, "/api/admin/v1/namespaces/team-a/ai-backends", "")
	requireHTTPStatus(t, response, http.StatusOK)
	var listReply adminv1.ListAIBackendsReply
	decodeReply(t, response, &listReply)
	if diff := cmp.Diff(&adminv1.ListAIBackendsReply{Backends: []*adminv1.AIBackend{want}, TotalCount: 1}, &listReply, protocmp.Transform()); diff != "" {
		t.Errorf("ListAIBackends(default page) mismatch (-want +got):\n%s", diff)
	}

	query.ExpectQuery("FROM `aibackends` WHERE namespace = \\? AND name =").WithArgs("team-a", "missing", 1).
		WillReturnRows(sqlmock.NewRows(columns))
	response = requestResource(httpServer, http.MethodGet, "/api/admin/v1/namespaces/team-a/ai-backends/missing", "")
	requireHTTPStatus(t, response, http.StatusNotFound)
	if actions := client.Actions(); len(actions) != 0 {
		t.Errorf("Get/ListAIBackends() API actions = %v, want query model only", actions)
	}
}

func TestAIBackendGRPC(t *testing.T) {
	connection, backend := newTestGRPCConnection(t)
	client := adminv1.NewAIBackendServiceClient(connection)
	if _, err := client.CreateAIBackend(t.Context(), &adminv1.CreateAIBackendRequest{Namespace: "team-a"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("CreateAIBackend(empty) error = %v, want InvalidArgument", err)
	}
	if actions := backend.Actions(); len(actions) != 0 {
		t.Errorf("CreateAIBackend(empty) API actions = %v, want none", actions)
	}
	request := &adminv1.CreateAIBackendRequest{
		Namespace:   "team-a",
		DisplayName: "模型服务",
		BackendRef:  &adminv1.BackendReference{Name: "network"},
		Schema:      &adminv1.APISchema{Name: "OpenAI", Prefix: "/v1"},
	}
	first, err := client.CreateAIBackend(t.Context(), request)
	if err != nil {
		t.Fatalf("CreateAIBackend(first) error = %v, want nil", err)
	}
	second, err := client.CreateAIBackend(t.Context(), request)
	if err != nil {
		t.Fatalf("CreateAIBackend(duplicate display name) error = %v, want nil", err)
	}
	if first.GetBackend().GetName() == second.GetBackend().GetName() || second.GetBackend().GetDisplayName() != "模型服务" {
		t.Errorf("CreateAIBackend(duplicate display name) = %v, want distinct ID and same display name", second)
	}

	resource := schema.GroupResource{Group: gatewayv1alpha1.GroupName, Resource: "aibackends"}
	storageErr := apierrors.NewAlreadyExists(resource, "generated-id")
	backend.PrependReactor("create", "aibackends", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, storageErr
	})
	if _, err := client.CreateAIBackend(t.Context(), request); status.Code(err) != codes.Aborted || kratoserrors.Reason(err) != "AI_BACKEND_ALREADY_EXISTS" {
		t.Errorf("CreateAIBackend(identity conflict) error = %v, want Aborted/AI_BACKEND_ALREADY_EXISTS", err)
	}
	backend.PrependReactor("create", "aibackends", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("private storage failure")
	})
	if _, err := client.CreateAIBackend(t.Context(), request); status.Code(err) != codes.Internal || status.Convert(err).Message() != "服务器内部错误" {
		t.Errorf("CreateAIBackend(storage failure) error = %v, want Internal with safe public message", err)
	}
}
