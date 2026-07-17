import {
  AlertCircle,
  AlertTriangle,
  CircleHelp,
  FileText,
  GitBranch,
  Send,
  Server,
  ShieldCheck,
  SlidersHorizontal,
} from 'lucide-react';
import { Link } from 'react-router-dom';
import { consoleRepository } from '@/api/client';
import { useResource } from '@/api/useResource';
import { linePath, sectionList } from '@/features/shared/charts';
import { Badge, Panel, PageFrame, ResourceStatePanel } from '@/components/ui';
import type { CountSegment, HealthStatus, MetricCard, TimelineEvent } from '@/domain/common';
import { healthLabel, healthStatusClass, statusColor, statusTone } from '@/domain/common';
import type { ActionItem, DashboardContext, KeyLinkSummary } from '@/domain/home';

const loadHomeDashboard = () => consoleRepository.getHomeDashboard();

export function HomePage() {
  const dashboard = useResource(loadHomeDashboard);

  if (dashboard.loading) {
    return (
      <PageFrame title="首页" subtitle="聚焦生效状态、关键链路和第一阶段待处理事项">
        <ResourceStatePanel title="加载首页数据" message="正在读取网关、路由和服务运行状态。" />
      </PageFrame>
    );
  }

  if (dashboard.error || !dashboard.data) {
    return (
      <PageFrame title="首页" subtitle="聚焦生效状态、关键链路和第一阶段待处理事项">
        <ResourceStatePanel title="首页数据加载失败" message={dashboard.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  const { data } = dashboard;

  return (
    <PageFrame title="首页" subtitle="聚焦生效状态、关键链路和第一阶段待处理事项">
      <ContextBar context={data.context} />

      <section className="home-workbench-grid">
        <section className="home-main">
          <div className="home-metric-grid">
            {data.metrics.map((item) => (
              <WorkbenchMetric key={item.label} metric={item} />
            ))}
          </div>

          <Panel
            title="关键链路摘要"
            subtitle="展示当前范围内流量较高或存在风险的关键链路。"
            actions={
              <div className="panel-action-group">
                <Link className="text-action" to="/traffic/runtime">查看运行状态</Link>
                <Link className="panel-action-link" to="/traffic/routes">查看全部路由</Link>
              </div>
            }
          >
            <div className="link-matrix">
              <div className="link-matrix-heading">
                <span>网关</span>
                <span />
                <span>路由</span>
                <span />
                <span>服务</span>
                <span>状态</span>
              </div>
              {groupKeyLinksByGateway(data.keyLinks).map((group) => gatewayLinkGroup(group))}
            </div>
          </Panel>

          <div className="home-bottom-grid">
            <Panel title="运行趋势" subtitle="请求量与成功率趋势（最近 15 分钟）">
              <svg viewBox="0 0 500 180" width="100%" height="180" aria-hidden="true">
                {linePath(data.requestTrend)}
              </svg>
            </Panel>
            <Panel title="错误分布（Top 5）">
              <div className="drawer-list">
                {sectionList(data.errorDistribution)}
              </div>
            </Panel>
          </div>
        </section>

        <aside className="home-aside">
          <Panel title="待处理事项" subtitle="按优先级排序的待处理任务">
            <div className="action-list">
              {data.actionItems.map((item) => actionItemCard(item))}
            </div>
          </Panel>
          <Panel title="生效动态" subtitle="最近的自动生效与配置变更记录">
            <div className="publish-list">
              {data.changes.map((event) => changeCard(event))}
            </div>
          </Panel>
          <Panel title="健康状态摘要">
            <div className="legend">
              {data.healthSummary.map((item) => legendRow(item))}
            </div>
          </Panel>
        </aside>
      </section>
    </PageFrame>
  );
}

function WorkbenchMetric({ metric }: { metric: MetricCard }) {
  const Icon = metricIcon(metric.label);

  return (
    <article className={`workbench-metric ${metric.label.includes('风险') ? 'attention' : ''}`}>
      <div className="metric-icon">
        <Icon className="metric-icon-svg" />
      </div>
      <div className="metric-copy">
        <div className="metric-title">{metric.label}</div>
        <div className="metric-number">{metric.value}</div>
        <div className="metric-desc">{metric.label.includes('风险') ? '需关注与处理的风险项数量' : metric.footer}</div>
      </div>
    </article>
  );
}

function metricIcon(label: string) {
  if (label.includes('网关')) {
    return Server;
  }

  if (label.includes('路由')) {
    return GitBranch;
  }

  if (label.includes('服务')) {
    return ShieldCheck;
  }

  return AlertTriangle;
}

function ContextBar({ context }: { context: DashboardContext }) {
  return (
    <div className="context-bar">
      <span className="chip">部署形态：Docker all-in-one</span>
      <span className="chip">配置域：{context.configurationDomain}</span>
      <ContextSelect label="时间" value={context.timeRange} options={context.timeRangeOptions} />
    </div>
  );
}

function ContextSelect({ label, value, options = [value] }: { label: string; value: string; options?: string[] }) {
  return (
    <label className="context-select">
      <span>{label}：</span>
      <select defaultValue={value}>
        {options.map((option) => (
          <option key={option} value={option}>{option}</option>
        ))}
      </select>
    </label>
  );
}

function groupKeyLinksByGateway(links: KeyLinkSummary[]) {
  const groups = new Map<string, KeyLinkSummary[]>();

  for (const link of links) {
    const group = groups.get(link.gatewayName) ?? [];
    group.push(link);
    groups.set(link.gatewayName, group);
  }

  return Array.from(groups, ([gatewayName, groupLinks]) => ({ gatewayName, links: groupLinks }));
}

function gatewayLinkGroup(group: { gatewayName: string; links: KeyLinkSummary[] }) {
  const totalTraffic = group.links.map((link) => link.traffic).join(' / ');

  return (
    <div className="link-group" key={group.gatewayName}>
      <div className="link-node gateway">
        <strong>{group.gatewayName}</strong>
        <span>{group.links.length} 条关键路由</span>
        <small>{totalTraffic}</small>
      </div>
      <div className="link-group-routes">
        {group.links.map((link) => keyLinkRow(link))}
      </div>
    </div>
  );
}

function keyLinkRow(link: KeyLinkSummary) {
  return (
    <div className="link-matrix-row" key={link.id}>
      <span className="matrix-arrow">→</span>
      <div className="link-node route">
        <div>
          <span className={`method-pill ${link.routeMethod === 'POST' ? 'post' : ''}`}>{link.routeMethod}</span>
          <strong>{link.routePath}</strong>
        </div>
        <small>{link.successRate} / {link.latencyP95}</small>
      </div>
      <span className="matrix-arrow">→</span>
      <div className="link-node">
        <strong>{link.serviceName}</strong>
        <span>应用服务</span>
        <small>{link.traffic}</small>
      </div>
      <div className={`link-status ${healthStatusClass(link.status)}`}>
        <Badge tone={statusTone(link.status)}>{healthLabel(link.status)}</Badge>
      </div>
    </div>
  );
}

function actionItemCard(item: ActionItem) {
  const Icon = actionIcon(item.status);

  return (
    <div className={`action-card ${healthStatusClass(item.status)}`}>
      <div className="action-card-main">
        <div className="action-leading">
          <span className={`priority ${item.priority.toLowerCase()}`}>{item.priority}</span>
          <span className={`action-icon ${healthStatusClass(item.status)}`}>
            <Icon className="nav-icon" />
          </span>
        </div>
        <div className="action-copy">
          <strong>{item.title}</strong>
          <span>{item.description}</span>
          <small>{actionDelayText(item.id)}</small>
        </div>
        <Link className="link-button" to={actionTargetPath(item.target)}>查看</Link>
      </div>
    </div>
  );
}

function actionIcon(status: HealthStatus) {
  if (status === 'critical') {
    return AlertCircle;
  }

  if (status === 'unknown') {
    return CircleHelp;
  }

  if (status === 'warning') {
    return FileText;
  }

  return ShieldCheck;
}

function actionDelayText(id: string) {
  if (id.includes('route')) {
    return '最近发现：2 分钟前';
  }

  if (id.includes('service')) {
    return '最近发现：8 分钟前';
  }

  if (id.includes('runtime')) {
    return '创建时间：18 分钟前';
  }

  return '到期时间：7 天后';
}

function actionTargetPath(target: string) {
  if (target === 'route') {
    return '/traffic/routes';
  }

  if (target === 'service') {
    return '/traffic/services';
  }

  if (target === 'runtime') {
    return '/traffic/runtime';
  }

  return '/traffic/gateways';
}

function legendRow(item: CountSegment) {
  return (
    <div className="legend-row">
      <span className="legend-left">
        <span className="legend-dot" style={{ background: statusColor(item.status) }} />
        {item.label}
      </span>
      <span>{item.value}</span>
    </div>
  );
}

function changeCard(event: TimelineEvent) {
  const Icon = event.status === 'healthy' ? Send : event.status === 'warning' ? SlidersHorizontal : ShieldCheck;

  return (
    <div className="publish-item">
      <span className="timeline-dot" style={{ background: statusColor(event.status) }} />
      <div className="publish-time">{event.time}</div>
      <div className="publish-icon">
        <Icon className="nav-icon" />
      </div>
      <div className="publish-content">
        <div>
          <strong>{event.title}：</strong>
          <span>{event.description}</span>
        </div>
        <small>{event.status === 'healthy' ? '操作人：alex@ingate.io' : '更新人：system'}</small>
      </div>
      <Badge tone={statusTone(event.status)}>{event.title}</Badge>
    </div>
  );
}
