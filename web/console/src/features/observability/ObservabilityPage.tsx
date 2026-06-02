import { consoleRepository } from '@/api/client';
import { useResource } from '@/api/useResource';
import { Badge, Button, PageFrame, Panel, ResourceStatePanel } from '@/components/ui';
import type { CountSegment, TimelineEvent } from '@/domain/common';
import { statusColor } from '@/domain/common';
import { linePath, sectionList } from '@/features/shared/charts';

const loadObservability = () => consoleRepository.getObservabilityOverview();

export function ObservabilityPage() {
  const overview = useResource(loadObservability);

  if (overview.loading) {
    return (
      <PageFrame title="观测" subtitle="查看流量运行态、路由质量、服务健康和 AI Token 用量">
        <ResourceStatePanel title="加载观测数据" message="正在读取请求趋势、调用日志和告警列表。" />
      </PageFrame>
    );
  }

  if (overview.error || !overview.data) {
    return (
      <PageFrame title="观测" subtitle="查看流量运行态、路由质量、服务健康和 AI Token 用量">
        <ResourceStatePanel title="观测数据加载失败" message={overview.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  return (
    <PageFrame
      title="观测"
      subtitle="查看流量运行态、路由质量、服务健康和 AI Token 用量"
      actions={
        <>
          <Badge>最近 15 分钟</Badge>
          <Button variant="soft">导出</Button>
        </>
      }
    >
      <section className="grid-main">
        <Panel title="观测概览">
          <div className="hero-cards" style={{ gridTemplateColumns: 'repeat(5, minmax(0, 1fr))' }}>
            {overview.data.metrics.map((metric) => (
              <SmallStat key={metric.label} {...metric} />
            ))}
          </div>
          <div style={{ height: 14 }} />
          <div className="composer" style={{ gridTemplateColumns: '1.2fr 1fr' }}>
            <div className="panel">
              <div className="panel-header">
                <h3 className="panel-title">请求率趋势</h3>
              </div>
              <svg viewBox="0 0 500 180" width="100%" height="180" aria-hidden="true">
                {linePath(overview.data.requestTrend)}
              </svg>
            </div>
            <div className="panel">
              <div className="panel-header">
                <h3 className="panel-title">最近调用日志</h3>
              </div>
              <div className="drawer-list">
                {sectionList(overview.data.callLogs.map((log) => [log.route, log.statusCode, log.result] as [string, string, string]))}
              </div>
            </div>
          </div>
        </Panel>

        <section className="right-stack">
          <Panel title="服务健康状态">
            <div className="donut" data-value="27" />
            <div className="legend">
              {overview.data.serviceHealth.map((item) => legendRow(item))}
            </div>
          </Panel>
          <Panel title="告警列表">
            <div className="drawer-list">
              {overview.data.alerts.map((alert) => miniLine(alert))}
            </div>
          </Panel>
        </section>
      </section>
    </PageFrame>
  );
}

function SmallStat({ label, value, meta, footer }: { label: string; value: string; meta: string; footer: string }) {
  return (
    <article className="stat-card">
      <div className="stat-label">{label}</div>
      <div className="stat-value">{value}</div>
      <div className="stat-meta">{meta}</div>
      <div style={{ marginTop: 14, display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 10 }}>
        <span className="badge">{footer}</span>
      </div>
    </article>
  );
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

function miniLine(alert: TimelineEvent) {
  return (
    <div className="mini-card">
      <div className="legend-row">
        <span className="legend-left">
          <span className="legend-dot" style={{ background: statusColor(alert.status) }} />
          {alert.title}
        </span>
        <span className="badge">{alert.time}</span>
      </div>
      <div className="mini-card-meta">{alert.description}</div>
    </div>
  );
}
