package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lgc202/ingate/internal/adminapi/repository/mysql"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	clientfake "github.com/lgc202/ingate/pkg/generated/clientset/versioned/fake"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"
)

func TestGatewayCreate(t *testing.T) {
	client := clientfake.NewSimpleClientset()
	gateways := NewGateway(client.GatewayV1alpha1().Gateways(), nil)

	created, err := gateways.Create(context.Background(), &gatewayv1alpha1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "public-gateway"}})
	if err != nil {
		t.Fatalf("Gateway.Create(public-gateway) error = %v, want nil", err)
	}
	if created.Name != "public-gateway" {
		t.Errorf("Gateway.Create(public-gateway) name = %q, want public-gateway", created.Name)
	}
}

func TestGatewayCreateAlreadyExists(t *testing.T) {
	client := clientfake.NewSimpleClientset()
	resource := gatewayv1alpha1.SchemeGroupVersion.WithResource("gateways").GroupResource()
	storageErr := apierrors.NewAlreadyExists(resource, "public-gateway")
	client.PrependReactor("create", "gateways", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, storageErr
	})
	gateways := NewGateway(client.GatewayV1alpha1().Gateways(), nil)

	_, err := gateways.Create(context.Background(), &gatewayv1alpha1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "public-gateway"}})
	if !errors.Is(err, ErrAlreadyExists) {
		t.Errorf("Gateway.Create(public-gateway) error = %v, want ErrAlreadyExists", err)
	}
}

func TestGatewayReadsQueryModelAndUpdatesAPIServer(t *testing.T) {
	ctx := context.Background()
	client := clientfake.NewSimpleClientset()
	if _, err := client.GatewayV1alpha1().Gateways().Create(ctx, &gatewayv1alpha1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: "edge", ResourceVersion: "8"},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("seed API Server Gateway: %v", err)
	}

	client.ClearActions()
	db, query, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		query.ExpectClose()
		if err := db.Close(); err != nil {
			t.Errorf("Close() error = %v, want nil", err)
		}
		if err := query.ExpectationsWereMet(); err != nil {
			t.Errorf("SQL expectations error = %v, want nil", err)
		}
	})

	gormDB, err := gorm.Open(gormmysql.New(gormmysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v, want nil", err)
	}
	store := mysql.New(gormDB)
	gateways := NewGateway(client.GatewayV1alpha1().Gateways(), store)

	query.ExpectBegin()
	query.ExpectQuery("FROM `gateways` WHERE name = ").WillReturnRows(
		sqlmock.NewRows([]string{"id", "name", "version"}).AddRow(1, "edge", "7"),
	)
	query.ExpectQuery("FROM `gateway_listeners` WHERE gateway_id =").WillReturnRows(
		sqlmock.NewRows([]string{"id", "gateway_id", "name", "protocol", "port", "hostname"}),
	)
	query.ExpectCommit()

	got, err := gateways.Get(ctx, "edge")
	if err != nil {
		t.Fatalf("Get Gateway from query model: %v", err)
	}
	if got.Version != "7" || len(client.Actions()) != 0 {
		t.Errorf("Get = version %q with %d API actions, want version 7 and no API actions", got.Version, len(client.Actions()))
	}

	query.ExpectBegin()
	query.ExpectQuery("SELECT count\\(\\*\\) FROM `gateways`").WillReturnRows(
		sqlmock.NewRows([]string{"count"}).AddRow(1),
	)
	query.ExpectQuery("FROM `gateways` ORDER BY name LIMIT").WillReturnRows(
		sqlmock.NewRows([]string{"id", "name", "version"}).AddRow(1, "edge", "7"),
	)
	query.ExpectQuery("FROM `gateway_listeners` WHERE gateway_id IN").WillReturnRows(
		sqlmock.NewRows([]string{"id", "gateway_id", "name", "protocol", "port", "hostname"}),
	)
	query.ExpectCommit()

	page, err := gateways.List(ctx, ResourceListParams{PageSize: 20, PageNum: 1})
	if err != nil || page.TotalCount != 1 || len(page.Items) != 1 || len(client.Actions()) != 0 {
		t.Errorf("List = %d items, total %d, err %v, API actions %d; want 1, 1, nil, 0", len(page.Items), page.TotalCount, err, len(client.Actions()))
	}

	_, err = gateways.Update(ctx, &gatewayv1alpha1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "edge", ResourceVersion: "7"}})
	if !errors.Is(err, ErrConflict) {
		t.Errorf("Update with stale projected version error = %v, want ErrConflict", err)
	}
	if len(client.Actions()) != 1 || client.Actions()[0].GetVerb() != "get" {
		t.Errorf("Update API actions = %v, want one current-object Get", client.Actions())
	}
}
