package plugincatalog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz/pagination"
	sourcebiz "github.com/lgc202/ingate/internal/adminapi/biz/plugin/source"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/version"
)

const (
	maxCatalogBytes          = 1 << 20
	maxCatalogETagBytes      = 4 << 10
	maxConcurrentSourceSyncs = 4
)

type manifestResponse struct {
	data        []byte
	etag        string
	notModified bool
}

func (c *Catalog) run(ctx context.Context, done chan struct{}) {
	defer c.finishRun(done)
	c.syncAllAndLog(ctx)

	ticker := time.NewTicker(c.refreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.syncAllAndLog(ctx)
		}
	}
}

func (c *Catalog) syncAllAndLog(ctx context.Context) {
	if err := c.syncAll(ctx); err != nil && ctx.Err() == nil {
		c.logger.Warn("sync plugin sources failed", "err", err)
	}
}

func (c *Catalog) syncAll(ctx context.Context) error {
	definitions := make([]sourceDefinition, 0)
	if c.official.enabled {
		definitions = append(definitions, c.official)
	}
	if err := pagination.VisitPages(ctx, c.store.ListPage, func(source resource.PluginSource) (bool, error) {
		definitions = append(definitions, definitionFromResource(&source))
		return false, nil
	}); err != nil {
		return fmt.Errorf("list custom plugin sources: %w", err)
	}

	active := make(map[string]bool, len(definitions))
	var group errgroup.Group
	group.SetLimit(maxConcurrentSourceSyncs)
	for _, definition := range definitions {
		active[definition.id] = true
		group.Go(func() error {
			err := c.syncSource(ctx, definition)
			switch {
			case err == nil:
				return nil
			case ctx.Err() != nil:
				return ctx.Err()
			case errors.Is(err, sourcebiz.ErrSyncFailed),
				errors.Is(err, sourcebiz.ErrSyncUnavailable):
				// 远程来源错误已经写入该来源的观测状态，并在状态变化时记录一次。
				return nil
			default:
				return fmt.Errorf("sync plugin source %q: %w", definition.id, err)
			}
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	return c.removeDeletedSources(ctx, active)
}

func (c *Catalog) sourceDefinition(
	ctx context.Context,
	sourceID string,
) (sourceDefinition, error) {
	if sourceID == sourcebiz.OfficialSourceID {
		if c.official.catalogURL == "" {
			return sourceDefinition{}, adminv1.ErrorResourceNotFound("资源不存在或已被删除")
		}
		return c.official, nil
	}
	source, err := c.store.Get(ctx, sourceID)
	if err != nil {
		return sourceDefinition{}, err
	}
	return definitionFromResource(source), nil
}

func (c *Catalog) syncSource(ctx context.Context, definition sourceDefinition) error {
	// 同一来源串行提交，不同来源独立下载；等待中的手动请求仍响应调用方取消。
	release, err := c.acquireSourceSync(ctx, definition.id)
	if err != nil {
		return err
	}
	defer release()

	current, err := c.isCurrentDefinition(ctx, definition)
	if err != nil {
		return fmt.Errorf("verify plugin source %q before sync: %w", definition.id, err)
	}
	if !current {
		return nil
	}
	if !definition.enabled {
		c.storeDisabledSource(definition)
		return nil
	}

	previous := c.loadSourceState(definition.id)
	if previous.definition.catalogURL != definition.catalogURL {
		previous = sourceState{}
	}
	manifest, err := c.fetchManifest(ctx, definition.catalogURL, previous.etag)
	if err != nil {
		c.recordSyncFailure(ctx, definition, previous, err)
		category := sourcebiz.ErrSyncFailed
		if errors.Is(err, sourcebiz.ErrSyncUnavailable) {
			category = sourcebiz.ErrSyncUnavailable
		}
		return fmt.Errorf(
			"%w: sync plugin source %q: %w",
			category,
			definition.id,
			err,
		)
	}
	if manifest.notModified {
		applySourceDefinition(&previous, definition)
		return c.storeSuccessfulSync(
			ctx,
			definition,
			previous,
			previous.etag,
			previous.observation.State == sourcebiz.SyncStateError,
		)
	}
	state, err := parseManifest(manifest.data, definition, version.String())
	if err != nil {
		c.recordSyncFailure(ctx, definition, previous, err)
		return fmt.Errorf(
			"%w: parse plugin source %q manifest: %w",
			sourcebiz.ErrSyncFailed,
			definition.id,
			err,
		)
	}
	return c.storeSuccessfulSync(
		ctx,
		definition,
		state,
		manifest.etag,
		previous.observation.State == sourcebiz.SyncStateError,
	)
}

func (c *Catalog) storeSuccessfulSync(
	ctx context.Context,
	definition sourceDefinition,
	state sourceState,
	etag string,
	recovered bool,
) error {
	state.etag = etag
	state.available = true
	state.observation = sourcebiz.Observation{
		State:        sourcebiz.SyncStateReady,
		PluginCount:  len(state.items),
		LastSyncedAt: time.Now(),
	}
	stored, err := c.storeStateIfCurrent(ctx, definition, state)
	if err != nil {
		return err
	}
	if stored && recovered {
		c.logger.Info("plugin source recovered", "source_id", definition.id)
	}
	return nil
}

func (c *Catalog) fetchManifest(
	ctx context.Context,
	sourceURL string,
	etag string,
) (manifestResponse, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return manifestResponse{}, fmt.Errorf("create HTTP request: %w", err)
	}
	if etag != "" {
		request.Header.Set("If-None-Match", etag)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "ingate/"+version.String())
	response, err := c.client.Do(request)
	if err != nil {
		if errors.Is(err, sourcebiz.ErrSyncFailed) {
			return manifestResponse{}, err
		}
		return manifestResponse{}, fmt.Errorf(
			"%w: send HTTP request: %w",
			sourcebiz.ErrSyncUnavailable,
			err,
		)
	}
	defer func() {
		// 响应体已完整读取或不再消费；关闭阶段的传输错误不改变同步结果。
		_ = response.Body.Close()
	}()

	switch response.StatusCode {
	case http.StatusNotModified:
		if etag == "" {
			return manifestResponse{}, errors.New("received HTTP 304 without a conditional request")
		}
		return manifestResponse{notModified: true}, nil
	case http.StatusOK:
	default:
		if response.StatusCode >= http.StatusInternalServerError {
			return manifestResponse{}, fmt.Errorf(
				"%w: unexpected HTTP status %s",
				sourcebiz.ErrSyncUnavailable,
				response.Status,
			)
		}
		return manifestResponse{}, fmt.Errorf("unexpected HTTP status %s", response.Status)
	}
	if response.ContentLength > maxCatalogBytes {
		return manifestResponse{}, fmt.Errorf("response exceeds %d bytes", maxCatalogBytes)
	}

	data, err := io.ReadAll(io.LimitReader(response.Body, maxCatalogBytes+1))
	if err != nil {
		return manifestResponse{}, fmt.Errorf(
			"%w: read response body: %w",
			sourcebiz.ErrSyncUnavailable,
			err,
		)
	}
	if len(data) > maxCatalogBytes {
		return manifestResponse{}, fmt.Errorf("response exceeds %d bytes", maxCatalogBytes)
	}
	responseETag := response.Header.Get("ETag")
	if len(responseETag) > maxCatalogETagBytes {
		return manifestResponse{}, fmt.Errorf("response ETag exceeds %d bytes", maxCatalogETagBytes)
	}
	return manifestResponse{
		data: data,
		etag: responseETag,
	}, nil
}

func (c *Catalog) removeDeletedSources(ctx context.Context, active map[string]bool) error {
	c.stateMu.RLock()
	missingSourceIDs := make([]string, 0)
	for sourceID := range c.states {
		if !active[sourceID] {
			missingSourceIDs = append(missingSourceIDs, sourceID)
		}
	}
	c.stateMu.RUnlock()

	for _, sourceID := range missingSourceIDs {
		if err := c.removeDeletedSource(ctx, sourceID); err != nil {
			return err
		}
	}
	return nil
}

func (c *Catalog) removeDeletedSource(ctx context.Context, sourceID string) error {
	release, err := c.acquireSourceSync(ctx, sourceID)
	if err != nil {
		return err
	}
	defer release()

	if sourceID != sourcebiz.OfficialSourceID {
		_, err := c.store.Get(ctx, sourceID)
		switch {
		case err == nil:
			return nil
		case !adminv1.IsResourceNotFound(err):
			return fmt.Errorf("verify deleted plugin source %q: %w", sourceID, err)
		}
	}
	c.stateMu.Lock()
	delete(c.states, sourceID)
	c.stateMu.Unlock()
	return nil
}
