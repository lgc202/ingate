import { useState } from 'react';
import { Bot, Check, CircleDollarSign, Clock3, Code2, Copy, KeyRound, RotateCcw, Send, Server, ShieldCheck, Sparkles, UserRound, Zap } from 'lucide-react';
import { useSearchParams } from 'react-router-dom';
import { Badge, Button, PageFrame, StatusDot, Toast } from '@/components/ui';

function exampleAnswer(model: string) {
  const service = model === 'claude-sonnet' ? 'Anthropic 公网' : '通义千问生产';
  return `我是通过 Ingate 访问的企业 AI 助手。当前请求使用模型 ${model}，并由${service}完成响应。`;
}

export function PlaygroundPage() {
  const [searchParams] = useSearchParams();
  const kind = searchParams.get('type') === 'ai' ? 'ai' : 'api';
  const route = searchParams.get('route') ?? (kind === 'ai' ? '生产 AI 路由' : '订单查询 API');
  const model = searchParams.get('model') ?? 'qwen-max';
  const [prompt, setPrompt] = useState('你是谁？简单说明当前请求经过了哪些环节。');
  const [answer, setAnswer] = useState(() => exampleAnswer(model));
  const [sending, setSending] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);

  const send = () => {
    setSending(true);
    if (kind === 'ai') setAnswer('');
    window.setTimeout(() => {
      if (kind === 'ai') setAnswer(exampleAnswer(model));
      setSending(false);
    }, 650);
  };

  return (
    <PageFrame
      title="调试请求"
      subtitle={`从“${route}”发起一次${kind === 'ai' ? ' AI' : '普通 API'} 请求，验证认证、策略、路由和服务响应`}
      actions={<Button variant="outline" onClick={() => setNotice('已复制 curl 示例')}><Code2 className="h-4 w-4" />复制 curl</Button>}
    >
      <div className="playground-layout">
        <section className={`playground-console surface-card ${kind === 'api' ? 'api-debug-console' : ''}`}>
          <header className="playground-toolbar">
            <div className="playground-select"><span>{kind === 'ai' ? <Bot /> : <Server />}</span><div><small>{kind === 'ai' ? 'AI 路由' : 'API 路由'}</small><strong>{route}</strong></div></div>
            <div className="playground-select"><span><KeyRound /></span><div><small>调用方</small><strong>{kind === 'ai' ? '客服助手' : '电商 Web'}</strong></div></div>
            <Badge tone="success"><StatusDot status="healthy" /> 路由配置已生效</Badge>
          </header>

          {kind === 'ai' ? (
            <>
              <div className="conversation">
                <div className="conversation-message is-user"><span><UserRound /></span><div><small>用户</small><p>{prompt}</p></div></div>
                <div className="conversation-message is-assistant"><span><Sparkles /></span><div><small>{model}</small>{sending ? <div className="typing"><i /><i /><i /></div> : <p>{answer}</p>}</div></div>
              </div>
              <div className="prompt-editor"><textarea value={prompt} onChange={(event) => setPrompt(event.target.value)} aria-label="调试消息" /><div><span>模型：{model}</span><Button variant="ghost" onClick={() => setPrompt('')}><RotateCcw /></Button><Button disabled={sending || !prompt.trim()} onClick={send}><Send />发送请求</Button></div></div>
            </>
          ) : (
            <APIRequestBuilder sending={sending} onSend={send} />
          )}
        </section>

        <aside className="playground-inspector">
          <section className="surface-card inspector-card">
            <div className="surface-heading"><div><span className="eyebrow">本次结果</span><h3>请求摘要</h3></div><Badge tone="success">200 成功</Badge></div>
            <div className="result-metrics">
              {kind === 'ai' ? <><ResultMetric icon={Clock3} label="首 Token" value="612 ms" /><ResultMetric icon={Zap} label="总 Token" value="86" /><ResultMetric icon={CircleDollarSign} label="估算成本" value="¥ 0.006" /></> : <><ResultMetric icon={Clock3} label="总耗时" value="86 ms" /><ResultMetric icon={Zap} label="响应流量" value="12.4 KB" /><ResultMetric icon={Server} label="目标服务" value="订单服务" /></>}
            </div>
          </section>

          <section className="surface-card inspector-card">
            <div className="surface-heading"><div><span className="eyebrow">实际路径</span><h3>执行过程</h3></div></div>
            <div className="execution-timeline">
              <TimelineItem icon={KeyRound} title="调用方认证" detail={`${kind === 'ai' ? '客服助手' : '电商 Web'} · 生产密钥`} value="2 ms" />
              <TimelineItem icon={ShieldCheck} title="访问策略" detail={kind === 'ai' ? '权限、请求限流和 Token 额度通过' : '权限、请求限流和 IP 限制通过'} value="4 ms" />
              <TimelineItem icon={kind === 'ai' ? Bot : Server} title={route} detail={kind === 'ai' ? `${model} → 通义千问生产 / qwen-max` : 'GET /api/orders/78421 → 订单服务'} value="1 ms" />
              <TimelineItem icon={Server} title={kind === 'ai' ? '通义千问生产' : '订单服务'} detail={kind === 'ai' ? 'qwen-max · 主模型服务' : 'HTTP 200 · 12.4 KB'} value={kind === 'ai' ? '1.42 s' : '79 ms'} last />
            </div>
          </section>

          <section className="surface-card inspector-card request-code-card">
            <div className="surface-heading"><div><span className="eyebrow">接入信息</span><h3>{kind === 'ai' ? 'OpenAI 兼容入口' : 'API 请求地址'}</h3></div><button type="button" onClick={() => setNotice('请求地址已复制')}><Copy /></button></div>
            <code>{kind === 'ai' ? 'https://api.example.com/v1' : 'https://api.example.com/api/orders/78421'}</code>
            <p>{kind === 'ai' ? 'OpenAI SDK 只需替换 Base URL、API Key 和模型名。' : '客户端通过调用方密钥访问该路由，不直接连接内部服务。'}</p>
          </section>
        </aside>
      </div>
      <Toast message={notice} onClose={() => setNotice(null)} />
    </PageFrame>
  );
}

