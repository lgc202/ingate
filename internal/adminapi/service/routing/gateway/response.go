package gateway

import (
	"github.com/samber/lo"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz/resource/status"
	"github.com/lgc202/ingate/internal/adminapi/service/conversion"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func gatewayResponse(gateway *resource.Gateway) *adminv1.Gateway {
	resourceStatus := status.ForEnabledResource(
		gateway.Generation,
		gateway.Spec.Enabled,
		gateway.Status.Conditions,
	)
	listeners := lo.Map(gateway.Spec.Listeners, func(listener resource.Listener, _ int) *adminv1.GatewayListener {
		return &adminv1.GatewayListener{
			Name:          listener.Name,
			Protocol:      gatewayProtocolResponse(listener.Protocol),
			Port:          uint32(listener.Port),
			Hostname:      listener.Hostname,
			CertificateId: listener.CertificateRef,
		}
	})

	return &adminv1.Gateway{
		Id:        gateway.Name,
		Name:      gateway.Spec.DisplayName,
		Enabled:   gateway.Spec.Enabled,
		Listeners: listeners,
		State:     conversion.ResourceState(resourceStatus.State),
		Message:   conversion.ResourceMessage(resourceStatus.Reason),
		Version:   gateway.Generation,
		CreatedAt: conversion.Timestamp(gateway.CreationTimestamp.Time),
		UpdatedAt: conversion.Timestamp(conversion.ResourceUpdatedAt(gateway.Annotations)),
	}
}

func gatewayProtocolResponse(protocol resource.Protocol) adminv1.GatewayProtocol {
	switch protocol {
	case resource.ProtocolHTTP:
		return adminv1.GatewayProtocol_GATEWAY_PROTOCOL_HTTP
	case resource.ProtocolHTTPS:
		return adminv1.GatewayProtocol_GATEWAY_PROTOCOL_HTTPS
	default:
		return adminv1.GatewayProtocol_GATEWAY_PROTOCOL_UNSPECIFIED
	}
}
