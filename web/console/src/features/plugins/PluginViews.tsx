import { ExternalLink, PackagePlus } from 'lucide-react';
import {
  Badge,
  Button,
  EmptyState,
  ResourceFilterField,
  ResourceListFilters,
  RowActions,
  SearchField,
} from '@/components/ui';
import { formatDateTime, type ResourceState } from '@/domain/common';
import {
  pluginStatusLabel,
  pluginStatusTone,
  type PluginCatalogItem,
  type WasmPlugin,
} from '@/domain/plugin';

export type PluginStateFilter = 'all' | ResourceState;

export interface PluginFilters {
  query: string;
  state: PluginStateFilter;
}

export const emptyPluginFilters = (): PluginFilters => ({ query: '', state: 'all' });

export function PluginMarket({
  items,
  installed,
  onInstall,
  onManage,
  onManageSources,
}: {
  items: PluginCatalogItem[];
  installed: WasmPlugin[];
  onInstall: (item: PluginCatalogItem) => void;
  onManage: () => void;
  onManageSources: () => void;
}) {
  if (items.length === 0) {
    return (
      <div className="space-y-4 p-5">
        <EmptyState title="暂无可用插件" message="请检查已启用插件源的同步状态，或添加新的插件源。" />
        <div className="flex justify-center"><Button variant="outline" onClick={onManageSources}>管理插件源</Button></div>
      </div>
    );
  }
  return (
    <div className="plugin-market">
      {items.map((item) => {
        const installedPlugin = installed.find((plugin) => plugin.package === item.package);
        const installedFromCurrentSource = installedPlugin?.sourceID === item.sourceID;
        return (
          <article className="plugin-market-card" key={`${item.sourceID}:${item.package}`}>
            <div className="plugin-market-icon"><PackagePlus aria-hidden="true" /></div>
            <div className="plugin-market-main">
              <div className="plugin-market-title">
                <h2>{item.name}</h2>
                <Badge tone="accent">{item.category}</Badge>
              </div>
              <p>{item.description}</p>
              <div className="plugin-market-meta">
                <span>{item.sourceName}</span>
                <span>{item.provider}</span>
                <span>v{item.pluginVersion}</span>
                <span>{item.license}</span>
                <a href={item.sourceURL} target="_blank" rel="noreferrer">查看源码<ExternalLink aria-hidden="true" /></a>
              </div>
            </div>
            <div className="plugin-market-action">
              {installedPlugin ? (
                <>
                  <Badge tone={installedFromCurrentSource && installedPlugin.upgradeAvailable ? 'warning' : pluginStatusTone(installedPlugin.state)}>
                    {installedFromCurrentSource
                      ? (installedPlugin.upgradeAvailable ? '可升级' : `已安装 · ${pluginStatusLabel(installedPlugin.state)}`)
                      : `已从${installedPlugin.sourceName}安装`}
                  </Badge>
                  <Button variant="outline" onClick={onManage}>管理</Button>
                </>
              ) : <Button onClick={() => onInstall(item)}>安装</Button>}
            </div>
          </article>
        );
      })}
    </div>
  );
}

