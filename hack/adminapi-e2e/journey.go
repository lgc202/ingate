package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

type resourceResult struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	State   string     `json:"state"`
	Version int64Value `json:"version"`
}

type cleanupRef struct {
	path    string
	version int64
}

type journey struct {
	client       *apiClient
	prefix       string
	cleanupStack []cleanupRef
}

func newJourney(client *apiClient, prefix string) *journey {
	return &journey{client: client, prefix: prefix}
}

func (j *journey) run(ctx context.Context) error {
	hostname := j.prefix + ".example.test"
	certificatePEM, privateKeyPEM, err := selfSignedCertificate(hostname)
	if err != nil {
		return err
	}

	certificate, err := j.create(ctx, "certificate", "/api/v1/certificates", map[string]any{
		"name":           j.prefix + " certificate",
		"certificatePEM": certificatePEM,
		"privateKeyPEM":  privateKeyPEM,
	})
	if err != nil {
		return err
	}

	port, err := randomPort()
	if err != nil {
		return err
	}
	gateway, err := j.create(ctx, "gateway", "/api/v1/gateways", map[string]any{
		"name":    j.prefix + " gateway",
		"enabled": true,
		"listeners": []any{map[string]any{
			"name":          "https",
			"protocol":      "GATEWAY_PROTOCOL_HTTPS",
			"port":          port,
			"hostname":      hostname,
			"certificateID": certificate.ID,
		}},
	})
	if err != nil {
		return err
	}

	service, err := j.create(ctx, "service", "/api/v1/services", serviceRequest(j.prefix+" service"))
	if err != nil {
		return err
	}

	route, err := j.create(ctx, "route", "/api/v1/routes", map[string]any{
		"name":   j.prefix + " route",
		"config": routeConfig(gateway.ID, service.ID),
	})
	if err != nil {
		return err
	}

	policy, err := j.create(ctx, "rate-limit policy", "/api/v1/rate-limit-policies", map[string]any{
		"name":    j.prefix + " rate limit",
		"enabled": true,
		"targets": []any{map[string]any{
			"kind": "POLICY_TARGET_KIND_ROUTE",
			"id":   route.ID,
		}},
		"subject": map[string]any{"type": "RATE_LIMIT_SUBJECT_TYPE_SHARED"},
		"limit":   map[string]any{"requests": 100, "windowSeconds": 60},
	})
	if err != nil {
		return err
	}

	checks := []struct {
		name       string
		collection string
		path       string
		item       resourceResult
	}{
		{name: "certificate", collection: "certificates", path: "/api/v1/certificates", item: certificate},
		{name: "gateway", collection: "gateways", path: "/api/v1/gateways", item: gateway},
		{name: "service", collection: "services", path: "/api/v1/services", item: service},
		{name: "route", collection: "routes", path: "/api/v1/routes", item: route},
		{name: "rate-limit policy", collection: "policies", path: "/api/v1/rate-limit-policies", item: policy},
	}
	for _, check := range checks {
		if err := j.assertReadable(ctx, check.name, check.collection, check.path, check.item); err != nil {
			return err
		}
	}

	updatedName := j.prefix + " service updated"
	var updated resourceResult
	updateBody := serviceRequest(updatedName)
	updateBody["version"] = int64(service.Version)
	if err := j.client.call(ctx, http.MethodPut, "/api/v1/services/"+service.ID, updateBody, &updated); err != nil {
		return fmt.Errorf("update service: %w", err)
	}
	if updated.Name != updatedName || updated.Version <= service.Version {
		return fmt.Errorf("update service: updated value or version is not visible")
	}
	j.updateCleanupVersion("/api/v1/services/"+service.ID, int64(updated.Version))

	staleBody := serviceRequest(j.prefix + " stale update")
	staleBody["version"] = int64(service.Version)
	if err := j.client.expectFailure(
		ctx,
		http.MethodPut,
		"/api/v1/services/"+service.ID,
		staleBody,
		http.StatusConflict,
		"RESOURCE_VERSION_CONFLICT",
	); err != nil {
		return fmt.Errorf("reject stale service update: %w", err)
	}

	deletePath := "/api/v1/services/" + service.ID + "?version=" + strconv.FormatInt(int64(updated.Version), 10)
	if err := j.client.expectFailure(
		ctx,
		http.MethodDelete,
		deletePath,
		nil,
		http.StatusConflict,
		"RESOURCE_REFERENCED",
	); err != nil {
		return fmt.Errorf("reject referenced service deletion: %w", err)
	}

	if err := j.client.expectFailure(
		ctx,
		http.MethodGet,
		"/api/v1/services/00000000-0000-4000-8000-000000000000",
		nil,
		http.StatusNotFound,
		"RESOURCE_NOT_FOUND",
	); err != nil {
		return fmt.Errorf("get missing service: %w", err)
	}
	return nil
}

