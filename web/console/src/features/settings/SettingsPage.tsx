import { useMemo } from 'react';
import { Link, useParams } from 'react-router-dom';
import { systemNav } from '@/app/navigation';
import { consoleRepository } from '@/api/client';
import { useResource } from '@/api/useResource';
import { Badge, Button, PageFrame, Panel, ResourceStatePanel } from '@/components/ui';
import { healthLabel, statusTone } from '@/domain/common';
import type { SettingsInspectorData, SettingsSectionData, SettingsTable, ToggleGroup } from '@/domain/settings';

const sectionMap: Record<string, string> = {
  users: '用户权限',
  certificates: '证书',
  audit: '审计日志',
  notifications: '通知',
  params: '系统参数',
};

const dataSectionMap: Record<string, string> = {
  users: 'users-roles',
  certificates: 'global-security',
  audit: 'audit',
  notifications: 'notifications',
  params: 'system-params',
};

const loadSettings = () => consoleRepository.getSettingsWorkspace();

export function SettingsPage() {
  const { section = 'users' } = useParams();
  const active = useMemo(() => sectionMap[section] ?? '用户权限', [section]);
  const settings = useResource(loadSettings);

  if (settings.loading) {
    return (
      <PageFrame title="系统" subtitle="管理用户权限、证书、审计日志、通知和系统参数">
        <ResourceStatePanel title="加载系统数据" message="正在读取权限、审计和系统配置。" />
      </PageFrame>
    );
  }

  if (settings.error || !settings.data) {
    return (
      <PageFrame title="系统" subtitle="管理用户权限、证书、审计日志、通知和系统参数">
        <ResourceStatePanel title="系统数据加载失败" message={settings.error?.message ?? '请稍后重试。'} />
      </PageFrame>
    );
  }

  const dataSectionKey = dataSectionMap[section] ?? 'users-roles';
  const activeSection = settings.data.sections[dataSectionKey] ?? settings.data.sections['users-roles'];

  return (
    <PageFrame
      title="系统"
      subtitle="管理用户权限、证书、审计日志、通知和系统参数"
      actions={<Button variant="primary">新增配置</Button>}
    >
      <section className="grid-main">
        <Panel title={active}>
          <div className="composer" style={{ gridTemplateColumns: '220px minmax(0, 1fr)' }}>
            <div className="drawer">
              <h3 className="drawer-title">设置分类</h3>
              <div className="drawer-list">
                {systemNav.map((item) => (
                  <Link
                    key={item.key}
                    className={`nav-item ${item.to.endsWith(section) ? 'active' : ''}`}
                    to={item.to}
                    style={{
                      color: 'var(--text)',
                      background: item.to.endsWith(section) ? 'rgba(47,107,79,.12)' : '#fff',
                      border: `1px solid ${item.to.endsWith(section) ? 'rgba(47,107,79,.16)' : 'var(--line)'}`,
                    }}
                  >
                    <item.icon className="nav-icon" style={{ color: 'var(--green)' }} />
                    <span>{item.label}</span>
                  </Link>
                ))}
              </div>
            </div>
            <div className="panel" style={{ margin: 0 }}>
              <SettingContent section={activeSection} />
            </div>
          </div>
        </Panel>
        <section className="right-stack">
          <SettingInspector inspector={settings.data.inspector} />
        </section>
      </section>
    </PageFrame>
  );
}

function SettingContent({ section }: { section: SettingsSectionData }) {
  if (section.table && section.toggleGroups?.length) {
    return (
      <div className="composer" style={{ gridTemplateColumns: '1fr 0.9fr' }}>
        <TablePanel table={section.table} />
        {section.toggleGroups.map((group) => (
          <TogglePanel key={group.title} group={group} />
        ))}
      </div>
    );
  }

  if (section.table) {
    return <TablePanel table={section.table} />;
  }

  if (section.toggleGroups?.length && section.keyValues?.length) {
    return (
      <div className="composer" style={{ gridTemplateColumns: '1fr 0.9fr' }}>
        {section.toggleGroups.map((group) => (
          <TogglePanel key={group.title} group={group} />
        ))}
        <KeyValuePanel title="风险概览" values={section.keyValues} />
      </div>
    );
  }

  if (section.toggleGroups?.length) {
    return (
      <div className="field-grid">
        {section.toggleGroups.map((group) => (
          <TogglePanel key={group.title} group={group} />
        ))}
      </div>
    );
  }

  return (
    <Panel title={section.title}>
      <div className="mini-card">
        <div className="mini-card-meta">暂无配置数据。</div>
      </div>
    </Panel>
  );
}

function TablePanel({ table }: { table: SettingsTable }) {
  return (
    <Panel title={table.title} subtitle={table.subtitle}>
      <div style={{ overflow: 'auto' }}>
        <table className="table">
          <thead>
            <tr>
              {table.headers.map((header) => (
                <th key={header}>{header}</th>
              ))}
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {table.rows.map((row) => (
              <tr key={row[0]}>
                {row.map((cell) => (
                  <td key={cell}>{cell}</td>
                ))}
                <td>
                  <Badge>详情</Badge> <Badge>编辑</Badge>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Panel>
  );
}

function TogglePanel({ group }: { group: ToggleGroup }) {
  return (
    <Panel title={group.title} subtitle={group.subtitle}>
      <div className="drawer-list">
        {group.items.map((item) => (
          <label className="toggle" key={item.label}>
            <span className={`switch ${item.enabled ? 'on' : ''}`} />
            {item.label}
          </label>
        ))}
      </div>
    </Panel>
  );
}

function KeyValuePanel({ title, values }: { title: string; values: SettingsSectionData['keyValues'] }) {
  return (
    <Panel title={title}>
      <div className="kv">
        {(values ?? []).flatMap((item) => [
          <div key={`${item.label}-label`}>{item.label}</div>,
          <div key={`${item.label}-value`}>{item.status ? <Badge tone={statusTone(item.status)}>{item.value}</Badge> : item.value}</div>,
        ])}
      </div>
    </Panel>
  );
}

function SettingInspector({ inspector }: { inspector: SettingsInspectorData }) {
  return (
    <>
      <Panel title="当前配置域">
        <div className="kv">
          {inspector.configurationDomain.flatMap((item) => [
            <div key={`${item.label}-label`}>{item.label}</div>,
            <div key={`${item.label}-value`}>{item.status ? <Badge tone={statusTone(item.status)}>{item.value}</Badge> : item.value}</div>,
          ])}
        </div>
      </Panel>
      <Panel title="Envoy 实例健康">
        <div className="drawer-list">
          {inspector.envoyHealth.map((item) => (
            <div className="mini-card" key={item.label}>
              <div className="legend-row">
                <span>{item.label}</span>
                <Badge tone={statusTone(item.status)}>{healthLabel(item.status)}</Badge>
              </div>
            </div>
          ))}
        </div>
      </Panel>
      <Panel title="安全基线">
        <div className="drawer-list">
          {inspector.securityBaseline.map((item) => (
            <div className="mini-card" key={item.label}>
              <div className="legend-row">
                <span>{item.label}</span>
                <Badge tone={statusTone(item.status)}>已启用</Badge>
              </div>
            </div>
          ))}
        </div>
      </Panel>
    </>
  );
}