export function InstalledPlugins({
  allPlugins,
  plugins,
  total,
  filters,
  appliedFilters,
  catalog,
  onFiltersChange,
  onSearch,
  onReset,
  onDetail,
  onUpgrade,
  onDelete,
}: {
  allPlugins: WasmPlugin[];
  plugins: WasmPlugin[];
  total: number;
  filters: PluginFilters;
  appliedFilters: PluginFilters;
  catalog: PluginCatalogItem[];
  onFiltersChange: (filters: PluginFilters) => void;
  onSearch: () => void;
  onReset: () => void;
  onDetail: (plugin: WasmPlugin) => void;
  onUpgrade: (plugin: WasmPlugin, item: PluginCatalogItem) => void;
  onDelete: (plugin: WasmPlugin) => void;
}) {
  return (
    <>
      <ResourceListFilters summary={pluginFilterSummary(appliedFilters)} resultLabel={`${total} 个插件`} onSearch={onSearch} onReset={onReset}>
        <ResourceFilterField label="关键词">
          <SearchField value={filters.query} onChange={(query) => onFiltersChange({ ...filters, query })} placeholder="搜索插件名称、包名或版本" />
        </ResourceFilterField>
        <ResourceFilterField label="安装状态">
          <select className="select" value={filters.state} onChange={(event) => onFiltersChange({ ...filters, state: event.target.value as PluginStateFilter })}>
            <option value="all">全部安装状态</option>
            <option value="Ready">可用</option>
            <option value="Pending">准备中</option>
            <option value="Error">不可用</option>
          </select>
        </ResourceFilterField>
      </ResourceListFilters>
      {plugins.length === 0 ? (
        <div className="p-5">
          <EmptyState
            title={allPlugins.length === 0 ? '暂无已安装插件' : '没有匹配的插件'}
            message={allPlugins.length === 0 ? '可以从插件市场安装当前支持的插件。' : '请调整筛选条件。'}
          />
        </div>
      ) : (
        <div className="table-scroll resource-table-scroll">
          <table className="table resource-table plugin-table">
            <thead><tr><th>插件名称</th><th>版本</th><th>安装状态</th><th>更新时间</th><th>操作</th></tr></thead>
            <tbody>
              {plugins.map((plugin) => (
                <InstalledPluginRow
                  key={plugin.id}
                  plugin={plugin}
                  catalogItem={catalog.find((item) => item.sourceID === plugin.sourceID && item.package === plugin.package)}
                  onDetail={onDetail}
                  onUpgrade={onUpgrade}
                  onDelete={onDelete}
                />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}

function InstalledPluginRow({
  plugin,
  catalogItem,
  onDetail,
  onUpgrade,
  onDelete,
}: {
  plugin: WasmPlugin;
  catalogItem?: PluginCatalogItem;
  onDetail: (plugin: WasmPlugin) => void;
  onUpgrade: (plugin: WasmPlugin, item: PluginCatalogItem) => void;
  onDelete: (plugin: WasmPlugin) => void;
}) {
  const upgradeAvailable = plugin.upgradeAvailable && catalogItem;
  return (
    <tr>
      <td><div className="table-primary">{plugin.name}</div><div className="table-secondary">{plugin.sourceName || '来源已删除'}</div></td>
      <td>
        <div className="table-primary">v{plugin.pluginVersion}</div>
        <div className="table-secondary">
          {plugin.upgradeAvailable && plugin.latestVersion ? `可升级至 v${plugin.latestVersion}` : catalogItem?.category ?? plugin.package}
        </div>
      </td>
      <td><Badge tone={pluginStatusTone(plugin.state)}>{pluginStatusLabel(plugin.state)}</Badge>{plugin.message ? <div className="table-secondary plugin-status-message">{plugin.message}</div> : null}</td>
      <td className="resource-table-time">{formatDateTime(plugin.updatedAt || plugin.createdAt)}</td>
      <td><RowActions onDetail={() => onDetail(plugin)} onEdit={upgradeAvailable ? () => onUpgrade(plugin, catalogItem) : undefined} editLabel="升级" onDelete={() => onDelete(plugin)} deleteLabel="卸载" /></td>
    </tr>
  );
}

export function PluginDetail({ plugin, catalogItem }: { plugin: WasmPlugin; catalogItem?: PluginCatalogItem }) {
  return (
    <div className="space-y-5">
      <section className="resource-detail-hero">
        <div><h3>{plugin.name}</h3><p>{plugin.sourceName ? `来自 ${plugin.sourceName}` : '插件来源已删除'}</p></div>
        <Badge tone={pluginStatusTone(plugin.state)}>{pluginStatusLabel(plugin.state)}</Badge>
      </section>
      <section className="resource-detail-section">
        <h3>安装信息</h3>
        <div className="resource-detail-grid">
          <div><span>插件包名</span><strong>{plugin.package}</strong></div>
          <div><span>插件来源</span><strong>{plugin.sourceName || '来源已删除'}</strong></div>
          <div><span>插件版本</span><strong>{plugin.pluginVersion}</strong></div>
          <div><span>安装状态</span><strong>{pluginStatusLabel(plugin.state)}</strong></div>
          <div><span>更新时间</span><strong>{formatDateTime(plugin.updatedAt || plugin.createdAt)}</strong></div>
          <div><span>创建时间</span><strong>{formatDateTime(plugin.createdAt)}</strong></div>
          <div><span>拉取策略</span><strong>{pullPolicyLabel(plugin.pullPolicy)}</strong></div>
        </div>
        {plugin.message ? <p className="plugin-detail-message">{plugin.message}</p> : null}
      </section>
      <section className="resource-detail-section space-y-3">
        <div className="plugin-detail-section-heading">
          <h3>模块信息</h3>
          {catalogItem ? <a href={catalogItem.sourceURL} target="_blank" rel="noreferrer">查看源码<ExternalLink aria-hidden="true" /></a> : null}
        </div>
        <div className="plugin-module-detail">
          <span>模块地址</span><code>{plugin.url}</code>
          {plugin.sha256 ? <><span>校验摘要</span><code>{plugin.sha256}</code></> : null}
        </div>
      </section>
      <section className="resource-detail-section space-y-3">
        <h3>使用情况</h3>
        {plugin.usages.length ? (
          <ul className="plugin-usage-list">
            {plugin.usages.map((usage) => (
              <li key={`${usage.policyKind}:${usage.policyID}`}>
                <strong>{usage.policyName}</strong>
                <span>{pluginPolicyKindLabel(usage.policyKind)}</span>
              </li>
            ))}
          </ul>
        ) : <p className="plugin-usage-empty">当前没有策略依赖该插件</p>}
      </section>
    </div>
  );
}

function pluginPolicyKindLabel(kind: WasmPlugin['usages'][number]['policyKind']): string {
  return kind === 'HeaderTransformationPolicy' ? '请求响应转换策略' : '模拟响应策略';
}

function pullPolicyLabel(policy: WasmPlugin['pullPolicy']): string {
  return policy === 'WASM_PLUGIN_PULL_POLICY_ALWAYS' ? '每次更新时重新拉取' : '缓存中不存在时拉取';
}

function pluginFilterSummary(filters: PluginFilters): string {
  const conditions = [];
  if (filters.query.trim()) conditions.push(`关键词“${filters.query.trim()}”`);
  if (filters.state !== 'all') conditions.push(`安装状态：${pluginStatusLabel(filters.state)}`);
  return conditions.join(' · ') || '全部已安装插件';
}
