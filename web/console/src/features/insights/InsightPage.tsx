import { useState } from 'react';
import { Badge, Button, EmptyState, PageFrame, Panel } from '@/components/ui';

type InsightKind = 'assets' | 'risks' | 'governance';
type InsightStatus = 'normal' | 'warning' | 'critical' | 'processing' | 'done' | 'unknown';

interface InsightMetric {
  label: string;
  value: string;
  meta: string;
}

interface InsightRow {
  id: string;
  name: string;
  type: string;
  primary: string;
  secondary: string;
  owner: string;
  status: InsightStatus;
  statusText: string;
  updatedAt: string;
  evidence: string;
  suggestion: string;
}

interface InsightModel {
  title: string;
  subtitle: string;
  createLabel?: string;
  searchPlaceholder: string;
  allTypeLabel: string;
  metrics: InsightMetric[];
  columns: [string, string, string, string, string, string];
  rows: InsightRow[];
}

const insightModels: Record<InsightKind, InsightModel> = {
  assets: {
    title: '资产',
    subtitle: '从访问日志、路由配置和样本中识别 API、应用、用户、数据和文件资产。',
    createLabel: '确认资产',
    searchPlaceholder: '搜索资产名称 / 类型 / 归属',
    allTypeLabel: '全部资产',
    metrics: [
      { label: 'API 资产', value: '128', meta: '从路由与日志识别' },
      { label: '应用资产', value: '18', meta: '关联 54 条路由' },
      { label: '账号资产', value: '3.4w', meta: '最近 24 小时活跃' },
      { label: '文件资产', value: '7', meta: '下载 / 导出接口' },
    ],
    columns: ['资产名称', '类型', '发现来源', '关联对象', '状态', '最近发现'],
    rows: [
      {
        id: 'api-orders',
        name: 'POST /v1/orders',
        type: 'API 资产',
        primary: '网关路由 + 访问日志',
        secondary: 'order-svc / gw-public',
        owner: '订单应用',
        status: 'normal',
        statusText: '已确认',
        updatedAt: '5 分钟前',
        evidence: '路由配置命中 3.2k req/s，最近 15 分钟持续有调用。',
        suggestion: '可继续绑定身份认证、限流和请求校验策略。',
      },
      {
        id: 'api-inventory',
        name: 'GET /v1/inventory',
        type: 'API 资产',
        primary: '访问日志识别',
        secondary: 'inventory-svc / gw-sandbox',
        owner: '库存应用',
        status: 'warning',
        statusText: '待确认',
        updatedAt: '18 分钟前',
        evidence: '日志中持续出现请求，但路由成功率低于当前范围基线。',
        suggestion: '确认是否为有效 API，确认后进入风险分析和治理。',
      },
      {
        id: 'account-partner-a',
        name: 'partner-a',
        type: '账号资产',
        primary: 'API Key 识别',
        secondary: '12 条 API / 2 个应用',
        owner: '合作方接入',
        status: 'normal',
        statusText: '已确认',
        updatedAt: '34 分钟前',
        evidence: '通过 X-API-Key 与调用行为聚合为同一账号。',
        suggestion: '建议配置账号级限流和访问审计。',
      },
      {
        id: 'file-report',
        name: 'order-export.xlsx',
        type: '文件资产',
        primary: '响应内容识别',
        secondary: 'POST /v1/orders/export',
        owner: '订单应用',
        status: 'unknown',
        statusText: '待采样',
        updatedAt: '2 小时前',
        evidence: '响应头出现文件下载特征，内容采样尚未接入。',
        suggestion: '后续接入脱敏采样后再判断文件敏感级别。',
      },
    ],
  },
  risks: {
    title: '风险',
    subtitle: '围绕资产、主体和证据组织风险事件，并从事件进入治理动作。',
    searchPlaceholder: '搜索风险事件 / 资产 / 主体',
    allTypeLabel: '全部风险',
    metrics: [
      { label: '未处置风险', value: '8', meta: 'P1 2 个' },
      { label: '异常访问', value: '23', meta: '最近 15 分钟' },
      { label: '弱点事件', value: '5', meta: '涉及 3 个应用' },
      { label: '已生成证据', value: '41', meta: '可追溯到请求' },
    ],
    columns: ['风险事件', '类型', '影响资产', '触发主体', '状态', '最近发生'],
    rows: [
      {
        id: 'risk-inventory-error',
        name: '/v1/inventory 成功率异常下降',
        type: '服务风险',
        primary: 'GET /v1/inventory',
        secondary: 'inventory-svc',
        owner: '匿名用户段 10.3.18.0/24',
        status: 'critical',
        statusText: '未处置',
        updatedAt: '2 分钟前',
        evidence: '最近 15 分钟 5xx 占比 7.6%，高于历史基线 4.2 倍。',
        suggestion: '先查看路由和服务健康，必要时停用异常路由或切换目标服务。',
      },
      {
        id: 'risk-login-weak',
        name: '疑似弱密码登录入口',
        type: '弱点事件',
        primary: 'POST /login',
        secondary: 'user-svc',
        owner: 'user-web',
        status: 'warning',
        statusText: '待确认',
        updatedAt: '9 分钟前',
        evidence: '短时间内出现高频 401，账号枚举特征明显。',
        suggestion: '确认登录接口策略，必要时增加账号级限流和验证码策略。',
      },
      {
        id: 'risk-api-unknown',
        name: '未知 API 被访问',
        type: '资产风险',
        primary: 'PUT /internal/export',
        secondary: 'gw-public',
        owner: '未知主体',
        status: 'warning',
        statusText: '待确认',
        updatedAt: '42 分钟前',
        evidence: '访问日志出现路由未登记路径，响应状态为 200。',
        suggestion: '先确认是否为有效 API 资产，再决定下线、纳管或加入白名单。',
      },
    ],
  },
  governance: {
    title: '治理',
    subtitle: '把风险、资产和策略连接起来，对 API、账号、应用和服务执行可审计的处置动作。',
    createLabel: '新建治理动作',
    searchPlaceholder: '搜索治理对象 / 动作 / 来源',
    allTypeLabel: '全部动作',
    metrics: [
      { label: '待执行动作', value: '6', meta: '2 个高优先级' },
      { label: '自动处置', value: '14', meta: '最近 24 小时' },
      { label: '人工审批', value: '3', meta: '等待确认' },
      { label: '可回滚动作', value: '18', meta: '保留审计记录' },
    ],
    columns: ['治理对象', '动作', '触发来源', '执行方式', '状态', '最近更新'],
    rows: [
      {
        id: 'gov-limit-partner',
        name: 'partner-a',
        type: '账号限流',
        primary: '限流策略 100 req/min',
        secondary: '登录接口高频失败',
        owner: '自动执行',
        status: 'processing',
        statusText: '执行中',
        updatedAt: '刚刚',
        evidence: '来源风险：疑似弱密码登录入口；作用范围：partner-a 访问的登录路由。',
        suggestion: '等待运行状态回执，确认生效后继续观察 401 与 429 趋势。',
      },
      {
        id: 'gov-disable-api',
        name: 'PUT /internal/export',
        type: 'API 下线',
        primary: '停用路由',
        secondary: '未知 API 被访问',
        owner: '人工审批',
        status: 'warning',
        statusText: '待审批',
        updatedAt: '12 分钟前',
        evidence: '该 API 未在资产清单确认，且出现在公网网关访问日志中。',
        suggestion: '审批前应确认业务归属；若无归属，执行停用路由。',
      },
      {
        id: 'gov-block-ip',
        name: '10.3.18.0/24',
        type: '源 IP 封禁',
        primary: '加入临时黑名单 30 分钟',
        secondary: '异常访问',
        owner: '自动执行',
        status: 'done',
        statusText: '已完成',
        updatedAt: '28 分钟前',
        evidence: '来源风险：登录接口高频 401；执行后错误峰值已回落。',
        suggestion: '保留审计记录，过期后自动解除。',
      },
      {
        id: 'gov-add-auth',
        name: 'GET /v1/inventory',
        type: '策略加固',
        primary: '绑定身份认证策略',
        secondary: 'API 资产待确认',
        owner: '人工执行',
        status: 'unknown',
        statusText: '待配置',
        updatedAt: '1 小时前',
        evidence: '该 API 当前未绑定认证策略，且存在成功率异常。',
        suggestion: '确认资产归属后，在路由编辑页绑定认证和限流策略。',
      },
    ],
  },
};

