import type { ReactNode } from 'react';
import { Drawer, Section } from '../components/ui';
import { ResourceName } from '../components/ResourceName';
import {
  labels,
  timestamp,
  title,
  type Kind,
  type AnyResource,
  type Gateway,
  type Backend,
  type AIBackend,
  type HTTPRoute,
  type ModelRoute,
  type MCPRoute,
} from '../domain/resources';

function Values({ entries }: { entries: [string, ReactNode][] }) {
  return (
    <div className="detail-grid">
      {entries.map(([label, value]) => (
        <div key={label}>
          <span>{label}</span>
          <strong>{value}</strong>
        </div>
      ))}
    </div>
  );
}

export function ResourceDetail({
  namespace,
  kind,
  resource,
  onClose,
  onEdit,
}: {
  namespace: string;
  kind: Kind;
  resource: AnyResource;
  onClose: () => void;
  onEdit: () => void;
}) {
  const ref = (type: Kind, name: string) => (
    <ResourceName namespace={namespace} kind={type} name={name} />
  );
  let content: ReactNode;
  if (kind === 'gateways') {
    const gateway = resource as Gateway;
    content = (
      <Section title="监听入口">
        <div className="detail-list">
          {gateway.listeners?.map((listener) => (
            <div className="detail-row" key={listener.name}>
              <div>
                <strong>{listener.name}</strong>
                <span className="secondary mono">{listener.hostname || '全部主机名'}</span>
              </div>
              <span className="badge">HTTP · :{listener.port}</span>
            </div>
          ))}
        </div>
      </Section>
    );
  } else if (kind === 'backends') {
    const backend = resource as Backend;
    content = (
      <>
        <Section title="连接设置">
          <Values
            entries={[
              ['连接方式', backend.tls ? 'HTTPS · 系统 CA' : 'HTTP'],
              ...(backend.tls
                ? [['服务器名称（SNI）', backend.tls.sni] as [string, ReactNode]]
                : []),
            ]}
          />
        </Section>
        <Section title="服务地址">
          <div className="detail-list">
            {backend.endpoints?.map((endpoint) => (
              <div key={endpoint.address + ':' + endpoint.port} className="detail-row">
                <strong className="mono">
                  {endpoint.address}:{endpoint.port}
                </strong>
                <span className="badge">权重 {endpoint.weight}</span>
              </div>
            ))}
          </div>
        </Section>
      </>
    );
  } else if (kind === 'ai-backends') {
    const backend = resource as AIBackend;
    content = (
      <Section title="模型接入">
        <Values
          entries={[
            ['接口协议', backend.schema?.name ?? '—'],
            ['API 路径前缀', backend.schema?.prefix || '/v1'],
            ['网络服务', ref('backends', backend.backendRef?.name ?? '')],
          ]}
        />
      </Section>
    );
  } else {
    const route = resource as HTTPRoute | ModelRoute | MCPRoute;
    const parents = (
      <Section title="入口配置">
        <div className="detail-list">
          {route.parentRefs?.map((parent) => (
            <div className="detail-row" key={parent.name + ':' + (parent.sectionName ?? '')}>
              <strong>{ref('gateways', parent.name)}</strong>
              <span className="badge">{parent.sectionName || '全部合格入口'}</span>
            </div>
          ))}
        </div>
      </Section>
    );
    if (kind === 'http-routes') {
      const http = resource as HTTPRoute;
      content = (
        <>
          {parents}
          {http.rules?.map((rule, index) => (
            <Section key={index} title={rule.name || `规则 ${index + 1}`}>
              <Values
                entries={[
                  [
                    '请求匹配',
                    rule.matches?.length
                      ? rule.matches.map((match, matchIndex) => (
                          <div className="mono" key={matchIndex}>
                            {match.path?.type === 'PATH_MATCH_TYPE_EXACT' ? '精确' : '前缀'}{' '}
                            {match.path?.value || '/'}
                          </div>
                        ))
                      : '前缀 /',
                  ],
                  [
                    '目标服务',
                    rule.backendRefs?.map((backend) => (
                      <div key={backend.name}>{ref('backends', backend.name)}</div>
                    )),
                  ],
                ]}
              />
            </Section>
          ))}
        </>
      );
    } else if (kind === 'model-routes') {
      const model = resource as ModelRoute;
      content = (
        <>
          {parents}
          {model.rules?.map((rule, index) => (
            <Section key={index} title={rule.name || `模型规则 ${index + 1}`}>
              <Values
                entries={[
                  [
                    '客户端模型名',
                    rule.matches?.length
                      ? rule.matches.map((match, matchIndex) => (
                          <div className="mono" key={matchIndex}>
                            {match.headers?.length
                              ? match.headers.map((header) => header.value).join(' & ')
                              : '所有模型'}
                          </div>
                        ))
                      : '所有模型',
                  ],
                  [
                    '模型服务',
                    rule.backendRefs?.map((backend) => (
                      <div key={backend.name}>{ref('ai-backends', backend.name)}</div>
                    )),
                  ],
                  [
                    '上游模型名',
                    rule.backendRefs?.map((backend) => (
                      <div key={backend.name}>
                        {backend.modelNameOverride || '保留客户端模型名'}
                      </div>
                    )),
                  ],
                ]}
              />
            </Section>
          ))}
        </>
      );
    } else {
      const mcp = resource as MCPRoute;
      content = (
        <>
          {parents}
          <Section title="MCP 转发">
            <Values
              entries={[
                ['入口路径', mcp.path ?? '/mcp'],
                [
                  '目标服务',
                  mcp.backendRefs?.map((backend) => (
                    <div key={backend.name}>{ref('backends', backend.name)}</div>
                  )),
                ],
                [
                  '上游 MCP 路径',
                  mcp.backendRefs?.map((backend) => (
                    <div key={backend.name}>{backend.path ?? '/mcp'}</div>
                  )),
                ],
              ]}
            />
          </Section>
        </>
      );
    }
  }

  return (
    <Drawer
      namespace={namespace}
      title={labels[kind] + '详情'}
      onClose={onClose}
      footer={
        <button className="button primary" onClick={onEdit}>
          编辑{labels[kind]}
        </button>
      }
    >
      <div className="detail-hero">
        <strong>{title(resource)}</strong>
        <span className={'badge ' + (kind === 'ai-backends' ? 'model' : '')}>{labels[kind]}</span>
      </div>
      {content}
      <Section title="资源信息">
        <Values entries={[['创建时间', timestamp(resource.createTime)]]} />
      </Section>
      <details className="technical">
        <summary>技术标识</summary>
        <dl>
          <dt>name</dt>
          <dd>{resource.name}</dd>
          <dt>version</dt>
          <dd>{resource.version}</dd>
        </dl>
      </details>
    </Drawer>
  );
}
