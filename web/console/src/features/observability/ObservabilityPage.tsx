import { useMemo, useState } from 'react';
import { ArrowDownUp, Bot, Check, ChevronRight, KeyRound, Route, Search, Server, ShieldAlert, ShieldCheck } from 'lucide-react';
import { useSearchParams } from 'react-router-dom';
import { Badge, Drawer, PageFrame } from '@/components/ui';
import { prototypeScenario, type RequestRecord, type RequestState } from '@/prototype/scenario';

type RequestFilter = 'all' | 'api' | 'ai' | 'problem';

export function ObservabilityPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const initialQuery = searchParams.get('query') ?? '';
  const initialRequest = prototypeScenario.requests.find((request) => request.id === initialQuery) ?? null;
  const [query, setQuery] = useState(initialQuery);
  const [filter, setFilter] = useState<RequestFilter>('all');
  const [selected, setSelected] = useState<RequestRecord | null>(initialRequest);
  const requests = useMemo(() => prototypeScenario.requests.filter((request) => {
    const matchesFilter = filter === 'all'
      || request.kind === filter
      || (filter === 'problem' && request.status !== 'success');
    const searchText = `${request.id}${request.caller}${request.route}${request.method}${request.path}${request.model}${request.service}`.toLowerCase();
    return matchesFilter && searchText.includes(query.trim().toLowerCase());
  }), [filter, query]);

  const selectRequest = (request: RequestRecord) => {
    setSelected(request);
    setSearchParams({ query: request.id });
  };

  return (
    <PageFrame title="请求" subtitle="在同一条时间线上排查普通 API 和 AI 请求的认证、策略、路由与服务响应">
      <section className="request-summary-line">
        <div><strong>142,700</strong><span>今日请求</span></div>
        <div><strong>98.3K</strong><span>普通 API</span></div>
        <div><strong>44.4K</strong><span>AI 请求</span></div>
        <div className="is-error"><strong>257</strong><span>失败或拒绝</span></div>
        <p>请求正文默认不记录；AI 请求额外保存模型线路、Token 和估算成本。</p>
      </section>

      <section className="surface-card request-log-card">
        <header className="request-toolbar">
          <label className="log-search"><Search /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索请求 ID、路由、调用方或服务" /></label>
          <div className="request-filters">
            <FilterButton active={filter === 'all'} onClick={() => setFilter('all')}>全部</FilterButton>
            <FilterButton active={filter === 'api'} onClick={() => setFilter('api')}>普通 API</FilterButton>
            <FilterButton active={filter === 'ai'} onClick={() => setFilter('ai')}>AI 请求</FilterButton>
            <FilterButton active={filter === 'problem'} onClick={() => setFilter('problem')}>异常</FilterButton>
          </div>
          <span>{requests.length} 条结果</span>
        </header>
        <div className="log-table">
          <div className="log-table-head"><span>时间 / 请求 ID</span><span>调用方</span><span>路由 / 请求</span><span>结果</span><span>用量</span><span>服务 / 耗时</span><span /></div>
          {requests.map((request) => (
            <button type="button" key={request.id} className="log-row" onClick={() => selectRequest(request)}>
              <div><strong>{request.time}</strong><code>{request.id}</code></div>
              <div><strong>{request.caller}</strong><span>生产密钥</span></div>
              <div><strong>{request.route}</strong><span>{request.kind === 'ai' ? request.model : `${request.method} ${request.path}`}</span></div>
              <div><Badge tone={statusTone(request.status)}>{request.code} · {statusLabel(request.status)}</Badge></div>
              <div><strong>{request.kind === 'ai' ? request.tokens : request.bytes}</strong><span>{request.kind === 'ai' ? 'Tokens' : '响应流量'}</span></div>
              <div><strong>{request.service}</strong><span>{request.latency}{request.kind === 'ai' && request.cost !== '—' ? ` · ${request.cost}` : ''}</span></div>
              <ChevronRight />
            </button>
          ))}
          {requests.length === 0 ? <div className="request-empty">没有匹配的请求记录</div> : null}
        </div>
      </section>
      <RequestDrawer request={selected} onClose={() => setSelected(null)} />
    </PageFrame>
  );
}

function FilterButton({ active, onClick, children }: { active: boolean; onClick: () => void; children: string }) {
  return <button type="button" className={active ? 'is-active' : ''} onClick={onClick}>{children}</button>;
}

