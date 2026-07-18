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
  type ConfigurationStatusItem,
  type ConfigurationStatusView,
} from '@/domain/configuration';

const loadConfigurationStatus = () => getConfigurationStatus();
const pendingRefreshIntervalMs = 4000;
const errorRefreshIntervalMs = 12000;
const resourceStates: ResourceState[] = ['Ready', 'Pending', 'Error', 'Disabled'];

type ResourceStateCounts = Record<ResourceState, number>;

export function ConfigurationStatusPage() {
  const status = useResource(loadConfigurationStatus);
  const hasPending = status.data?.items.some((item) => item.status.state === 'Pending') ?? false;
  const hasError = status.data?.items.some((item) => item.status.state === 'Error') ?? false;
  const refreshIntervalMs = hasPending
    ? pendingRefreshIntervalMs
    : hasError
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

  const counts = countResourceStates(status.data);
  const attentionItems = status.data.items.filter((item) => item.status.state === 'Error' || item.status.state === 'Pending');

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
      <ConfigurationOverview counts={counts} total={status.data.items.length} />
      <AttentionList items={attentionItems} counts={counts} total={status.data.items.length} />
    </PageFrame>
  );
}

function ConfigurationOverview({ counts, total }: { counts: ResourceStateCounts; total: number }) {
  const state = overviewState(counts, total);
  const Icon = state.icon;

  return (
    <section className={`configuration-status-overview ${state.tone}`}>
      <div className="configuration-status-summary">
        <div className="configuration-status-icon" aria-hidden="true">
          <Icon size={21} strokeWidth={2.2} />
        </div>
        <div>
          <span>当前配置</span>
          <h2>{state.title}</h2>
          <p>{state.description}</p>
        </div>
      </div>
      <div className="configuration-status-counts">
        {resourceStates.map((resourceState) => (
          <div key={resourceState} className={resourceState.toLowerCase()}>
            <span>{resourceStateLabel(resourceState)}</span>
            <strong>{counts[resourceState]}</strong>
          </div>
        ))}
      </div>
    </section>
  );
}

function AttentionList({
  items,
  counts,
  total,
}: {
  items: ConfigurationStatusItem[];
  counts: ResourceStateCounts;
  total: number;
}) {
  const emptyState = attentionEmptyState(counts, total);
  const EmptyIcon = emptyState.icon;

  return (
    <Panel
      title={items.length > 0 ? '需要关注' : emptyState.panelTitle}
      subtitle={items.length > 0 ? '仅展示正在处理或需要修改的资源' : emptyState.panelSubtitle}
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
                  <td data-label="操作"><Link className="link-button" to={item.href}>查看</Link></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <div className={`configuration-status-empty ${emptyState.tone}`}>
          <span aria-hidden="true"><EmptyIcon size={20} strokeWidth={2.5} /></span>
          <div>
            <strong>{emptyState.title}</strong>
            <p>{emptyState.description}</p>
          </div>
        </div>
      )}
    </Panel>
  );
}

function countResourceStates(data: ConfigurationStatusView): ResourceStateCounts {
  const counts: ResourceStateCounts = { Ready: 0, Pending: 0, Error: 0, Disabled: 0 };
  for (const item of data.items) {
    counts[item.status.state]++;
  }
  return counts;
}

function overviewState(counts: ResourceStateCounts, total: number) {
  if (counts.Error > 0) {
    return {
      tone: 'error',
      icon: AlertTriangle,
      title: `${counts.Error} 项配置需要处理`,
      description: '请根据下方说明修改相关资源，保存后系统会自动重新处理。',
    };
  }
  if (counts.Pending > 0) {
    return {
      tone: 'pending',
      icon: Clock3,
      title: `${counts.Pending} 项配置正在处理`,
      description: '最近保存的配置尚未完成处理，页面会自动更新。',
    };
  }
  if (total === 0) {
    return {
      tone: 'empty',
      icon: CirclePause,
      title: '还没有配置资源',
      description: '创建网关、路由或服务后，可以在这里查看处理结果。',
    };
  }
  if (counts.Disabled === total) {
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

function attentionEmptyState(counts: ResourceStateCounts, total: number) {
  if (total === 0) {
    return {
      tone: 'neutral',
      icon: CirclePause,
      panelTitle: '配置检查结果',
      panelSubtitle: '还没有可检查的配置资源',
      title: '还没有配置资源',
      description: '先创建网关、路由或服务，之后可在这里查看处理进度。',
    };
  }
  if (counts.Disabled === total) {
    return {
      tone: 'neutral',
      icon: CirclePause,
      panelTitle: '配置检查结果',
      panelSubtitle: '当前没有启用中的配置资源',
      title: '所有资源均已停用',
      description: '启用网关、路由或策略后，系统会重新检查对应配置。',
    };
  }
  return {
    tone: 'ready',
    icon: Check,
    panelTitle: '配置检查结果',
    panelSubtitle: '当前没有需要处理的配置',
    title: '当前没有需要处理的配置',
    description: '已启用资源均已完成处理；后续配置变化会继续显示在这里。',
  };
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
