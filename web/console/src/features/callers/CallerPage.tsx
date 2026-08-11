import { useMemo, useState } from 'react';
import {
  Activity,
  Bot,
  Check,
  ChevronRight,
  Copy,
  KeyRound,
  LockKeyhole,
  MoreHorizontal,
  Plus,
  RefreshCw,
  Route,
  ShieldCheck,
  UsersRound,
  Zap,
} from 'lucide-react';
import { Badge, Button, Drawer, Modal, PageFrame, StatusDot, Toast } from '@/components/ui';

interface Caller {
  id: string;
  name: string;
  owner: string;
  keys: number;
  models: string[];
  requests: string;
  tokens: string;
  cost: string;
  lastSeen: string;
  status: 'active' | 'limited';
}

const callers: Caller[] = [
  { id: 'customer-support', name: '客服助手', owner: '客户体验团队', keys: 2, models: ['qwen-max', 'text-embedding'], requests: '58.4K', tokens: '9.6M', cost: '¥ 628', lastSeen: '刚刚', status: 'active' },
  { id: 'knowledge-base', name: '研发知识库', owner: '研发效能团队', keys: 3, models: ['claude-sonnet', 'text-embedding'], requests: '31.7K', tokens: '6.2M', cost: '¥ 486', lastSeen: '2 分钟前', status: 'active' },
  { id: 'automation-jobs', name: '内部自动化', owner: '平台工程团队', keys: 1, models: ['qwen-max'], requests: '12.6K', tokens: '2.1M', cost: '¥ 142', lastSeen: '18 分钟前', status: 'limited' },
  { id: 'code-review', name: '代码审查助手', owner: '研发效能团队', keys: 1, models: ['claude-sonnet'], requests: '8.2K', tokens: '800K', cost: '¥ 28', lastSeen: '6 分钟前', status: 'active' },
];

const callerTabs = ['概览', '访问密钥', '权限', '额度', '用量'];