func (j *journey) create(
	ctx context.Context,
	name string,
	collectionPath string,
	body any,
) (resourceResult, error) {
	var result resourceResult
	if err := j.client.call(ctx, http.MethodPost, collectionPath, body, &result); err != nil {
		return resourceResult{}, fmt.Errorf("create %s: %w", name, err)
	}
	if result.ID == "" || result.Version <= 0 {
		return resourceResult{}, fmt.Errorf("create %s: response lacks id or version", name)
	}
	j.cleanupStack = append(j.cleanupStack, cleanupRef{
		path:    collectionPath + "/" + result.ID,
		version: int64(result.Version),
	})
	if !validState(result.State) {
		return resourceResult{}, fmt.Errorf("create %s: response lacks valid state", name)
	}
	fmt.Printf("created %s\n", name)
	return result, nil
}

func (j *journey) assertReadable(
	ctx context.Context,
	name string,
	collection string,
	collectionPath string,
	want resourceResult,
) error {
	var got resourceResult
	if err := j.client.call(ctx, http.MethodGet, collectionPath+"/"+want.ID, nil, &got); err != nil {
		return fmt.Errorf("get %s: %w", name, err)
	}
	if got.ID != want.ID || got.Name != want.Name || !validState(got.State) {
		return fmt.Errorf("get %s: resource fields are not visible", name)
	}
	cursor := ""
	for {
		query := url.Values{"limit": {"50"}, "query": {want.Name}}
		if cursor != "" {
			query.Set("cursor", cursor)
		}
		var page map[string]json.RawMessage
		if err := j.client.call(ctx, http.MethodGet, collectionPath+"?"+query.Encode(), nil, &page); err != nil {
			return fmt.Errorf("list %s: %w", name, err)
		}
		var items []resourceResult
		if err := json.Unmarshal(page[collection], &items); err != nil {
			return fmt.Errorf("list %s: decode collection: %w", name, err)
		}
		for _, item := range items {
			if item.ID == want.ID {
				return nil
			}
		}
		cursor = ""
		if rawCursor := page["nextCursor"]; len(rawCursor) > 0 {
			if err := json.Unmarshal(rawCursor, &cursor); err != nil {
				return fmt.Errorf("list %s: decode next cursor: %w", name, err)
			}
		}
		if cursor == "" {
			break
		}
	}
	return fmt.Errorf("list %s: created resource is missing", name)
}

func (j *journey) updateCleanupVersion(path string, version int64) {
	for i := range j.cleanupStack {
		if j.cleanupStack[i].path == path {
			j.cleanupStack[i].version = version
			return
		}
	}
}

func (j *journey) cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for i := len(j.cleanupStack) - 1; i >= 0; i-- {
		ref := j.cleanupStack[i]
		path := ref.path + "?" + url.Values{"version": {strconv.FormatInt(ref.version, 10)}}.Encode()
		if err := j.client.call(ctx, http.MethodDelete, path, nil, nil); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "cleanup %s failed: %v\n", ref.path, err)
		}
	}
}

func serviceRequest(name string) map[string]any {
	return map[string]any{
		"name":          name,
		"endpoints":     []any{map[string]any{"address": "127.0.0.1", "port": 8080, "weight": 1}},
		"loadBalancing": "LOAD_BALANCING_POLICY_ROUND_ROBIN",
	}
}

func routeConfig(gatewayID, serviceID string) map[string]any {
	return map[string]any{
		"enabled":    true,
		"accessMode": "ROUTE_ACCESS_MODE_PUBLIC",
		"gatewayIDs": []string{gatewayID},
		"match": map[string]any{
			"path": map[string]any{"type": "ROUTE_PATH_MATCH_TYPE_PREFIX", "value": "/"},
		},
		"forwarding": map[string]any{
			"service": map[string]any{
				"targets": []any{map[string]any{"serviceID": serviceID, "weight": 1}},
			},
		},
	}
}

func validState(state string) bool {
	switch state {
	case "DISABLED", "PENDING", "READY", "ERROR":
		return true
	default:
		return false
	}
}

func uniquePrefix() (string, error) {
	buffer := make([]byte, 4)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate unique resource prefix: %w", err)
	}
	return "e2e-" + strconv.FormatInt(time.Now().UTC().Unix(), 36) + "-" + hex.EncodeToString(buffer), nil
}

func randomPort() (int, error) {
	buffer := make([]byte, 2)
	if _, err := rand.Read(buffer); err != nil {
		return 0, fmt.Errorf("generate listener port: %w", err)
	}
	value := int(buffer[0])<<8 | int(buffer[1])
	return 30000 + value%20000, nil
}