function RequestDrawer({ request, onClose }: { request: RequestRecord | null; onClose: () => void }) {
  return (
    <Drawer title={request ? `请求 ${request.id}` : '请求详情'} subtitle={request ? `${request.time} · ${request.caller} · ${request.kind === 'api' ? '普通 API' : 'AI 请求'}` : undefined} isOpen={Boolean(request)} onClose={onClose}>
      {request ? (
        <div className="request-detail">
          <div className="request-detail-status"><span className={request.status === 'blocked' || request.status === 'error' ? 'is-error' : 'is-success'}>{request.status === 'blocked' || request.status === 'error' ? <ShieldAlert /> : <Check />}</span><div><strong>{request.code} · {statusLabel(request.status)}</strong><p>{resultDescription(request)}</p></div></div>
          <div className="request-facts"><Fact label="调用方" value={request.caller} /><Fact label="路由" value={request.route} /><Fact label="请求" value={`${request.method} ${request.path}`} /><Fact label={request.kind === 'ai' ? '模型' : '目标服务'} value={request.kind === 'ai' ? request.model : request.service} /></div>
          <section>
            <h3>执行过程</h3>
            <div className="execution-timeline request-timeline">
              <Timeline icon={KeyRound} title="调用方认证" detail={`${request.caller} · 生产密钥`} value="2 ms" />
              <Timeline icon={ShieldCheck} title="访问策略" detail={request.status === 'blocked' ? '用量额度不足，请求被策略拒绝' : '权限、限流和访问限制检查通过'} value="4 ms" tone={request.status === 'blocked' ? 'error' : 'success'} />
              {request.status !== 'blocked' ? <>
                <Timeline icon={Route} title={request.route} detail={request.kind === 'ai' ? `${request.model} · 已选择模型服务线路` : `${request.method} ${request.path} · 已匹配目标服务`} value="1 ms" />
                {request.status === 'fallback' ? <Timeline icon={Bot} title="Anthropic 公网" detail="主服务连接超时，准备切换" value="2.0 s" tone="warning" /> : null}
                <Timeline icon={request.status === 'fallback' ? ArrowDownUp : Server} title={request.service} detail={serviceResult(request)} value={request.status === 'fallback' ? '1.4 s' : request.latency} tone={request.status === 'error' ? 'error' : 'success'} />
              </> : null}
            </div>
          </section>
          {request.kind === 'ai' ? (
            <section><h3>AI 用量</h3><div className="token-breakdown"><Fact label="输入 Token" value={request.inputTokens} /><Fact label="输出 Token" value={request.outputTokens} /><Fact label="缓存命中 Token" value={request.cachedTokens} /><Fact label="估算成本" value={request.cost} /></div></section>
          ) : (
            <section><h3>HTTP 结果</h3><div className="token-breakdown"><Fact label="状态码" value={request.code} /><Fact label="响应流量" value={request.bytes} /><Fact label="总耗时" value={request.latency} /><Fact label="目标服务" value={request.service} /></div></section>
          )}
          <div className="payload-note"><ShieldCheck /><div><strong>请求正文未记录</strong><p>当前数据策略只保存执行元数据和结果；AI 请求额外保存 Token 与成本。</p></div></div>
        </div>
      ) : null}
    </Drawer>
  );
}

function resultDescription(request: RequestRecord) {
  if (request.status === 'fallback') return '主模型服务失败后由备用服务成功响应';
  if (request.status === 'blocked') return '请求在访问目标服务前被流量策略拒绝';
  if (request.status === 'error') return '路由匹配成功，但目标服务未能正常响应';
  return `请求由${request.kind === 'ai' ? '模型服务' : '目标服务'}正常完成`;
}

function serviceResult(request: RequestRecord) {
  if (request.status === 'error') return '上游连接被重置，返回 502';
  if (request.status === 'fallback') return `${request.model} · 备用模型服务`;
  return request.kind === 'ai' ? `${request.model} · 主模型服务` : `${request.code} · 响应 ${request.bytes}`;
}

function Fact({ label, value }: { label: string; value: string }) { return <div><span>{label}</span><strong>{value}</strong></div>; }
function Timeline({ icon: Icon, title, detail, value, tone = 'success' }: { icon: typeof KeyRound; title: string; detail: string; value: string; tone?: 'success' | 'warning' | 'error' }) { return <div className={`timeline-item is-${tone}`}><span><Icon /></span><i /><div><strong>{title}</strong><p>{detail}</p></div><small>{value}</small></div>; }
function statusLabel(status: RequestState) { if (status === 'success') return '成功'; if (status === 'fallback') return '主备切换'; if (status === 'blocked') return '策略拒绝'; return '失败'; }
function statusTone(status: RequestState): 'success' | 'warning' | 'error' { if (status === 'success') return 'success'; if (status === 'fallback') return 'warning'; return 'error'; }
