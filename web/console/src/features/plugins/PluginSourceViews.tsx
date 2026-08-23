import {
  Badge,
  EmptyState,
  ResourceFilterField,
  ResourceListFilters,
  RowActions,
  SearchField,
} from '@/components/ui';
import { formatDateTime } from '@/domain/common';
import {
  pluginSourceStateLabel,
  pluginSourceStateTone,
  type PluginSource,
  type PluginSourceInput,
  type PluginSourceSyncState,
} from '@/domain/plugin';

export type PluginSourceStateFilter = 'all' | PluginSourceSyncState;

export interface PluginSourceFilters {
  query: string;
  state: PluginSourceStateFilter;
}

export const emptyPluginSourceFilters = (): PluginSourceFilters => ({ query: '', state: 'all' });

export function PluginSources({
  allSources,
  sources,
  total,
  filters,
  appliedFilters,
  busySourceID,
  onFiltersChange,
  onSearch,
  onReset,
  onDetail,
  onEdit,
  onSync,
  onDelete,
}: {
  allSources: PluginSource[];
  sources: PluginSource[];
  total: number;
  filters: PluginSourceFilters;
  appliedFilters: PluginSourceFilters;
  busySourceID: string;
  onFiltersChange: (filters: PluginSourceFilters) => void;
  onSearch: () => void;
  onReset: () => void;
  onDetail: (source: PluginSource) => void;
  onEdit: (source: PluginSource) => void;
  onSync: (source: PluginSource) => void;
  onDelete: (source: PluginSource) => void;
}) {
  return (
    <>
      <ResourceListFilters summary={pluginSourceFilterSummary(appliedFilters)} resultLabel={`${total} 个插件源`} onSearch={onSearch} onReset={onReset}>
        <ResourceFilterField label="关键词">
          <SearchField value={filters.query} onChange={(query) => onFiltersChange({ ...filters, query })} placeholder="搜索插件源名称或目录地址" />
        </ResourceFilterField>
        <ResourceFilterField label="同步状态">
          <select className="select" value={filters.state} onChange={(event) => onFiltersChange({ ...filters, state: event.target.value as PluginSourceStateFilter })}>
            <option value="all">全部同步状态</option>
            <option value="PLUGIN_SOURCE_SYNC_STATE_READY">同步正常</option>
            <option value="PLUGIN_SOURCE_SYNC_STATE_ERROR">同步失败</option>
            <option value="PLUGIN_SOURCE_SYNC_STATE_NOT_SYNCED">尚未同步</option>
            <option value="PLUGIN_SOURCE_SYNC_STATE_DISABLED">已停用</option>
          </select>
        </ResourceFilterField>
      </ResourceListFilters>
      {sources.length === 0 ? (
        <div className="p-5">
          <EmptyState
            title={allSources.length === 0 ? '暂无插件源' : '没有匹配的插件源'}
            message={allSources.length === 0 ? '添加一个公开的插件目录地址后即可发现插件。' : '请调整筛选条件。'}
          />
        </div>
      ) : (
        <div className="table-scroll resource-table-scroll">
          <table className="table resource-table plugin-source-table">
            <thead><tr><th>插件源</th><th>同步状态</th><th>可用插件</th><th>最近同步</th><th>操作</th></tr></thead>
            <tbody>
              {sources.map((source) => (
                <tr key={source.id}>
                  <td>
                    <div className="table-primary">{source.name}{source.builtin ? <Badge tone="accent">官方</Badge> : null}</div>
                    <div className="table-secondary plugin-source-url">{source.url}</div>
                  </td>
                  <td>
                    <Badge tone={pluginSourceStateTone(source.syncState)}>{pluginSourceStateLabel(source.syncState)}</Badge>
                    {source.message ? <div className="table-secondary plugin-status-message">{source.message}</div> : null}
                  </td>
                  <td><div className="table-primary">{source.pluginCount} 个</div></td>
                  <td className="resource-table-time">{source.lastSyncedAt ? formatDateTime(source.lastSyncedAt) : '尚未同步'}</td>
                  <td>
                    <RowActions
                      onDetail={() => onDetail(source)}
                      onEdit={source.builtin ? undefined : () => onEdit(source)}
                      onToggle={source.enabled ? () => onSync(source) : undefined}
                      toggleLabel={busySourceID === source.id ? '同步中...' : '同步'}
                      toggleDisabled={busySourceID !== ''}
                      onDelete={source.builtin ? undefined : () => onDelete(source)}
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}

export function PluginSourceEditor({
  draft,
  error,
  onChange,
}: {
  draft: PluginSourceInput;
  error: string;
  onChange: (draft: PluginSourceInput) => void;
}) {
  return (
    <div className="editor-main-stack">
      <section className="form-section">
        <div className="form-section-title"><h3>基本信息</h3></div>
        <div className="policy-editor-grid">
          <label className="plugin-field"><span>插件源名称</span><input className="input" value={draft.name} placeholder="例如：团队插件源" onChange={(event) => onChange({ ...draft, name: event.target.value })} /></label>
          <label className="plugin-field"><span>目录地址</span><input className="input font-mono" type="url" value={draft.url} placeholder="https://plugins.example.com/catalog.json" onChange={(event) => onChange({ ...draft, url: event.target.value })} /></label>
        </div>
        <label className="policy-check-row">
          <input type="checkbox" checked={draft.enabled} onChange={(event) => onChange({ ...draft, enabled: event.target.checked })} />
          <span>启用插件源</span>
        </label>
        {error ? <div className="form-error" role="alert">{error}</div> : null}
      </section>
    </div>
  );
}

export function PluginSourceDetail({ source }: { source: PluginSource }) {
  return (
    <div className="space-y-5">
      <section className="resource-detail-hero">
        <div><h3>{source.name}</h3><p>{source.builtin ? 'Ingate 内置官方来源' : '自定义插件来源'}</p></div>
        <Badge tone={pluginSourceStateTone(source.syncState)}>{pluginSourceStateLabel(source.syncState)}</Badge>
      </section>
      <section className="resource-detail-section">
        <h3>来源信息</h3>
        <div className="resource-detail-grid">
          <div><span>来源类型</span><strong>{source.builtin ? '官方' : '自定义'}</strong></div>
          <div><span>启用状态</span><strong>{source.enabled ? '已启用' : '已停用'}</strong></div>
          <div><span>可用插件</span><strong>{source.pluginCount} 个</strong></div>
          <div><span>最近同步</span><strong>{source.lastSyncedAt ? formatDateTime(source.lastSyncedAt) : '尚未同步'}</strong></div>
        </div>
        {source.message ? <p className="plugin-detail-message">{source.message}</p> : null}
      </section>
      <section className="resource-detail-section space-y-3">
        <h3>目录地址</h3>
        <div className="plugin-module-detail"><code>{source.url}</code></div>
      </section>
    </div>
  );
}

function pluginSourceFilterSummary(filters: PluginSourceFilters): string {
  const conditions = [];
  if (filters.query.trim()) conditions.push(`关键词“${filters.query.trim()}”`);
  if (filters.state !== 'all') conditions.push(`同步状态：${pluginSourceStateLabel(filters.state)}`);
  return conditions.join(' · ') || '全部插件源';
}
