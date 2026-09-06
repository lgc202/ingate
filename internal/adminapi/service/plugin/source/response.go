package source

import (
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	sourcebiz "github.com/lgc202/ingate/internal/adminapi/biz/plugin/source"
	"github.com/lgc202/ingate/internal/adminapi/service/conversion"
)

func sourceResponse(source sourcebiz.Source) *adminv1.PluginSource {
	return &adminv1.PluginSource{
		Id:           source.ID,
		Name:         source.DisplayName,
		Url:          source.URL,
		Builtin:      source.Builtin,
		Enabled:      source.Enabled,
		SyncState:    syncStateResponse(source.Observation.State),
		Message:      source.Observation.Message,
		PluginCount:  int32(source.Observation.PluginCount),
		LastSyncedAt: conversion.Timestamp(source.Observation.LastSyncedAt),
		Version:      source.Generation,
		CreatedAt:    conversion.Timestamp(source.CreatedAt),
		UpdatedAt:    conversion.Timestamp(source.UpdatedAt),
	}
}

func syncStateResponse(state sourcebiz.SyncState) adminv1.PluginSourceSyncState {
	switch state {
	case sourcebiz.SyncStateReady:
		return adminv1.PluginSourceSyncState_PLUGIN_SOURCE_SYNC_STATE_READY
	case sourcebiz.SyncStateError:
		return adminv1.PluginSourceSyncState_PLUGIN_SOURCE_SYNC_STATE_ERROR
	case sourcebiz.SyncStateDisabled:
		return adminv1.PluginSourceSyncState_PLUGIN_SOURCE_SYNC_STATE_DISABLED
	case sourcebiz.SyncStateNotSynced:
		return adminv1.PluginSourceSyncState_PLUGIN_SOURCE_SYNC_STATE_NOT_SYNCED
	default:
		return adminv1.PluginSourceSyncState_PLUGIN_SOURCE_SYNC_STATE_UNSPECIFIED
	}
}
