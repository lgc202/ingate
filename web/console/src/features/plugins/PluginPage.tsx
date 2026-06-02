import { useState } from 'react';
import { consoleRepository } from '@/api/client';
import { useResource } from '@/api/useResource';
import { Badge, Button, PageFrame, Panel, ResourceStatePanel } from '@/components/ui';
import type { HealthStatus } from '@/domain/common';
import { healthLabel, statusTone } from '@/domain/common';
import type { PluginResource } from '@/domain/plugin';

const loadPlugins = () => consoleRepository.listPlugins();
type PluginMode = 'list' | 'detail' | 'create' | 'edit';
type PluginDraft = Pick<PluginResource, 'name' | 'type' | 'version' | 'source' | 'deploymentScope' | 'healthStatus'>;

export function PluginPage() {
  const plugins = useResource(loadPlugins);
  const [mode, setMode] = useState<PluginMode>('list');
  const [selectedPluginId, setSelectedPluginId] = useState('auth-plugin');
  const [query, setQuery] = useState('');
  const [hiddenPluginIds, setHiddenPluginIds] = useState<string[]>([]);
  const [draft, setDraft] = useState<PluginDraft>({
    name: 'new-plugin',
    type: '认证',
    version: 'v1.0.0',
    source: '本地上传',
    deploymentScope: '按网关选择',
    healthStatus: 'unknown',
  });

  if (plugins.loading) {
    return (
      <PageFrame title="插件" subtitle="管理插件包、版本、部署范围和运行状态">
        <ResourceStatePanel title="加载插件数据" message="正在读取插件列表和运行状态。" />
      </PageFrame>
    );
  }

  if (plugins.error || !plugins.data) {
    return (
      <PageFrame title="插件" subtitle="管理插件包、版本、部署范围和运行状态">
        <ResourceStatePanel title="插件数据加载失败" message={plugins.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  const selectedPlugin = plugins.data.plugins.find((plugin) => plugin.id === selectedPluginId) ?? plugins.data.plugins[0];
  const visiblePlugins = plugins.data.plugins.filter((plugin) => {
    const normalizedQuery = query.trim().toLowerCase();
    const matchedQuery = !normalizedQuery || [plugin.name, plugin.type, plugin.version, plugin.deploymentScope].some((value) => value.toLowerCase().includes(normalizedQuery));

    return matchedQuery && !hiddenPluginIds.includes(plugin.id);
  });

  const openCreate = () => {
    setDraft({ name: 'new-plugin', type: '认证', version: 'v1.0.0', source: '本地上传', deploymentScope: '按网关选择', healthStatus: 'unknown' });
    setMode('create');
  };

  const openEdit = (plugin: PluginResource) => {
    setSelectedPluginId(plugin.id);
    setDraft({
      name: plugin.name,
      type: plugin.type,
      version: plugin.version,
      source: plugin.source,
      deploymentScope: plugin.deploymentScope,
      healthStatus: plugin.healthStatus,
    });
    setMode('edit');
  };

  if (mode === 'detail') {
    return (
      <PageFrame
        title="插件详情"
        subtitle={selectedPlugin?.name ?? '未选择插件'}
        actions={<Button variant="soft" onClick={() => setMode('list')}>返回列表</Button>}
      >
        <Panel title="基础信息">
          {selectedPlugin ? <PluginDetail plugin={selectedPlugin} /> : null}
        </Panel>
      </PageFrame>
    );
  }

  if (mode === 'create' || mode === 'edit') {
    return (
      <PageFrame
        title={mode === 'create' ? '新建插件部署' : '编辑插件部署'}
        subtitle="管理插件包版本和部署范围；插件只是执行能力的一种扩展形态"
        actions={<Button variant="soft" onClick={() => setMode('list')}>返回列表</Button>}
      >
        <Panel title={mode === 'create' ? '新建插件部署' : '编辑插件部署'} subtitle={draft.name}>
          <div className="editor-grid">
            <div className="field-grid">
              <InputField label="插件名称" value={draft.name} onChange={(value) => setDraft({ ...draft, name: value })} />
              <InputField label="类型" value={draft.type} onChange={(value) => setDraft({ ...draft, type: value })} />
              <InputField label="版本" value={draft.version} onChange={(value) => setDraft({ ...draft, version: value })} />
              <InputField label="来源" value={draft.source} onChange={(value) => setDraft({ ...draft, source: value })} />
              <InputField label="部署范围" value={draft.deploymentScope} onChange={(value) => setDraft({ ...draft, deploymentScope: value })} />
              <SelectField label="运行状态" value={draft.healthStatus} options={['healthy', 'warning', 'critical', 'unknown']} onChange={(value) => setDraft({ ...draft, healthStatus: value as HealthStatus })} />
            </div>
            <div className="detail-card editor-side-card">
              <h4>部署说明</h4>
              <div className="mini-card-meta">第一阶段只展示插件部署意图。后端接入后需要校验包签名、版本兼容性、部署范围和回滚策略。</div>
            </div>
            <div className="form-actions">
              <Button variant="soft">保存草稿</Button>
              <Button variant="primary" disabled={!draft.name.trim()}>提交变更</Button>
              <Button variant="ghost" onClick={() => setMode('list')}>取消</Button>
            </div>
          </div>
        </Panel>
      </PageFrame>
    );
  }

  return (
    <PageFrame
      title="插件"
      subtitle="管理插件包、版本、部署范围和运行状态"
      actions={
        <>
          <Button variant="soft" onClick={() => setQuery('')}>重置筛选</Button>
          <Button variant="primary" onClick={openCreate}>新建部署</Button>
        </>
      }
    >
        <Panel
          title="插件列表"
          actions={
            <div className="table-toolbar">
              <input className="toolbar-input" value={query} placeholder="搜索插件名称 / 类型 / 版本" onChange={(event) => setQuery(event.target.value)} />
            </div>
          }
        >
          <div style={{ overflow: 'auto' }}>
            <table className="table">
              <thead>
                <tr>
                  <th>插件名称</th>
                  <th>类型</th>
                  <th>当前版本</th>
                  <th>来源</th>
                  <th>校验摘要</th>
                  <th>部署范围</th>
                  <th>运行状态</th>
                  <th>被使用路由</th>
                  <th>最近更新</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {visiblePlugins.map((plugin) => (
                  <tr key={plugin.id} className={plugin.id === selectedPluginId ? 'selected' : ''} onClick={() => setSelectedPluginId(plugin.id)}>
                    <td>{plugin.name}</td>
                    <td>{plugin.type}</td>
                    <td>{plugin.version}</td>
                    <td>{plugin.source}</td>
                    <td>{plugin.checksum}</td>
                    <td>{plugin.deploymentScope}</td>
                    <td>
                      <Badge tone={statusTone(plugin.healthStatus)}>{healthLabel(plugin.healthStatus)}</Badge>
                    </td>
                    <td>{plugin.usedRoutes}</td>
                    <td>{plugin.lastUpdatedAt}</td>
                    <td>
                      <div className="row-actions">
                        <button className="link-button" type="button" onClick={(event) => {
                          event.stopPropagation();
                          setSelectedPluginId(plugin.id);
                          setMode('detail');
                        }}>详情</button>
                        <button className="link-button" type="button" onClick={(event) => {
                          event.stopPropagation();
                          openEdit(plugin);
                        }}>编辑</button>
                        <button className="link-button danger" type="button" onClick={(event) => {
                          event.stopPropagation();
                          setHiddenPluginIds((ids) => [...ids, plugin.id]);
                        }}>删除</button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Panel>
    </PageFrame>
  );
}

function PluginDetail({ plugin }: { plugin: PluginResource }) {
  const rows = [
    ['插件名称', plugin.name],
    ['类型', plugin.type],
    ['当前版本', plugin.version],
    ['来源', plugin.source],
    ['校验摘要', plugin.checksum],
    ['部署范围', plugin.deploymentScope],
    ['运行状态', healthLabel(plugin.healthStatus)],
    ['被使用路由', String(plugin.usedRoutes)],
    ['最近更新', plugin.lastUpdatedAt],
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

function InputField({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return (
    <div className="field">
      <label>{label}</label>
      <input value={value} onChange={(event) => onChange(event.target.value)} />
    </div>
  );
}

function SelectField({ label, value, options, onChange }: { label: string; value: HealthStatus; options: HealthStatus[]; onChange: (value: string) => void }) {
  return (
    <div className="field">
      <label>{label}</label>
      <select value={value} onChange={(event) => onChange(event.target.value)}>
        {options.map((option) => (
          <option key={option} value={option}>{healthLabel(option)}</option>
        ))}
      </select>
    </div>
  );
}
