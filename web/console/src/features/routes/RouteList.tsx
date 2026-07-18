import { useState } from 'react';
import { Badge, Button, EmptyState, Panel } from '@/components/ui';
import type { PolicyWorkspace } from '@/domain/policy';
import type { RouteGatewayOption, RouteResource, UpstreamOption } from '@/domain/route';
import {
  formatGatewayIDs,
  formatHostnames,
  formatMethods,
  primaryRouteRule,
  routeGovernancePolicyLabel,
  routeUpstreamIDs,
  routeUpstreamLabels,
  routeUpstreams,
  routeUpstreamSummary,
  upstreamLabel,
} from './routeView';

type RouteEnabledFilter = 'all' | 'enabled' | 'disabled';

interface RouteFilters {
  keyword: string;
  gatewayID: string;
  upstreamID: string;
  enabled: RouteEnabledFilter;
}

const emptyFilters: RouteFilters = {
  keyword: '',
  gatewayID: 'all',
  upstreamID: 'all',
  enabled: 'all',
};

export function RouteList({
  routes,
  gateways,
  upstreams,
  policyWorkspace,
  toggling,
  onDetail,
  onEdit,
  onDelete,
  onToggleEnabled,
}: {
  routes: RouteResource[];
  gateways: RouteGatewayOption[];
  upstreams: UpstreamOption[];
  policyWorkspace: PolicyWorkspace | null | undefined;
  toggling: boolean;
  onDetail: (route: RouteResource) => void;
  onEdit: (route: RouteResource) => void;
  onDelete: (route: RouteResource) => void;
  onToggleEnabled: (route: RouteResource) => void;
}) {
  const [filterDraft, setFilterDraft] = useState<RouteFilters>(emptyFilters);
  const [filters, setFilters] = useState<RouteFilters>(emptyFilters);
  const upstreamIDs = Array.from(new Set([
    ...upstreams.map((upstream) => upstream.id),
    ...routes.flatMap(routeUpstreamIDs),
  ])).sort();
  const visibleRoutes = routes.filter((route) => matchesFilters(route, filters, gateways, upstreams));
  const hasActiveFilters = Boolean(
    filters.keyword.trim()
    || filters.gatewayID !== 'all'
    || filters.upstreamID !== 'all'
    || filters.enabled !== 'all',
  );

  const resetFilters = () => {
    setFilterDraft(emptyFilters);
    setFilters(emptyFilters);
  };

  return (
    <Panel title="路由列表" subtitle={`${routes.length} 条路由 · 请求匹配后直接转发到目标服务`}>
      <div className="gateway-query">
        <div className="gateway-query-grid route-query-grid">
          <label className="query-control">
            <span>关键词</span>
            <input
              value={filterDraft.keyword}
              placeholder="搜索路由、路径、域名、目标服务或网关"
              onChange={(event) => setFilterDraft({ ...filterDraft, keyword: event.target.value })}
            />
          </label>
          <label className="query-control">
            <span>网关</span>
            <select value={filterDraft.gatewayID} onChange={(event) => setFilterDraft({ ...filterDraft, gatewayID: event.target.value })}>
              <option value="all">全部网关</option>
              {gateways.map((gateway) => <option key={gateway.id} value={gateway.id}>{gateway.name}</option>)}
            </select>
          </label>
          <label className="query-control">
            <span>服务</span>
            <select value={filterDraft.upstreamID} onChange={(event) => setFilterDraft({ ...filterDraft, upstreamID: event.target.value })}>
              <option value="all">全部目标服务</option>
              {upstreamIDs.map((upstreamID) => (
                <option key={upstreamID} value={upstreamID}>{upstreamLabel(upstreamID, upstreams)}</option>
              ))}
            </select>
          </label>
          <label className="query-control">
            <span>状态</span>
            <select
              value={filterDraft.enabled}
              onChange={(event) => setFilterDraft({ ...filterDraft, enabled: event.target.value as RouteEnabledFilter })}
            >
              <option value="all">全部</option>
              <option value="enabled">启用</option>
              <option value="disabled">停用</option>
            </select>
          </label>
        </div>
        <div className="query-actions">
          <Button variant="soft" onClick={resetFilters}>重置</Button>
          <Button variant="primary" onClick={() => setFilters(filterDraft)}>查询</Button>
        </div>
      </div>

      <div className="table-scroll route-table-scroll">
        <table className="table route-table">
          <thead>
            <tr>
              <th>路由名称</th>
              <th>匹配条件</th>
              <th>作用范围</th>
              <th>目标服务</th>
              <th>状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {visibleRoutes.map((route) => {
              const rule = primaryRouteRule(route);
              const editReason = route.rules.length === 1
                ? ''
                : '当前控制台暂不支持编辑包含多条规则的路由，请在原配置来源中修改';
              return (
                <tr key={route.id}>
                  <td>
                    <div className="table-primary">{route.name}</div>
                    <div className="table-secondary">{route.rules.length} 条规则{editReason ? ' · 控制台只读' : ''} · {routeGovernancePolicyLabel(route, policyWorkspace)}</div>
                  </td>
                  <td>
                    <div className="table-primary">
                      <Badge tone="accent">{formatMethods(rule?.methods ?? [])}</Badge>{' '}
                      {rule?.pathPrefix ?? '-'}
                    </div>
                    <div className="table-secondary">{rule?.headers.length ?? 0} 个请求头条件</div>
                  </td>
                  <td>
                    <div className="table-primary">{formatGatewayIDs(route.gatewayIDs, gateways)}</div>
                    <div className="table-secondary">域名：{formatHostnames(route.hostnames)}</div>
                  </td>
                  <td>
                    <div className="table-primary">{routeUpstreamSummary(route, upstreams)}</div>
                    <div className="table-secondary">{routeUpstreams(route).length} 个目标服务</div>
                  </td>
                  <td>
                    <div className={`gateway-status ${route.enabled ? 'on' : ''}`.trim()}>
                      <button
                        className="gateway-switch"
                        type="button"
                        role="switch"
                        disabled={toggling}
                        aria-checked={route.enabled}
                        aria-label={`${route.name} ${route.enabled ? '已启用' : '已停用'}`}
                        onClick={(event) => {
                          event.stopPropagation();
                          onToggleEnabled(route);
                        }}
                      ><span aria-hidden="true" /></button>
                      <strong>{route.enabled ? '启用' : '停用'}</strong>
                    </div>
                  </td>
                  <td>
                    <div className="row-actions">
                      <button className="link-button" type="button" onClick={(event) => {
                        event.stopPropagation();
                        onDetail(route);
                      }}>详情</button>
                      <button className="link-button" type="button" onClick={(event) => {
                        event.stopPropagation();
                        onEdit(route);
                      }} disabled={Boolean(editReason)} title={editReason || undefined}>编辑</button>
                      <button className="link-button danger" type="button" onClick={(event) => {
                        event.stopPropagation();
                        onDelete(route);
                      }}>删除</button>
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {visibleRoutes.length === 0 ? (
        <div className="table-empty">
          <EmptyState
            title={hasActiveFilters ? '没有匹配的路由' : '暂无路由'}
            message={hasActiveFilters ? '调整查询条件后再试，或重置筛选查看全部路由。' : '新建路由后，请求就可以从网关转发到目标服务。'}
          />
        </div>
      ) : null}
    </Panel>
  );
}

function matchesFilters(
  route: RouteResource,
  filters: RouteFilters,
  gateways: RouteGatewayOption[],
  upstreams: UpstreamOption[],
): boolean {
  const keyword = filters.keyword.trim().toLowerCase();
  const rule = primaryRouteRule(route);
  const matchedKeyword = !keyword || [
    route.name,
    rule?.pathPrefix ?? '',
    ...route.gatewayIDs,
    ...route.gatewayIDs.map((gatewayID) => gateways.find((gateway) => gateway.id === gatewayID)?.name ?? gatewayID),
    ...routeUpstreamIDs(route),
    ...routeUpstreamLabels(route, upstreams),
    ...route.hostnames,
  ].some((value) => value.toLowerCase().includes(keyword));

  return matchedKeyword
    && (filters.gatewayID === 'all' || route.gatewayIDs.includes(filters.gatewayID))
    && (filters.upstreamID === 'all' || routeUpstreamIDs(route).includes(filters.upstreamID))
    && (filters.enabled === 'all' || (filters.enabled === 'enabled' ? route.enabled : !route.enabled));
}
