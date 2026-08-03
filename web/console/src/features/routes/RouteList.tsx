import { useState } from 'react';
import { Badge, EmptyState, Panel } from '@/components/ui';
import type { PolicyWorkspace } from '@/domain/policy';
import type { RouteGatewayOption, RouteResource, UpstreamOption } from '@/domain/route';
import {
  formatGatewayIDs,
  formatMethods,
  primaryRouteRule,
  routeGovernancePolicyLabel,
  routeModelNames,
  routeUpstreamIDs,
  routeUpstreamLabels,
  routeUpstreamSummary,
} from './routeView';
import { Route as RouteIcon, Sparkles, Power, Edit3, Trash2 } from 'lucide-react';

type RouteEnabledFilter = 'all' | 'enabled' | 'disabled';

export function RouteList({
  routes,
  gateways,
  upstreams,
  selectedRouteID,
  policyWorkspace,
  onSelect,
  onCreate,
  onEdit,
  onRequestDisable,
  onRequestDelete,
  onToggleEnabled,
}: {
  routes: RouteResource[];
  gateways: RouteGatewayOption[];
  upstreams: UpstreamOption[];
  selectedRouteID: string;
  policyWorkspace?: PolicyWorkspace | null;
  onSelect: (id: string) => void;
  onCreate: () => void;
  onEdit: (route: RouteResource) => void;
  onRequestDisable: (route: RouteResource) => void;
  onRequestDelete: (route: RouteResource) => void;
  onToggleEnabled: (route: RouteResource) => void;
}) {
  const [keyword, setKeyword] = useState('');
  const [gatewayFilter, setGatewayFilter] = useState('all');
  const [upstreamFilter, setUpstreamFilter] = useState('all');
  const [enabledFilter, setEnabledFilter] = useState<RouteEnabledFilter>('all');

  const filteredRoutes = routes.filter((route) =>
    matchesFilter(
      route,
      { keyword, gatewayID: gatewayFilter, upstreamID: upstreamFilter, enabled: enabledFilter },
      gateways,
      upstreams,
    ),
  );

  return (
    <Panel
      actions={
        <div className="flex items-center gap-2">
          <input
            type="text"
            placeholder="搜索路由路径/名称..."
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            className="px-3 py-1.5 text-xs border border-slate-300 rounded-lg w-48 bg-white focus:outline-hidden"
          />
          <select
            value={gatewayFilter}
            onChange={(e) => setGatewayFilter(e.target.value)}
            className="px-3 py-1.5 text-xs border border-slate-300 rounded-lg bg-white focus:outline-hidden"
          >
            <option value="all">所有网关</option>
            {gateways.map((g) => (
              <option key={g.id} value={g.id}>
                {g.name}
              </option>
            ))}
          </select>
          <select
            value={upstreamFilter}
            onChange={(e) => setUpstreamFilter(e.target.value)}
            className="px-3 py-1.5 text-xs border border-slate-300 rounded-lg bg-white focus:outline-hidden"
          >
            <option value="all">所有目标服务</option>
            {upstreams.map((u) => (
              <option key={u.id} value={u.id}>
                {u.name}
              </option>
            ))}
          </select>
        </div>
      }
    >
      {filteredRoutes.length === 0 ? (
        <EmptyState title="暂无匹配的路由规则" message="尝试调整搜索或过滤条件" />
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs border-collapse">
            <thead>
              <tr className="border-b border-slate-200 text-slate-500 bg-slate-50/50 font-medium">
                <th className="py-2.5 px-3">路由名称 / ID</th>
                <th className="py-2.5 px-3">状态</th>
                <th className="py-2.5 px-3">匹配条件 (Path / Method)</th>
                <th className="py-2.5 px-3">所属 Gateway</th>
                <th className="py-2.5 px-3">转发目标 / 公开模型</th>
                <th className="py-2.5 px-3">策略标示</th>
                <th className="py-2.5 px-3 text-right">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 font-normal">
              {filteredRoutes.map((item) => {
                const isSelected = selectedRouteID === item.id;
                const rule = primaryRouteRule(item);
                const isAI = item.rules.some((r) => r.modelRouting && r.modelRouting.models.length > 0);
                const policyLabel = routeGovernancePolicyLabel(item, policyWorkspace);

                return (
                  <tr
                    key={item.id}
                    onClick={() => onSelect(item.id)}
                    className={`hover:bg-slate-50/80 transition-colors cursor-pointer ${
                      isSelected ? (isAI ? 'bg-purple-50/30' : 'bg-blue-50/30') : ''
                    }`}
                  >
                    <td className="py-3 px-3">
                      <div className="flex items-center gap-2">
                        {isAI ? (
                          <Sparkles className="w-4 h-4 text-purple-600 shrink-0" />
                        ) : (
                          <RouteIcon className="w-4 h-4 text-blue-600 shrink-0" />
                        )}
                        <div>
                          <div className="font-semibold text-slate-900">{item.name}</div>
                          <div className="text-[11px] font-mono text-slate-400">{item.id}</div>
                        </div>
                      </div>
                    </td>

                    <td className="py-3 px-3">
                      <button
                        type="button"
                        onClick={(e) => {
                          e.stopPropagation();
                          onToggleEnabled(item);
                        }}
                        className="focus:outline-hidden cursor-pointer"
                      >
                        <Badge tone={item.enabled ? 'success' : 'neutral'}>
                          <Power className="w-3 h-3" />
                          {item.enabled ? '已启用' : '已停用'}
                        </Badge>
                      </button>
                    </td>

                    <td className="py-3 px-3">
                      <div className="space-y-1">
                        <div className="font-mono text-xs text-slate-900 font-semibold">
                          {rule?.pathPrefix ?? '/'}
                        </div>
                        <div className="text-[10px] text-slate-400 font-mono">
                          {formatMethods(rule?.methods ?? [])}
                        </div>
                      </div>
                    </td>

                    <td className="py-3 px-3 font-mono text-[11px] text-slate-600">
                      {formatGatewayIDs(item.gatewayIDs, gateways)}
                    </td>

                    <td className="py-3 px-3">
                      {isAI ? (
                        <div className="flex flex-wrap gap-1">
                          {routeModelNames(item).map((m) => (
                            <span
                              key={m}
                              className="px-1.5 py-0.5 bg-purple-50 text-purple-700 text-[10px] font-mono rounded border border-purple-200/60"
                            >
                              {m}
                            </span>
                          ))}
                        </div>
                      ) : (
                        <div className="text-[11px] font-mono text-slate-700">
                          {routeUpstreamSummary(item, upstreams)}
                        </div>
                      )}
                    </td>

                    <td className="py-3 px-3">
                      {policyLabel !== '未挂载策略' ? (
                        <Badge tone="accent">{policyLabel}</Badge>
                      ) : (
                        <span className="text-slate-400 text-[11px]">-</span>
                      )}
                    </td>

                    <td className="py-3 px-3 text-right space-x-1" onClick={(e) => e.stopPropagation()}>
                      <button
                        type="button"
                        onClick={() => onEdit(item)}
                        className="p-1.5 text-slate-400 hover:text-blue-600 hover:bg-blue-50 rounded transition-colors cursor-pointer"
                        title="编辑"
                      >
                        <Edit3 className="w-3.5 h-3.5" />
                      </button>
                      <button
                        type="button"
                        onClick={() => onRequestDelete(item)}
                        className="p-1.5 text-slate-400 hover:text-rose-600 hover:bg-rose-50 rounded transition-colors cursor-pointer"
                        title="删除"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </Panel>
  );
}

function matchesFilter(
  route: RouteResource,
  filters: { keyword: string; gatewayID: string; upstreamID: string; enabled: RouteEnabledFilter },
  gateways: RouteGatewayOption[],
  upstreams: UpstreamOption[],
): boolean {
  const keyword = filters.keyword.trim().toLowerCase();
  const rule = primaryRouteRule(route);
  const matchedKeyword =
    !keyword ||
    [
      route.name,
      rule?.pathPrefix ?? '',
      ...route.gatewayIDs,
      ...route.gatewayIDs.map((gatewayID) => gateways.find((gateway) => gateway.id === gatewayID)?.name ?? gatewayID),
      ...routeUpstreamIDs(route),
      ...routeUpstreamLabels(route, upstreams),
      ...routeModelNames(route),
      ...route.hostnames,
    ].some((value) => value.toLowerCase().includes(keyword));

  return (
    matchedKeyword &&
    (filters.gatewayID === 'all' || route.gatewayIDs.includes(filters.gatewayID)) &&
    (filters.upstreamID === 'all' || routeUpstreamIDs(route).includes(filters.upstreamID)) &&
    (filters.enabled === 'all' || (filters.enabled === 'enabled' ? route.enabled : !route.enabled))
  );
}
