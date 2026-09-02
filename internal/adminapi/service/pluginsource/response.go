package pluginsource

import (
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	pluginsourcebiz "github.com/lgc202/ingate/internal/adminapi/biz/pluginsource"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service/protocol"
)

func sourceResponse(source pluginsourcebiz.Source) *adminv1.PluginSource {
	return &adminv1.PluginSource{
		Id:           source.ID,
		Name:         source.DisplayName,
		Url:          source.URL,
		Builtin:      source.Builtin,
		Enabled:      source.Enabled,
		SyncState:    syncStateResponse(source.Observation.State),
		Message:      source.Observation.Message,
		PluginCount:  int32(source.Observation.PluginCount),
		LastSyncedAt: adminservice.Timestamp(source.Observation.LastSyncedAt),
		Version:      source.Generation,
		CreatedAt:    adminservice.Timestamp(source.CreatedAt),
		UpdatedAt:    adminservice.Timestamp(source.UpdatedAt),
	}
}

func syncStateResponse(state pluginsourcebiz.SyncState) adminv1.PluginSourceSyncState {
	switch state {
	case pluginsourcebiz.SyncStateReady:
		return adminv1.PluginSourceSyncState_PLUGIN_SOURCE_SYNC_STATE_READY
	case pluginsourcebiz.SyncStateError:
		return adminv1.PluginSourceSyncState_PLUGIN_SOURCE_SYNC_STATE_ERROR
	case pluginsourcebiz.SyncStateDisabled:
		return adminv1.PluginSourceSyncState_PLUGIN_SOURCE_SYNC_STATE_DISABLED
	case pluginsourcebiz.SyncStateNotSynced:
		return adminv1.PluginSourceSyncState_PLUGIN_SOURCE_SYNC_STATE_NOT_SYNCED
	default:
		return adminv1.PluginSourceSyncState_PLUGIN_SOURCE_SYNC_STATE_UNSPECIFIED
	}
}
