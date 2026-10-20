package aiconfig

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"buf.build/go/protovalidate"
	"k8s.io/apimachinery/pkg/util/validation"

	aidataplanev1 "github.com/lgc202/ingate/api/aidataplane/v1"
)

// validateResource 校验 protobuf 约束及 HTTP 路径语义；跨资源引用在整批合并后检查。
func validateResource(resource *aidataplanev1.Resource) error {
	if err := protovalidate.Validate(resource); err != nil {
		return err
	}

	switch payload := resource.Payload.(type) {
	case *aidataplanev1.Resource_Entry:
		if payload.Entry.Hostname != "" && len(validation.IsDNS1123Subdomain(payload.Entry.Hostname)) != 0 {
			return errors.New("entry hostname must be a DNS hostname")
		}
	case *aidataplanev1.Resource_ModelRule:
		if err := payload.ModelRule.Origin.CreationTime.CheckValid(); err != nil {
			return fmt.Errorf("model rule creation time: %w", err)
		}

		if target := payload.ModelRule.GetTarget(); target != nil {
			if err := validateResourcePath(target.Schema.Prefix); err != nil {
				return fmt.Errorf("model prefix: %w", err)
			}

			if strings.ContainsAny(target.TargetName, " \t\r\n") {
				return errors.New("model target name must be a single header token")
			}
		}
	case *aidataplanev1.Resource_McpRoute:
		if err := payload.McpRoute.Origin.CreationTime.CheckValid(); err != nil {
			return fmt.Errorf("MCP route creation time: %w", err)
		}

		if err := validateResourcePath(payload.McpRoute.Path); err != nil {
			return fmt.Errorf("MCP route path: %w", err)
		}

		if target := payload.McpRoute.GetTarget(); target != nil {
			if err := validateResourcePath(target.BackendPath); err != nil {
				return fmt.Errorf("MCP backend path: %w", err)
			}

			if strings.ContainsAny(target.TargetName, " \t\r\n") {
				return errors.New("MCP target name must be a single header token")
			}
		}
	}

	seen := make(map[hostKey]bool)
	for _, key := range resourceEntries(resource) {
		if seen[key] || strings.ContainsAny(key.name, " ,\t\r\n") {
			return errors.New("entry references must be unique with single-token host names")
		}

		seen[key] = true
	}

	return nil
}

func validateResourcePath(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("parse path: %w", err)
	}

	if !strings.HasPrefix(value, "/") || parsed.Host != "" || parsed.User != nil || strings.ContainsAny(value, "?#") {
		return errors.New("path must be absolute without authority, query or fragment")
	}

	return nil
}