export function CallerPage() {
  const [selectedID, setSelectedID] = useState(callers[0].id);
  const [activeTab, setActiveTab] = useState(callerTabs[0]);
  const [createOpen, setCreateOpen] = useState(false);
  const [keyOpen, setKeyOpen] = useState(false);
  const [secretOpen, setSecretOpen] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const selected = useMemo(() => callers.find((caller) => caller.id === selectedID) ?? callers[0], [selectedID]);

  return (
    <PageFrame
      title="调用方"
      subtitle="管理调用网关的应用或服务，以及访问密钥、权限和额度"
      actions={<Button onClick={() => setCreateOpen(true)}><Plus className="h-4 w-4" />创建调用方</Button>}
    >
      <section className="caller-overview-strip">
        <div><span><UsersRound /></span><p>调用方<strong>4</strong><small>3 个运行正常</small></p></div>
        <div><span><KeyRound /></span><p>有效密钥<strong>14</strong><small>2 个将在 30 天内过期</small></p></div>
        <div><span><Activity /></span><p>今日请求<strong>102.7K</strong><small>拒绝率 0.12%</small></p></div>
        <div><span><ShieldCheck /></span><p>策略拒绝<strong>124</strong><small>主要原因：额度不足</small></p></div>
      </section>

      <div className="caller-workbench">
        <aside className="caller-list surface-card">
          <div className="catalog-heading"><div><span className="eyebrow">访问主体</span><h3>全部调用方</h3></div><span>{callers.length}</span></div>
          {callers.map((caller) => (
            <button
              type="button"
              key={caller.id}
              className={`caller-list-item ${caller.id === selected.id ? 'is-selected' : ''}`}
              onClick={() => { setSelectedID(caller.id); setActiveTab(callerTabs[0]); }}
            >
              <span className="caller-avatar">{caller.name.slice(0, 1)}</span>
              <div><strong>{caller.name}</strong><span>{caller.owner}</span><small><StatusDot status={caller.status === 'active' ? 'healthy' : 'warning'} /> {caller.lastSeen}活跃</small></div>
              <ChevronRight />
            </button>
          ))}
        </aside>

        <section className="caller-detail surface-card">
          <header className="caller-detail-header">
            <div className="caller-heading-main"><span className="caller-large-avatar">{selected.name.slice(0, 1)}</span><div><div><h2>{selected.name}</h2><Badge tone={selected.status === 'active' ? 'success' : 'warning'}>{selected.status === 'active' ? '运行正常' : '额度受限'}</Badge></div><p>{selected.owner} · <span className="mono">{selected.id}</span></p></div></div>
            <div><Button variant="outline" onClick={() => setKeyOpen(true)}><KeyRound />签发密钥</Button><Button variant="ghost"><MoreHorizontal /></Button></div>
          </header>
          <nav className="detail-tabs">{callerTabs.map((tab) => <button type="button" key={tab} className={activeTab === tab ? 'is-active' : ''} onClick={() => setActiveTab(tab)}>{tab}</button>)}</nav>
          {activeTab === '概览' ? <CallerOverview caller={selected} onCreateKey={() => setKeyOpen(true)} /> : null}
          {activeTab === '访问密钥' ? <CallerKeys onCreateKey={() => setKeyOpen(true)} /> : null}
          {activeTab === '权限' ? <CallerPermissions /> : null}
          {activeTab === '额度' ? <CallerQuota /> : null}
          {activeTab === '用量' ? <CallerUsage caller={selected} /> : null}
        </section>
      </div>

      <Drawer title="创建调用方" subtitle="为一个实际使用网关的应用或服务建立独立身份" isOpen={createOpen} onClose={() => setCreateOpen(false)}>
        <div className="drawer-form">
          <label className="form-field"><span>调用方名称</span><input className="input" placeholder="例如 客服助手" /></label>
          <label className="form-field"><span>唯一标识</span><input className="input mono" placeholder="customer-support" /></label>
          <label className="form-field"><span>负责人</span><input className="input" placeholder="团队或负责人" /></label>
          <label className="form-field"><span>用途说明</span><textarea className="textarea" placeholder="这个调用方将通过网关完成什么业务" /></label>
          <div className="drawer-actions"><Button variant="ghost" onClick={() => setCreateOpen(false)}>取消</Button><Button onClick={() => { setCreateOpen(false); setNotice('调用方已添加到当前原型'); }}>创建调用方</Button></div>
        </div>
      </Drawer>

      <Drawer title="签发访问密钥" subtitle={`密钥将归属于“${selected.name}”，生成后只展示一次`} isOpen={keyOpen} onClose={() => setKeyOpen(false)}>
        <div className="drawer-form">
          <label className="form-field"><span>密钥名称</span><input className="input" defaultValue="生产环境" /></label>
          <label className="form-field"><span>有效期</span><select className="select"><option>90 天</option><option>180 天</option><option>1 年</option><option>永不过期</option></select></label>
          <div className="key-security-note"><ShieldCheck /><div><strong>继承调用方权限</strong><p>新密钥自动继承当前调用方的 API 路由、AI 模型和流量策略，不单独维护权限副本。</p></div></div>
          <div className="key-security-note"><LockKeyhole /><div><strong>密钥不会再次完整展示</strong><p>请在创建后立即复制，并保存到调用方使用的密钥管理系统。</p></div></div>
          <div className="drawer-actions"><Button variant="ghost" onClick={() => setKeyOpen(false)}>取消</Button><Button onClick={() => { setKeyOpen(false); setSecretOpen(true); }}>生成密钥</Button></div>
        </div>
      </Drawer>

      <Modal title="访问密钥已生成" isOpen={secretOpen} onClose={() => setSecretOpen(false)}>
        <div className="secret-result">
          <span className="secret-success"><Check /></span>
          <h3>请立即保存这段密钥</h3>
          <p>关闭窗口后无法再次查看完整内容。</p>
          <div className="secret-value"><code>ig_sk_live_2p9QF7mL3x8VnK4s</code><button type="button" onClick={() => setNotice('密钥已复制')}><Copy /></button></div>
          <Button onClick={() => setSecretOpen(false)}>我已安全保存</Button>
        </div>
      </Modal>
      <Toast message={notice} onClose={() => setNotice(null)} />
    </PageFrame>
  );
}

function CallerOverview({ caller, onCreateKey }: { caller: Caller; onCreateKey: () => void }) {
  return (
    <div className="caller-tab-body">
      <div className="model-kpis"><MiniMetric label="今日请求" value={caller.requests} note="成功率 99.88%" /><MiniMetric label="普通 API" value="38.2K" note="订单和客户资料接口" /><MiniMetric label="AI 请求" value="20.2K" note={`${caller.tokens} Token`} /><MiniMetric label="有效密钥" value={String(caller.keys)} note="最近使用于刚刚" /></div>
      <div className="caller-overview-grid">
        <section className="detail-section">
          <div className="detail-section-heading"><h3>访问权限</h3><button type="button">管理权限 <ChevronRight /></button></div>
          <div className="permission-summary"><span><Bot /></span><div><strong>{caller.models.length} 个模型</strong><p>{caller.models.join('、')}</p></div></div>
          <div className="permission-summary"><span><Route /></span><div><strong>2 条普通路由</strong><p>订单查询 API、客户资料 API</p></div></div>
        </section>
        <section className="detail-section">
          <div className="detail-section-heading"><h3>额度使用</h3><Badge tone="success">正常</Badge></div>
          <QuotaBar label="月度 Token" value="12.6M / 20M" percent={63} />
          <QuotaBar label="每分钟请求" value="382 / 1,000" percent={38} />
          <QuotaBar label="并发请求" value="18 / 50" percent={36} />
        </section>
      </div>
      <div className="quick-key-card"><span><KeyRound /></span><div><strong>为这个调用方签发访问密钥</strong><p>不同部署使用独立密钥，并共享当前调用方的权限和额度。</p></div><Button variant="outline" onClick={onCreateKey}>签发密钥</Button></div>
    </div>
  );
}

