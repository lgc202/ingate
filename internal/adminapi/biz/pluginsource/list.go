package pluginsource

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"github.com/lgc202/ingate/internal/adminapi/biz/apperror"
	"github.com/lgc202/ingate/internal/adminapi/biz/pagination"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// List 返回满足筛选条件的官方与自定义插件源。
func (uc *Usecase) List(
	ctx context.Context,
	page pagination.Request,
	filter ListFilter,
) (SourcePage, error) {
	filter.Query = strings.ToLower(strings.TrimSpace(filter.Query))
	sources := make([]Source, 0)
	if official := uc.catalog.OfficialSource(); official.URL != "" {
		sources = append(sources, official)
	}
	if err := pagination.VisitPages(ctx, uc.store.ListPage, func(source resource.PluginSource) (bool, error) {
		sources = append(sources, uc.sourceFromResource(&source))
		return false, nil
	}); err != nil {
		return SourcePage{}, err
	}

	slices.SortStableFunc(sources, compareSources)
	sources = slices.DeleteFunc(sources, func(source Source) bool {
		return !filter.matches(source)
	})
	start, err := sourcePageStart(sources, page.Cursor)
	if err != nil {
		return SourcePage{}, err
	}
	end := min(start+int(page.Limit), len(sources))
	items := slices.Clone(sources[start:end])

	var nextCursor string
	if end < len(sources) && len(items) > 0 {
		nextCursor = items[len(items)-1].ID
	}
	return SourcePage{Items: items, NextCursor: nextCursor}, nil
}

func (f ListFilter) matches(source Source) bool {
	if f.Query != "" && !strings.Contains(
		strings.ToLower(source.DisplayName+" "+source.URL),
		f.Query,
	) {
		return false
	}
	if f.Enabled != nil && source.Enabled != *f.Enabled {
		return false
	}
	return f.State == "" || source.Observation.State == f.State
}

func compareSources(left, right Source) int {
	if left.Builtin != right.Builtin {
		if left.Builtin {
			return -1
		}
		return 1
	}
	if result := cmp.Compare(left.DisplayName, right.DisplayName); result != 0 {
		return result
	}
	return cmp.Compare(left.ID, right.ID)
}

func sourcePageStart(sources []Source, cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	for i := range sources {
		if sources[i].ID == cursor {
			return i + 1, nil
		}
	}
	return 0, apperror.InvalidCursor(nil)
}