export function InsightPage({ kind }: { kind: InsightKind }) {
  const model = insightModels[kind];
  const [mode, setMode] = useState<'list' | 'detail'>('list');
  const [query, setQuery] = useState('');
  const [typeFilter, setTypeFilter] = useState('all');
  const [selectedId, setSelectedId] = useState(model.rows[0]?.id ?? '');
  const [hiddenIds, setHiddenIds] = useState<string[]>([]);
  const selected = model.rows.find((row) => row.id === selectedId) ?? model.rows[0];
  const typeOptions = Array.from(new Set(model.rows.map((row) => row.type)));
  const visibleRows = model.rows.filter((row) => {
    const normalizedQuery = query.trim().toLowerCase();
    const matchedQuery = !normalizedQuery || [row.name, row.type, row.primary, row.secondary, row.owner].some((value) => value.toLowerCase().includes(normalizedQuery));
    const matchedType = typeFilter === 'all' || row.type === typeFilter;

    return matchedQuery && matchedType && !hiddenIds.includes(row.id);
  });
  const hasActiveFilters = Boolean(query.trim() || typeFilter !== 'all');

  if (mode === 'detail') {
    return (
      <PageFrame
        title={`${model.title}详情`}
        subtitle={selected?.name ?? '未选择对象'}
        actions={<Button variant="soft" onClick={() => setMode('list')}>返回列表</Button>}
      >
        {selected ? <InsightDetail model={model} row={selected} /> : null}
      </PageFrame>
    );
  }

  return (
    <PageFrame
      title={model.title}
      subtitle={model.subtitle}
      actions={
        <>
          <Button variant="soft" onClick={() => {
            setQuery('');
            setTypeFilter('all');
          }}>重置筛选</Button>
          {model.createLabel ? <Button variant="primary">{model.createLabel}</Button> : null}
        </>
      }
    >
      <div className="insight-metrics">
        {model.metrics.map((metric) => (
          <article className="mini-card insight-metric" key={metric.label}>
            <div className="mini-card-meta">{metric.label}</div>
            <strong>{metric.value}</strong>
            <span>{metric.meta}</span>
          </article>
        ))}
      </div>

      <Panel
        title={`${model.title}列表`}
        actions={
          <div className="table-toolbar">
            <input className="toolbar-input" value={query} placeholder={model.searchPlaceholder} onChange={(event) => setQuery(event.target.value)} />
            <select className="toolbar-select" value={typeFilter} onChange={(event) => setTypeFilter(event.target.value)}>
              <option value="all">{model.allTypeLabel}</option>
              {typeOptions.map((type) => (
                <option key={type} value={type}>{type}</option>
              ))}
            </select>
          </div>
        }
      >
        <div style={{ overflow: 'auto' }}>
          <table className="table">
            <thead>
              <tr>
                {model.columns.map((column) => (
                  <th key={column}>{column}</th>
                ))}
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {visibleRows.map((row) => (
                <tr key={row.id} className={row.id === selectedId ? 'selected' : ''} onClick={() => setSelectedId(row.id)}>
                  <td>
                    <div className="table-primary">{row.name}</div>
                    <div className="table-secondary">{row.type}</div>
                  </td>
                  <td>{row.type}</td>
                  <td>
                    <div className="table-primary">{row.primary}</div>
                    <div className="table-secondary">{row.evidence}</div>
                  </td>
                  <td>
                    <div className="table-primary">{row.secondary}</div>
                    <div className="table-secondary">{row.owner}</div>
                  </td>
                  <td><Badge tone={statusTone(row.status)}>{row.statusText}</Badge></td>
                  <td>{row.updatedAt}</td>
                  <td>
                    <div className="row-actions">
                      <button className="link-button" type="button" onClick={(event) => {
                        event.stopPropagation();
                        setSelectedId(row.id);
                        setMode('detail');
                      }}>详情</button>
                      <button className="link-button" type="button" onClick={(event) => {
                        event.stopPropagation();
                        setSelectedId(row.id);
                      }}>处理</button>
                      <button className="link-button danger" type="button" onClick={(event) => {
                        event.stopPropagation();
                        setHiddenIds((ids) => [...ids, row.id]);
                      }}>忽略</button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {visibleRows.length === 0 ? (
            <div className="table-empty">
              <EmptyState
                title={hasActiveFilters ? `没有匹配的${model.title}` : `暂无${model.title}`}
                message={hasActiveFilters ? '调整查询条件后再试，或重置筛选查看全部数据。' : '当前范围内还没有可展示的数据。'}
              />
            </div>
          ) : null}
        </div>
      </Panel>
    </PageFrame>
  );
}

function InsightDetail({ model, row }: { model: InsightModel; row: InsightRow }) {
  const rows = [
    [model.columns[0], row.name],
    [model.columns[1], row.type],
    [model.columns[2], row.primary],
    [model.columns[3], row.secondary],
    ['归属 / 主体', row.owner],
    [model.columns[4], row.statusText],
    [model.columns[5], row.updatedAt],
    ['证据', row.evidence],
    ['建议动作', row.suggestion],
  ];

  return (
    <section className="grid-main">
      <Panel title="基础信息">
        <div className="kv">
          {rows.flatMap(([label, value]) => [
            <div key={`${label}-label`}>{label}</div>,
            <div key={`${label}-value`}>{value}</div>,
          ])}
        </div>
      </Panel>
      <section className="right-stack">
        <Panel title="当前状态">
          <div className="detail-card">
            <h4>{row.statusText}</h4>
            <Badge tone={statusTone(row.status)}>{row.type}</Badge>
            <div className="mini-card-meta" style={{ marginTop: 12 }}>{row.suggestion}</div>
          </div>
        </Panel>
        <Panel title="关联链路">
          <div className="drawer-list">
            <div className="mini-card">
              <div className="mini-card-title">{row.primary}</div>
              <div className="mini-card-meta">{row.secondary}</div>
            </div>
            <div className="mini-card">
              <div className="mini-card-title">{row.owner}</div>
              <div className="mini-card-meta">{row.evidence}</div>
            </div>
          </div>
        </Panel>
      </section>
    </section>
  );
}

function statusTone(status: InsightStatus) {
  if (status === 'normal' || status === 'done') {
    return 'green';
  }

  if (status === 'warning' || status === 'processing') {
    return 'amber';
  }

  if (status === 'critical') {
    return 'red';
  }

  return 'neutral';
}
