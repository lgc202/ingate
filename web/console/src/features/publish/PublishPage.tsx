import { useState } from 'react';
import { consoleRepository } from '@/api/client';
import { useResource } from '@/api/useResource';
import { Badge, Button, PageFrame, Panel, ResourceStatePanel } from '@/components/ui';
import { formatDateTime, statusTone } from '@/domain/common';
import type { PublishDiagnostic, PublishSnapshot, SnapshotStatus } from '@/domain/publish';
import { snapshotStatusLabel } from '@/domain/publish';

const loadPublishSnapshots = () => consoleRepository.listPublishSnapshots();

export function PublishPage() {
  const snapshots = useResource(loadPublishSnapshots);
  const [query, setQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState<'all' | SnapshotStatus>('all');
  const [selectedSnapshotId, setSelectedSnapshotId] = useState('');
  const [mode, setMode] = useState<'list' | 'detail'>('list');

  if (snapshots.loading) {
    return (
      <PageFrame title="运行状态" subtitle="查看配置自动生效、运行配置生成和运行时回执状态">
        <ResourceStatePanel title="加载运行状态" message="正在读取运行配置和生效诊断。" />
      </PageFrame>
    );
  }

  if (snapshots.error || !snapshots.data) {
    return (
      <PageFrame title="运行状态" subtitle="查看配置自动生效、运行配置生成和运行时回执状态">
        <ResourceStatePanel title="运行状态加载失败" message={snapshots.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  const latest = snapshots.data.snapshots[0];
  const selectedSnapshot = snapshots.data.snapshots.find((snapshot) => snapshot.id === selectedSnapshotId) ?? latest;
  const visibleSnapshots = snapshots.data.snapshots.filter((snapshot) => {
    const normalizedQuery = query.trim().toLowerCase();
    const matchedQuery = !normalizedQuery || [snapshot.gateway, snapshot.version, snapshot.name].some((value) => value.toLowerCase().includes(normalizedQuery));
    const matchedStatus = statusFilter === 'all' || snapshot.status === statusFilter;

    return matchedQuery && matchedStatus;
  });

  if (mode === 'detail') {
    return (
      <PageFrame
        title="生效详情"
        subtitle={selectedSnapshot?.name ?? '未选择运行配置'}
        actions={<Button variant="soft" onClick={() => setMode('list')}>返回列表</Button>}
      >
        <section className="grid-main">
          <Panel title="基础信息">
            {selectedSnapshot ? <SnapshotDetail snapshot={selectedSnapshot} /> : null}
          </Panel>
          <Panel title="运行诊断">
            <div className="drawer-list">
              {snapshots.data.diagnostics.map((item) => (
                <DiagnosticLine key={item.label} item={item} />
              ))}
            </div>
          </Panel>
        </section>
      </PageFrame>
    );
  }

  return (
    <PageFrame
      title="运行状态"
      subtitle="保存配置后系统自动生成运行配置并下发，这里用于查看生效状态和失败诊断"
      actions={
        <>
          <Button variant="soft" onClick={() => {
            setQuery('');
            setStatusFilter('all');
          }}>重置筛选</Button>
        </>
      }
    >
        <Panel
          title="运行配置"
          subtitle="运行配置由后端根据网关、路由、服务和策略自动编译生成；用户保存后无需额外操作"
          actions={
            <div className="table-toolbar">
              <input className="toolbar-input" value={query} placeholder="搜索网关 / 版本" onChange={(event) => setQuery(event.target.value)} />
              <select className="toolbar-select" value={statusFilter} onChange={(event) => setStatusFilter(event.target.value as typeof statusFilter)}>
                <option value="all">状态：全部</option>
                <option value="generated">已生成</option>
                <option value="published">已生效</option>
                <option value="failed">生效失败</option>
                <option value="unknown">未知</option>
              </select>
            </div>
          }
        >
          <div style={{ overflow: 'auto' }}>
            <table className="table">
              <thead>
                <tr>
                  <th>网关</th>
                  <th>生效目标</th>
                  <th>版本</th>
                  <th>资源摘要</th>
                  <th>状态</th>
                  <th>生成时间</th>
                  <th>说明</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {visibleSnapshots.map((snapshot) => (
                  <SnapshotRow
                    key={snapshot.id}
                    snapshot={snapshot}
                    selected={snapshot.id === selectedSnapshot?.id}
                    onSelect={() => setSelectedSnapshotId(snapshot.id)}
                    onDetail={() => {
                      setSelectedSnapshotId(snapshot.id);
                      setMode('detail');
                    }}
                  />
                ))}
              </tbody>
            </table>
          </div>
        </Panel>
    </PageFrame>
  );
}

function SnapshotRow({
  snapshot,
  selected,
  onSelect,
  onDetail,
}: {
  snapshot: PublishSnapshot;
  selected: boolean;
  onSelect: () => void;
  onDetail: () => void;
}) {
  return (
    <tr className={selected ? 'selected' : ''} onClick={onSelect}>
      <td>{snapshot.gateway}</td>
      <td>{snapshot.target === 'xds' ? '默认运行时' : '调试目标'}</td>
      <td>{snapshot.version}</td>
      <td>
        {snapshot.routeCount} 条路由 / {snapshot.clusterCount} 个服务 / {snapshot.endpointCount} 个端点
      </td>
      <td>
        <Badge tone={snapshot.status === 'published' || snapshot.status === 'generated' ? 'green' : snapshot.status === 'failed' ? 'red' : 'neutral'}>
          {snapshotStatusLabel(snapshot.status)}
        </Badge>
      </td>
      <td>{formatDateTime(snapshot.createdAt)}</td>
      <td>{snapshot.message}</td>
      <td>
        <div className="row-actions">
          <button className="link-button" type="button" onClick={(event) => {
            event.stopPropagation();
            onDetail();
          }}>详情</button>
        </div>
      </td>
    </tr>
  );
}

function SnapshotDetail({ snapshot }: { snapshot: PublishSnapshot }) {
  const rows = [
    ['网关', snapshot.gateway],
    ['生效目标', snapshot.target === 'xds' ? '默认运行时' : '调试目标'],
    ['版本', snapshot.version],
    ['路由数', String(snapshot.routeCount)],
    ['服务数', String(snapshot.clusterCount)],
    ['端点数', String(snapshot.endpointCount)],
    ['状态', snapshotStatusLabel(snapshot.status)],
    ['生成时间', formatDateTime(snapshot.createdAt)],
  ];

  return (
    <div className="detail-card">
      <div className="kv">
        {rows.flatMap(([label, value]) => [
          <div key={`${label}-label`}>{label}</div>,
          <div key={`${label}-value`}>{value}</div>,
        ])}
      </div>
    </div>
  );
}

function DiagnosticLine({ item }: { item: PublishDiagnostic }) {
  return (
    <div className="mini-card">
      <div className="legend-row">
        <span>{item.label}</span>
        <Badge tone={statusTone(item.status)}>{item.value}</Badge>
      </div>
    </div>
  );
}