function APIRequestBuilder({ sending, onSend }: { sending: boolean; onSend: () => void }) {
  return <div className="api-request-builder"><section><span className="eyebrow">请求</span><div className="api-request-line"><select className="select" defaultValue="GET"><option>GET</option><option>POST</option><option>PUT</option><option>DELETE</option></select><input className="input mono" defaultValue="https://api.example.com/api/orders/78421" /><Button disabled={sending} onClick={onSend}><Send />{sending ? '发送中' : '发送'}</Button></div></section><section><div className="api-editor-heading"><strong>请求头</strong><span>2 项</span></div><div className="api-header-row"><code>Authorization</code><span>Bearer ig_sk_live_••••••••</span></div><div className="api-header-row"><code>Accept</code><span>application/json</span></div></section><section className="api-response"><div className="api-editor-heading"><strong>响应</strong><Badge tone="success">200 OK</Badge></div><pre>{`{
  "id": "78421",
  "status": "paid",
  "amount": 268.00,
  "currency": "CNY"
}`}</pre></section></div>;
}

function ResultMetric({ icon: Icon, label, value }: { icon: typeof Clock3; label: string; value: string }) {
  return <div><Icon /><span>{label}</span><strong>{value}</strong></div>;
}

function TimelineItem({ icon: Icon, title, detail, value, last = false }: { icon: typeof Bot; title: string; detail: string; value: string; last?: boolean }) {
  return <div className={`timeline-item ${last ? 'is-last' : ''}`}><span><Icon /></span><i /><div><strong>{title}</strong><p>{detail}</p></div><small><Check />{value}</small></div>;
}
