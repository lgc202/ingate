import { consoleRepository } from '@/api/client';
import { useResource } from '@/api/useResource';
import { Badge, Button, PageFrame, Panel, ResourceStatePanel } from '@/components/ui';
import { formatDateTime } from '@/domain/common';
import type { RuntimeNACK, RuntimeStatusView } from '@/domain/runtime';
import { deliveryStateLabel, deliveryStateTone } from '@/domain/runtime';

const loadRuntimeStatus = () => consoleRepository.getRuntimeStatus();

export function RuntimePage() {
  const status = useResource(loadRuntimeStatus);

  if (status.loading) {
    return (
      <PageFrame title="运行状态" subtitle="查看当前 Ingate 配置域到 Envoy 的配置交付状态">
        <ResourceStatePanel title="加载运行状态" message="正在读取 Controller 与 Envoy 的实时状态。" />
      </PageFrame>
    );
  }

  if (status.error || !status.data) {
    return (
      <PageFrame title="运行状态" subtitle="查看当前 Ingate 配置域到 Envoy 的配置交付状态">
        <ResourceStatePanel title="运行状态加载失败" message={status.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  const runtime = status.data;

  if (!runtime.available) {
    return (
      <PageFrame
        title="运行状态"
        subtitle="查看当前 Ingate 配置域到 Envoy 的配置交付状态"
        actions={<Button variant="soft" onClick={() => void status.reload()}>刷新</Button>}
      >
        <ResourceStatePanel title="运行状态暂不可用" message={runtime.message || 'Controller 暂时不可用，请稍后重试。'} />
      </PageFrame>
    );
  }

  return (
    <PageFrame
      title="运行状态"
      subtitle="一套 Ingate 使用一个配置域，所有已连接 Envoy 实例接收完全相同的配置"
      actions={<Button variant="soft" onClick={() => void status.reload()}>刷新</Button>}
    >
      <RuntimeOverview runtime={runtime} />
      <section className="grid-main">
        <Panel title="配置版本" subtitle="Candidate 编译完成后通过 xDS 下发，Envoy 完整确认后成为 Active">
          <div className="section-grid">
            <VersionCard label="Candidate" version={runtime.candidateVersion} empty="当前没有等待确认的配置" />
            <VersionCard label="Active" version={runtime.activeVersion} empty="当前还没有 Envoy 已确认的配置" />
          </div>
        </Panel>
        <section className="right-stack">
          <ACKPanel runtime={runtime} />
          <NACKPanel nack={runtime.lastNack} />
        </section>
      </section>
    </PageFrame>
  );
}

function RuntimeOverview({ runtime }: { runtime: RuntimeStatusView }) {
  return (
    <Panel title="当前配置域" subtitle={runtime.message || 'Controller 正常提供配置编译和 xDS 交付服务'}>
      <div className="insight-metrics">
        <StatusMetric label="Controller" value="在线" detail="运行状态可用" tone="green" />
        <StatusMetric
          label="配置准备"
          value={runtime.configReady ? '已就绪' : '未就绪'}
          detail={runtime.configReady ? '存在可下发的 Envoy 配置' : '当前没有可用 Envoy 配置'}
          tone={runtime.configReady ? 'green' : 'neutral'}
        />
        <StatusMetric
          label="交付状态"
          value={deliveryStateLabel(runtime.deliveryState)}
          detail={runtime.deliveryState}
          tone={deliveryStateTone(runtime.deliveryState)}
        />
        <StatusMetric
          label="已连接 Envoy"
          value={String(runtime.connectedEnvoys)}
          detail="共享当前配置域"
          tone={runtime.connectedEnvoys > 0 ? 'green' : 'amber'}
        />
      </div>
    </Panel>
  );
}

function StatusMetric({
  label,
  value,
  detail,
  tone,
}: {
  label: string;
  value: string;
  detail: string;
  tone: 'green' | 'amber' | 'red' | 'neutral';
}) {
  return (
    <article className="mini-card insight-metric">
      <div className="mini-card-meta">{label}</div>
      <strong>{value}</strong>
      <div><Badge tone={tone}>{detail}</Badge></div>
    </article>
  );
}

function VersionCard({ label, version, empty }: { label: string; version?: string; empty: string }) {
  return (
    <div className="detail-card">
      <h4>{label}</h4>
      <div className="mini-card-title">{version || '-'}</div>
      <div className="mini-card-meta">{version ? 'Envoy 配置版本' : empty}</div>
    </div>
  );
}

function ACKPanel({ runtime }: { runtime: RuntimeStatusView }) {
  const complete = runtime.ack.required > 0 && runtime.ack.received >= runtime.ack.required;

  return (
    <Panel title="ACK 进度">
      <div className="detail-card">
        <div className="kv">
          <div>已确认</div><div>{runtime.ack.received}</div>
          <div>需要确认</div><div>{runtime.ack.required}</div>
          <div>状态</div><div><Badge tone={complete ? 'green' : runtime.ack.required > 0 ? 'amber' : 'neutral'}>{ackLabel(runtime)}</Badge></div>
        </div>
      </div>
    </Panel>
  );
}

function ackLabel(runtime: RuntimeStatusView) {
  if (runtime.ack.required === 0) {
    return '暂无待确认配置';
  }
  if (runtime.ack.received >= runtime.ack.required) {
    return '已完整确认';
  }
  return `${runtime.ack.received}/${runtime.ack.required}`;
}

function NACKPanel({ nack }: { nack?: RuntimeNACK }) {
  return (
    <Panel title="最近 NACK">
      {nack ? (
        <div className="detail-card">
          <div className="kv">
            <div>Envoy</div><div>{nack.nodeID}</div>
            <div>版本</div><div>{nack.version || '-'}</div>
            <div>资源类型</div><div>{nack.typeURL}</div>
            <div>时间</div><div>{formatDateTime(nack.time)}</div>
          </div>
          <div className="mini-card-meta" style={{ marginTop: 12 }}>{nack.message}</div>
        </div>
      ) : (
        <div className="mini-card">
          <div className="mini-card-title">没有 NACK</div>
          <div className="mini-card-meta">当前未收到 Envoy 配置拒绝回执。</div>
        </div>
      )}
    </Panel>
  );
}
