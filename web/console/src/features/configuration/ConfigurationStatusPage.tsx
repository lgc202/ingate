import { useEffect } from 'react';
import { Link } from 'react-router-dom';
import {
  AlertTriangle,
  Check,
  CirclePause,
  Clock3,
  RefreshCw,
  Cpu,
} from 'lucide-react';

import { getConfigurationStatus } from '@/api/configuration';
import { useResource } from '@/api/useResource';
import { Badge, PageFrame, Panel, ResourceStatePanel, StatCard } from '@/components/ui';
import type { ResourceState } from '@/domain/common';
import {
  configurationResourceKindLabel,
  type ConfigurationResourceKind,
} from '@/domain/configuration';

const defaultRefreshIntervalMs = 30000;
const errorRefreshIntervalMs = 12000;

export function ConfigurationStatusPage() {
  const status = useResource(getConfigurationStatus);

  const pollIntervalMs = !status.data
    ? 0
    : (status.data?.summary.error ?? 0) > 0
      ? errorRefreshIntervalMs
      : defaultRefreshIntervalMs;

  useEffect(() => {
    if (pollIntervalMs === 0) return;
    const timer = window.setInterval(() => status.reload(), pollIntervalMs);
    return () => window.clearInterval(timer);
  }, [pollIntervalMs, status]);

  if (status.loading && !status.data) {
    return (
      <PageFrame title="配置状态">
        <ResourceStatePanel title="正在抓取配置同步状态..." message="连接管理 API 检验中" />
      </PageFrame>
    );
  }

  if (status.error || !status.data) {
    return (
      <PageFrame title="配置状态">
        <ResourceStatePanel title="配置抓取失败" message={status.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  const { summary, items } = status.data;

  return (
    <PageFrame
      title="配置状态"
      subtitle="查看网关、路由、服务和治理策略的当前生效结果"
      actions={
        <button
          type="button"
          onClick={() => status.reload()}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold text-slate-700 bg-white border border-slate-300 hover:bg-slate-50 rounded-lg shadow-2xs transition-colors cursor-pointer"
        >
          <RefreshCw className={`w-3.5 h-3.5 ${status.loading ? 'animate-spin' : ''}`} />
          刷新
        </button>
      }
    >
      <div className="space-y-6 mt-4">
        {/* Metric Cards Top Row */}
        <div className="grid grid-cols-4 gap-4">
          <StatCard
            title="就绪配置"
            value={summary.ready}
            subvalue={`全量 ${summary.total} 项资源中就绪`}
            icon={Check}
            trend="同步正常"
          />
          <StatCard
            title="异常配置"
            value={summary.error}
            subvalue="存在校验或发布异常"
            icon={AlertTriangle}
          />
          <StatCard
            title="停用配置"
            value={summary.disabled}
            subvalue="已被手动禁用的规则"
            icon={Clock3}
          />
          <StatCard
            title="总声明资源数"
            value={summary.total}
            subvalue="Total Managed Resources"
            icon={Cpu}
          />
        </div>

        {/* Detailed Configuration Resource Table */}
        <Panel title="资源生效详情">
          {items.length === 0 ? (
            <div className="p-8 text-center text-xs text-slate-400">暂无生效状态记录</div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs border-collapse">
                <thead>
                  <tr className="border-b border-slate-200 text-slate-500 bg-slate-50/50 font-medium">
                    <th className="py-2.5 px-3">资源类型</th>
                    <th className="py-2.5 px-[12px]">资源 ID / 名称</th>
                    <th className="py-2.5 px-3">同步状态</th>
                    <th className="py-2.5 px-3 text-right">查看模块</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100 font-normal">
                  {items.map((item) => (
                    <tr key={`${item.kind}-${item.id}`} className="hover:bg-slate-50/80 transition-colors">
                      <td className="py-3 px-3">
                        <Badge tone="neutral">
                          {configurationResourceKindLabel(item.kind as ConfigurationResourceKind)}
                        </Badge>
                      </td>

                      <td className="py-3 px-3">
                        <div className="font-semibold text-slate-900">{item.name}</div>
                        <div className="text-[11px] font-mono text-slate-400">{item.id}</div>
                      </td>

                      <td className="py-3 px-3">
                        <ResourceStateBadge state={item.status.state} />
                      </td>

                      <td className="py-3 px-3 text-right">
                        <Link
                          to={kindRouteLink(item.kind)}
                          className="text-xs font-semibold text-blue-600 hover:text-blue-800"
                        >
                          跳至详情 →
                        </Link>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Panel>
      </div>
    </PageFrame>
  );
}

function ResourceStateBadge({ state }: { state: ResourceState }) {
  switch (state) {
    case 'Ready':
      return (
        <Badge tone="success">
          <Check className="w-3 h-3" /> 就绪
        </Badge>
      );
    case 'Error':
      return (
        <Badge tone="error">
          <AlertTriangle className="w-3 h-3" /> 错误
        </Badge>
      );
    case 'Disabled':
      return (
        <Badge tone="neutral">
          <CirclePause className="w-3 h-3" /> 已停用
        </Badge>
      );
    case 'Pending':
    default:
      return <Badge tone="warning">等待中</Badge>;
  }
}

function kindRouteLink(kind: string): string {
  switch (kind) {
    case 'Gateway': return '/gateways';
    case 'Route': return '/routes';
    case 'Upstream': return '/services';
    case 'Certificate': return '/certificates';
    case 'RateLimitPolicy':
    case 'IPRestrictionPolicy':
      return '/policies';
    default: return '/status';
  }
}
