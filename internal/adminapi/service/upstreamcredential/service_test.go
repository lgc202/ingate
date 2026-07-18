package upstreamcredential

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	upstreamstore "github.com/lgc202/ingate/internal/adminapi/store/upstream"
	credentialstore "github.com/lgc202/ingate/internal/adminapi/store/upstreamcredential"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientfake "github.com/lgc202/ingate/pkg/generated/clientset/versioned/fake"
)

func TestServiceUpdatePreservesAPIKeyWhenOmitted(t *testing.T) {
	credential := &resource.UpstreamCredential{
		ObjectMeta: metav1.ObjectMeta{Name: "credential-1", ResourceVersion: "1"},
		Spec: resource.UpstreamCredentialSpec{
			DisplayName: "OpenAI old",
			Type:        resource.UpstreamCredentialTypeAPIKey,
			APIKey:      &resource.APIKeyCredential{Value: "existing-secret"},
		},
	}
	client := clientfake.NewSimpleClientset(credential)
	service := New(credentialstore.New(client), upstreamstore.New(client))

	err := service.Update(context.Background(), credential.Name, UpdateParams{
		Version: "1",
		CredentialParams: CredentialParams{
			Name: "OpenAI production",
			Type: resource.UpstreamCredentialTypeAPIKey,
		},
	})
	if err != nil {
		t.Fatalf("Service.Update(%q) error = %v, want nil", credential.Name, err)
	}
	updated, err := client.GatewayV1().UpstreamCredentials().Get(context.Background(), credential.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("UpstreamCredentials.Get(%q) error = %v, want nil", credential.Name, err)
	}
	if updated.Spec.APIKey == nil {
		t.Fatalf("Service.Update(%q) API key = nil, want configured", credential.Name)
	}
	if updated.Spec.APIKey.Value != "existing-secret" {
		t.Errorf("Service.Update(%q) changed the existing API key", credential.Name)
	}
}

func TestServiceUpdateReplacesAPIKey(t *testing.T) {
	credential := &resource.UpstreamCredential{
		ObjectMeta: metav1.ObjectMeta{Name: "credential-1", ResourceVersion: "1"},
		Spec: resource.UpstreamCredentialSpec{
			DisplayName: "OpenAI production",
			Type:        resource.UpstreamCredentialTypeAPIKey,
			APIKey:      &resource.APIKeyCredential{Value: "existing-secret"},
		},
	}
	client := clientfake.NewSimpleClientset(credential)
	service := New(credentialstore.New(client), upstreamstore.New(client))

	err := service.Update(context.Background(), credential.Name, UpdateParams{
		Version: "1",
		CredentialParams: CredentialParams{
			Name:        credential.Spec.DisplayName,
			Type:        resource.UpstreamCredentialTypeAPIKey,
			APIKeyValue: "rotated-secret",
		},
	})
	if err != nil {
		t.Fatalf("Service.Update(%q) error = %v, want nil", credential.Name, err)
	}
	updated, err := client.GatewayV1().UpstreamCredentials().Get(context.Background(), credential.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("UpstreamCredentials.Get(%q) error = %v, want nil", credential.Name, err)
	}
	if updated.Spec.APIKey == nil {
		t.Fatalf("Service.Update(%q) API key = nil, want configured", credential.Name)
	}
	if updated.Spec.APIKey.Value != "rotated-secret" {
		t.Errorf("Service.Update(%q) did not replace the API key", credential.Name)
	}
}

func TestServiceCreateRejectsDuplicateDisplayName(t *testing.T) {
	credential := &resource.UpstreamCredential{
		ObjectMeta: metav1.ObjectMeta{Name: "credential-1"},
		Spec: resource.UpstreamCredentialSpec{
			DisplayName: "OpenAI production",
			Type:        resource.UpstreamCredentialTypeAPIKey,
			APIKey:      &resource.APIKeyCredential{Value: "existing-secret"},
		},
	}
	client := clientfake.NewSimpleClientset(credential)
	service := New(credentialstore.New(client), upstreamstore.New(client))

	_, err := service.Create(context.Background(), CreateParams{CredentialParams: CredentialParams{
		Name:        credential.Spec.DisplayName,
		Type:        resource.UpstreamCredentialTypeAPIKey,
		APIKeyValue: "new-secret",
	}})
	if err == nil {
		t.Fatal("Service.Create(duplicate name) error = nil, want duplicate name error")
	}
	var userError *xerrors.UserError
	if !errors.As(err, &userError) {
		t.Errorf("Service.Create(duplicate name) error = %T, want *xerrors.UserError", err)
	}
}

func TestServiceDeleteRejectsReferencedCredential(t *testing.T) {
	credential := &resource.UpstreamCredential{
		ObjectMeta: metav1.ObjectMeta{Name: "credential-1"},
		Spec: resource.UpstreamCredentialSpec{
			DisplayName: "OpenAI production",
			Type:        resource.UpstreamCredentialTypeAPIKey,
			APIKey:      &resource.APIKeyCredential{Value: "existing-secret"},
		},
	}
	upstream := &resource.Upstream{
		ObjectMeta: metav1.ObjectMeta{Name: "upstream-1"},
		Spec: resource.UpstreamSpec{
			DisplayName:   "OpenAI",
			CredentialRef: credential.Name,
		},
	}
	client := clientfake.NewSimpleClientset(credential, upstream)
	service := New(credentialstore.New(client), upstreamstore.New(client))

	err := service.Delete(context.Background(), credential.Name)
	if err == nil {
		t.Fatal("Service.Delete(referenced credential) error = nil, want reference error")
	}
	var userError *xerrors.UserError
	if !errors.As(err, &userError) {
		t.Errorf("Service.Delete(referenced credential) error = %T, want *xerrors.UserError", err)
	}
}