function CallerKeys({ onCreateKey }: { onCreateKey: () => void }) {
  return (
    <div className="caller-tab-body">
      <div className="tab-intro"><div><h3>访问密钥</h3><p>不同部署或任务使用独立密钥，便于轮换和审计。</p></div><Button onClick={onCreateKey}><Plus />签发密钥</Button></div>
      <div className="key-list">
        <KeyRow name="生产环境" prefix="ig_sk_live_2p9Q••••K4s" lastUsed="刚刚" expires="2026-11-09" />
        <KeyRow name="数据回填任务" prefix="ig_sk_job_7mL2••••P8d" lastUsed="2 天前" expires="2026-09-01" />
      </div>
    </div>
  );
}

function CallerPermissions() {
  return (
    <div className="caller-tab-body permission-grid">
      <PermissionCard icon={Route} title="普通 API 路由" items={['订单查询 API', '客户资料 API']} action="管理路由" />
      <PermissionCard icon={Bot} title="AI 模型权限" items={['qwen-max', 'text-embedding']} action="管理模型" />
      <PermissionCard icon={ShieldCheck} title="生效流量策略" items={['客服请求限流', '办公网 IP 访问限制']} action="查看策略" />
    </div>
  );
}

function CallerQuota() {
  return (
    <div className="caller-tab-body">
      <div className="tab-intro"><div><h3>用量额度</h3><p>达到硬限制时请求会被拒绝，并在请求日志中记录明确原因。</p></div><Button variant="outline">调整额度</Button></div>
      <div className="quota-config-grid">
        <QuotaConfig title="请求速率" value="1,000 RPM" note="短时突发上限 1,200" />
        <QuotaConfig title="Token 速率" value="200K TPM" note="输入与输出合并计算" />
        <QuotaConfig title="并发请求" value="50" note="超过后立即返回限流响应" />
        <QuotaConfig title="月度预算" value="¥ 1,000" note="每月 1 日自动重置" />
      </div>
    </div>
  );
}

function CallerUsage({ caller }: { caller: Caller }) {
  return <div className="caller-tab-body"><div className="model-kpis"><MiniMetric label="全部请求" value={caller.requests} note="过去 24 小时" /><MiniMetric label="普通 API" value="38.2K" note="响应流量 4.8 GB" /><MiniMetric label="AI 请求" value="20.2K" note={`${caller.tokens} Token · ${caller.cost}`} /><MiniMetric label="拒绝请求" value="124" note="主要原因：额度不足" /></div><div className="usage-chart-card"><div className="detail-section-heading"><h3>按流量类型统计</h3><span>过去 7 天</span></div><div className="usage-model-row"><span>普通 API</span><i><b style={{ width: '65%' }} /></i><strong>65%</strong></div><div className="usage-model-row"><span>AI 请求</span><i><b style={{ width: '35%' }} /></i><strong>35%</strong></div></div></div>;
}

function KeyRow({ name, prefix, lastUsed, expires }: { name: string; prefix: string; lastUsed: string; expires: string }) {
  return <div className="key-row"><span><KeyRound /></span><div><strong>{name}</strong><code>{prefix}</code></div><div><span>最近使用</span><strong>{lastUsed}</strong></div><div><span>到期时间</span><strong>{expires}</strong></div><Badge tone="success">有效</Badge><Button variant="ghost"><MoreHorizontal /></Button></div>;
}

function PermissionCard({ icon: Icon, title, items, action }: { icon: typeof Bot; title: string; items: string[]; action: string }) {
  return <section className="permission-card"><header><span><Icon /></span><div><h3>{title}</h3><p>{items.length} 项授权</p></div><Button variant="ghost" size="sm">{action}</Button></header><div>{items.map((item) => <span key={item}><Check />{item}</span>)}</div></section>;
}

function QuotaBar({ label, value, percent }: { label: string; value: string; percent: number }) {
  return <div className="quota-bar"><div><span>{label}</span><strong>{value}</strong></div><i><b style={{ width: `${percent}%` }} /></i></div>;
}

function QuotaConfig({ title, value, note }: { title: string; value: string; note: string }) {
  return <article><span><Zap /></span><div><small>{title}</small><strong>{value}</strong><p>{note}</p></div><button type="button"><RefreshCw /></button></article>;
}

function MiniMetric({ label, value, note }: { label: string; value: string; note: string }) {
  return <article><span>{label}</span><strong>{value}</strong><small>{note}</small></article>;
}
