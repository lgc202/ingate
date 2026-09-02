package plugincatalog

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/lgc202/ingate/internal/adminapi/biz/apperror"
	"github.com/lgc202/ingate/internal/adminapi/biz/pluginsource"
	"github.com/lgc202/ingate/internal/adminapi/biz/wasmplugin"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func (c *Catalog) loadSourceState(sourceID string) sourceState {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.states[sourceID]
}

func (c *Catalog) storeSourceState(state sourceState) bool {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	current, exists := c.states[state.definition.id]
	if exists {
		if current.definition.generation > state.definition.generation ||
			current.invalidated && current.definition.generation >= state.definition.generation {
			return false
		}
	}
	c.states[state.definition.id] = state
	return true
}

func (c *Catalog) recordSyncFailure(
	ctx context.Context,
	definition sourceDefinition,
	previous sourceState,
	err error,
) {
	if ctx.Err() != nil {
		return
	}
	current, checkErr := c.isCurrentDefinition(ctx, definition)
	if checkErr != nil {
		c.logger.Warn(
			"verify plugin source after sync failure",
			"source_id", definition.id,
			"err", checkErr,
		)
		return
	}
	if !current {
		return
	}
	becameUnavailable := previous.observation.State != pluginsource.SyncStateError
	lastSyncedAt := previous.observation.LastSyncedAt
	message := "目录同步失败，请检查地址和目录内容"
	if errors.Is(err, pluginsource.ErrSyncUnavailable) {
		message = "目录暂时不可用，请稍后重试"
	}
	applySourceDefinition(&previous, definition)
	previous.observation = pluginsource.Observation{
		State:        pluginsource.SyncStateError,
		Message:      message,
		PluginCount:  len(previous.items),
		LastSyncedAt: lastSyncedAt,
	}
	if c.storeSourceState(previous) && becameUnavailable {
		c.logger.Warn("plugin source unavailable", "source_id", definition.id, "err", err)
	}
}

func (c *Catalog) storeDisabledSource(definition sourceDefinition) {
	c.storeSourceState(sourceState{
		definition:  definition,
		items:       make([]wasmplugin.CatalogItem, 0),
		specs:       make(map[string]resource.WasmPluginSpec),
		observation: pluginsource.Observation{State: pluginsource.SyncStateDisabled},
	})
}

func applySourceDefinition(state *sourceState, definition sourceDefinition) {
	if state.definition.catalogURL != definition.catalogURL {
		state.items = nil
		state.specs = nil
		state.etag = ""
	}
	if state.definition.displayName != definition.displayName {
		state.items = slices.Clone(state.items)
		for i := range state.items {
			state.items[i].SourceName = definition.displayName
		}
	}
	state.definition = definition
}

func definitionFromResource(source *resource.PluginSource) sourceDefinition {
	return sourceDefinition{
		id:          source.Name,
		displayName: source.Spec.DisplayName,
		catalogURL:  source.Spec.URL,
		enabled:     source.Spec.Enabled,
		generation:  source.Generation,
	}
}

func (c *Catalog) isCurrentDefinition(
	ctx context.Context,
	definition sourceDefinition,
) (bool, error) {
	if definition.id == pluginsource.OfficialSourceID {
		return definition == c.official, nil
	}
	source, err := c.store.Get(ctx, definition.id)
	if err != nil {
		if errors.Is(err, apperror.ResourceNotFound()) {
			return false, nil
		}
		return false, err
	}
	return definitionFromResource(source) == definition, nil
}

func (c *Catalog) storeStateIfCurrent(
	ctx context.Context,
	definition sourceDefinition,
	state sourceState,
) (bool, error) {
	current, err := c.isCurrentDefinition(ctx, definition)
	if err != nil {
		return false, fmt.Errorf("verify plugin source %q after sync: %w", definition.id, err)
	}
	if !current {
		return false, nil
	}
	return c.storeSourceState(state), nil
}

func (c *Catalog) acquireSourceSync(ctx context.Context, sourceID string) (func(), error) {
	for {
		c.sourceSyncMu.Lock()
		wait, syncing := c.sourceSync[sourceID]
		if !syncing {
			done := make(chan struct{})
			c.sourceSync[sourceID] = done
			c.sourceSyncMu.Unlock()
			return func() {
				c.sourceSyncMu.Lock()
				delete(c.sourceSync, sourceID)
				close(done)
				c.sourceSyncMu.Unlock()
			}, nil
		}
		c.sourceSyncMu.Unlock()

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-wait:
		}
	}
}
