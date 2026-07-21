import { useEffect } from 'react';
import { Link } from 'react-router-dom';
import {
  AlertTriangle,
  Check,
  CirclePause,
  Clock3,
  RefreshCw,
} from 'lucide-react';

import { getConfigurationStatus } from '@/api/configuration';
import { useResource } from '@/api/useResource';
import { Badge, Button, PageFrame, Panel, ResourceStatePanel, type BadgeTone } from '@/components/ui';
import type { ResourceState } from '@/domain/common';
import {
  configurationResourceKindLabel,
  type ConfigurationResourceKind,
  type ConfigurationStatusItem,
  type ConfigurationStatusSummary,
} from '@/domain/configuration';

const loadConfigurationStatus = () => getConfigurationStatus();
const pendingRefreshIntervalMs = 4000;
const errorRefreshIntervalMs = 12000;

const resourceStateSummaries: Array<{
  state: ResourceState;
  summaryKey: Exclude<keyof ConfigurationStatusSummary, 'total'>;
}> = [
  { state: 'Ready', summaryKey: 'ready' },
  { state: 'Pending', summaryKey: 'pending' },
  { state: 'Error', summaryKey: 'error' },
  { state: 'Disabled', summaryKey: 'disabled' },
];

export function ConfigurationStatusPage() {
  const status = useResource(loadConfigurationStatus);
  const refreshIntervalMs = (status.data?.summary.pending ?? 0) > 0
    ? pendingRefreshIntervalMs
    : (status.data?.summary.error ?? 0) > 0
      ? errorRefreshIntervalMs
      : null;

  useEffect(() => {
    if (refreshIntervalMs === null || status.loading) {
      return undefined;
    }

    let disposed = false;
    let timer = 0;
    const refresh = async () => {
      await status.reload({ silent: true });
      if (!disposed) {
        timer = window.setTimeout(() => void refresh(), refreshIntervalMs);
      }
    };

    timer = window.setTimeout(() => void refresh(), refreshIntervalMs);
    return () => {
      disposed = true;
      window.clearTimeout(timer);
    };
  }, [refreshIntervalMs, status.loading, status.reload]);

  if (status.loading) {
    return (
      <PageFrame title="配置状态" subtitle="查看网关、路由、服务、证书和策略的处理结果">
        <ResourceStatePanel title="加载配置状态" message="正在读取各项资源的最新状态。" />
      </PageFrame>
    );
  }

  if (status.error || !status.data) {
    return (
      <PageFrame title="配置状态" subtitle="查看网关、路由、服务、证书和策略的处理结果">
        <ResourceStatePanel title="配置状态加载失败" message={status.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  return (
    <PageFrame
      title="配置状态"
      subtitle="系统会自动检查并处理保存的配置"
      actions={(
        <Button variant="soft" onClick={() => void status.reload()}>
          <RefreshCw size={15} strokeWidth={2} aria-hidden="true" />
          刷新状态
        </Button>
      )}
    >
      <ConfigurationOverview summary={status.data.summary} />
      <ConfigurationResourceList items={status.data.items} />
    </PageFrame>
  );
}

function ConfigurationOverview({ summary }: { summary: ConfigurationStatusSummary }) {
  const state = overviewState(summary);
  const Icon = state.icon;

  return (
    <section className={`configuration-status-overview ${state.tone}`}>
      <div className="configuration-status-summary">
        <div className="configuration-status-icon" aria-hidden="true">
          <Icon size={21} strokeWidth={2.2} />
        </div>
        <div>
          <span>当前配置 · 共 {summary.total} 项</span>
          <h2>{state.title}</h2>
          <p>{state.description}</p>
        </div>
      </div>
      <div className="configuration-status-counts">
        {resourceStateSummaries.map(({ state: resourceState, summaryKey }) => (
          <div key={resourceState} className={resourceState.toLowerCase()}>
            <span>{resourceStateLabel(resourceState)}</span>
            <strong>{summary[summaryKey]}</strong>
          </div>
        ))}
      </div>
    </section>
  );
}

function ConfigurationResourceList({
  items,
}: {
  items: ConfigurationStatusItem[];
}) {
  const emptyState = configurationEmptyState();

  return (
    <Panel
      title={items.length > 0 ? '资源状态' : emptyState.panelTitle}
      subtitle={items.length > 0 ? '需要处理的配置会优先显示' : emptyState.panelSubtitle}
    >
      {items.length > 0 ? (
        <div className="table-scroll configuration-status-table-scroll">
          <table className="table configuration-status-table">
            <thead>
              <tr>
                <th>资源名称</th>
                <th>类型</th>
                <th>状态</th>
                <th>说明</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {items.map((item) => (
                <tr key={`${item.kind}:${item.id}`}>
                  <td data-label="资源名称"><div className="table-primary">{item.name}</div></td>
                  <td data-label="类型">{configurationResourceKindLabel(item.kind)}</td>
                  <td data-label="状态">
                    <Badge tone={resourceStateTone(item.status.state)}>{resourceStateLabel(item.status.state)}</Badge>
                  </td>
                  <td data-label="说明" className="configuration-status-message">{resourceStatusMessage(item)}</td>
                  <td data-label="操作"><Link className="link-button" to={configurationResourcePath(item.kind)}>查看</Link></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <div className={`configuration-status-empty ${emptyState.tone}`}>
          <span aria-hidden="true"><CirclePause size={20} strokeWidth={2.5} /></span>
          <div>
            <strong>{emptyState.title}</strong>
            <p>{emptyState.description}</p>
          </div>
        </div>
      )}
    </Panel>
  );
}

function overviewState(summary: ConfigurationStatusSummary) {
  if (summary.error > 0) {
    return {
      tone: 'error',
      icon: AlertTriangle,
      title: `${summary.error} 项配置需要处理`,
      description: '请根据下方说明修改相关资源，保存后系统会自动重新处理。',
    };
  }
  if (summary.pending > 0) {
    return {
      tone: 'pending',
      icon: Clock3,
      title: `${summary.pending} 项配置正在处理`,
      description: '最近保存的配置尚未完成处理，页面会自动更新。',
    };
  }
  if (summary.total === 0) {
    return {
      tone: 'empty',
      icon: CirclePause,
      title: '还没有配置资源',
      description: '创建网关、路由或服务后，可以在这里查看处理结果。',
    };
  }
  if (summary.disabled === summary.total) {
    return {
      tone: 'disabled',
      icon: CirclePause,
      title: '所有资源均已停用',
      description: '当前没有启用中的配置；启用资源后，系统会重新检查并处理。',
    };
  }
  return {
    tone: 'ready',
    icon: Check,
    title: '当前配置状态正常',
    description: '所有已启用资源均已完成处理，没有发现需要修改的问题。',
  };
}

function configurationEmptyState() {
  return {
    tone: 'neutral',
    panelTitle: '资源状态',
    panelSubtitle: '还没有可检查的配置资源',
    title: '还没有配置资源',
    description: '先创建网关，再添加路由和服务；配置处理进度会统一显示在这里。',
  };
}

function configurationResourcePath(kind: ConfigurationResourceKind) {
  const paths: Record<ConfigurationResourceKind, string> = {
    Gateway: '/gateways',
    Route: '/routes',
    Upstream: '/services',
    Certificate: '/certificates',
    RateLimitPolicy: '/policies',
    AccessControlPolicy: '/policies',
  };
  return paths[kind];
}

function resourceStateLabel(state: ResourceState) {
  const labels: Record<ResourceState, string> = {
    Ready: '正常',
    Pending: '处理中',
    Error: '需要处理',
    Disabled: '已停用',
  };
  return labels[state];
}

function resourceStateTone(state: ResourceState): BadgeTone {
  const tones: Record<ResourceState, BadgeTone> = {
    Ready: 'success',
    Pending: 'warning',
    Error: 'danger',
    Disabled: 'neutral',
  };
  return tones[state];
}

function resourceStatusMessage(item: ConfigurationStatusItem) {
  if (item.status.message) {
    return item.status.message;
  }
  const messages: Record<ResourceState, string> = {
    Ready: '配置状态正常',
    Pending: '系统正在处理这项配置',
    Error: '配置存在问题，请检查后重试',
    Disabled: '资源已停用',
  };
  return messages[item.status.state];
}
