import { Plus, Trash2 } from 'lucide-react';
import { Field, Section } from '../../components/ui';
import { Tooltip } from '../../components/Tooltip';
import { ResourcePicker } from '../../components/ResourcePicker';
import type { AIBackend, Backend, Endpoint, Spec } from '../../domain/resources';

export function BackendFields({
  value,
  onChange,
}: {
  value: Spec<Backend>;
  onChange: (value: Spec<Backend>) => void;
}) {
  const endpoints = value.endpoints ?? [];
  const update = (index: number, changes: Partial<Endpoint>) =>
    onChange({
      ...value,
      endpoints: endpoints.map((item, current) =>
        current === index ? { ...item, ...changes } : item,
      ),
    });

  return (
    <Section title="连接设置">
      <div className="grid-two">
        <Field label="连接方式">
          <select
            aria-label="连接方式"
            value={value.tls ? 'tls' : 'http'}
            onChange={(event) =>
              onChange({
                ...value,
                tls:
                  event.target.value === 'tls'
                    ? { sni: '', wellKnownCaCertificates: 'System' }
                    : null,
              })
            }
          >
            <option value="http">HTTP</option>
            <option value="tls">HTTPS · 系统 CA</option>
          </select>
        </Field>
        {value.tls && (
          <Field
            label="服务器名称（SNI）"
            help="上游 TLS 使用的 DNS 身份，必须与证书匹配，不一定等于服务端点的 IP。固定使用系统 CA，不跳过证书校验。"
          >
            <input
              aria-label="服务器名称（SNI）"
              required
              value={value.tls.sni}
              placeholder="models.example.com"
              onChange={(event) =>
                onChange({
                  ...value,
                  tls: {
                    sni: event.target.value,
                    wellKnownCaCertificates: 'System',
                  },
                })
              }
            />
          </Field>
        )}
      </div>
      <div className="section-title">
        <h3>
          服务地址{' '}
          <Tooltip label="服务地址">
            应用路由和 MCP 路由都引用网络服务。这里只配置连接地址，MCP
            入口路径和上游路径在路由中设置。
          </Tooltip>
        </h3>
        <button
          type="button"
          className="button"
          onClick={() =>
            onChange({
              ...value,
              endpoints: [...endpoints, { address: '', port: 8080, weight: 1 }],
            })
          }
        >
          <Plus />
          添加地址
        </button>
      </div>
      <div className="endpoint-labels">
        <span>主机名 / IP</span>
        <span>端口</span>
        <span>
          权重{' '}
          <Tooltip label="端点权重">相对流量权重。例如 3 与 2 表示 3:2，不要求总和为 100。</Tooltip>
        </span>
        <span />
      </div>
      {endpoints.map((endpoint, index) => (
        <div className="endpoint" key={index}>
          <input
            aria-label={`服务地址 ${index + 1}`}
            required
            placeholder="10.0.12.8"
            value={endpoint.address}
            onChange={(event) => update(index, { address: event.target.value })}
          />
          <input
            aria-label={`服务端口 ${index + 1}`}
            required
            type="number"
            min={1}
            max={65535}
            value={endpoint.port || ''}
            onChange={(event) => update(index, { port: Number(event.target.value) })}
          />
          <input
            aria-label={`端点权重 ${index + 1}`}
            required
            type="number"
            min={1}
            value={endpoint.weight || ''}
            onChange={(event) => update(index, { weight: Number(event.target.value) })}
          />
          <button
            className="icon-button"
            type="button"
            aria-label={`删除地址 ${index + 1}`}
            disabled={endpoints.length <= 1}
            onClick={() =>
              onChange({
                ...value,
                endpoints: endpoints.filter((_, current) => current !== index),
              })
            }
          >
            <Trash2 />
          </button>
        </div>
      ))}
    </Section>
  );
}

export function AIBackendFields({
  namespace,
  value,
  onChange,
}: {
  namespace: string;
  value: Spec<AIBackend>;
  onChange: (value: Spec<AIBackend>) => void;
}) {
  return (
    <>
      <Section title="模型接入">
        <div className="grid-two">
          <Field label="接口协议">
            <select aria-label="接口协议" value="OpenAI" onChange={() => {}}>
              <option>OpenAI</option>
            </select>
          </Field>
          <Field
            label="API 路径前缀"
            help="上游模型 API 的基础路径；留空由 API Server 默认 /v1。模型名称映射在模型路由中配置。"
          >
            <input
              aria-label="API 路径前缀"
              value={value.schema?.prefix ?? ''}
              placeholder="/v1"
              onChange={(event) =>
                onChange({
                  ...value,
                  schema: { name: 'OpenAI', prefix: event.target.value },
                })
              }
            />
          </Field>
        </div>
      </Section>
      <Section title="网络连接">
        <Field
          label="网络服务"
          help="当前接口通过已有网络服务引用地址、端口和 TLS。此处不串联创建两个资源，修改也不会隐式改变被其他模型共享的网络服务。"
        >
          <ResourcePicker
            namespace={namespace}
            kind="backends"
            label="网络服务"
            value={value.backendRef?.name ?? ''}
            onChange={(name) => onChange({ ...value, backendRef: { name } })}
          />
        </Field>
      </Section>
    </>
  );
}
