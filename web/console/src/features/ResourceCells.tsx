import { Layers3, Server, BrainCircuit, GitBranch } from 'lucide-react';
import { ResourceName } from '../components/ResourceName';
import {
  timestamp,
  title,
  type AnyResource,
  type Kind,
  type Gateway,
  type Backend,
  type AIBackend,
  type HTTPRoute,
  type ModelRoute,
  type MCPRoute,
} from '../domain/resources';

export function ResourceCells({
  kind,
  resource,
  onDetail,
}: {
  kind: Kind;
  resource: AnyResource;
  onDetail: () => void;
}) {
  const Icon =
    kind === 'gateways'
      ? Layers3
      : kind === 'ai-backends'
        ? BrainCircuit
        : kind === 'backends'
          ? Server
          : GitBranch;
  const name = (
    <td>
      <button className="name-button" onClick={onDetail}>
        <Icon />
        <span>{title(resource)}</span>
      </button>
    </td>
  );
  const created = <td className="timestamp">{timestamp(resource.createTime)}</td>;
  if (kind === 'gateways') {
    const gateway = resource as Gateway;
    return (
      <>
        {name}
        <td>
          {gateway.listeners?.map((listener) => (
            <div key={listener.name}>
              <span className="badge">
                {listener.name} · HTTP :{listener.port}
              </span>
            </div>
          ))}
        </td>
        <td className="mono">
          {gateway.listeners?.map((listener) => (
            <div key={listener.name}>{listener.hostname || '全部主机名'}</div>
          ))}
        </td>
        {created}
      </>
    );
  }
  if (kind === 'backends') {
    const backend = resource as Backend;
    return (
      <>
        {name}
        <td>
          <span className="badge">应用 / MCP</span>
        </td>
        <td className="mono">
          {backend.endpoints?.map((endpoint) => (
            <div key={endpoint.address + ':' + endpoint.port}>
              {endpoint.address}:{endpoint.port}
            </div>
          ))}
        </td>
        <td>{backend.tls ? 'HTTPS' : 'HTTP'}</td>
        {created}
      </>
    );
  }
  if (kind === 'ai-backends') {
    const backend = resource as AIBackend;
    return (
      <>
        {name}
        <td>
          <span className="badge model">模型服务</span>
        </td>
        <td>
          <ResourceName
            namespace={resource.namespace}
            kind="backends"
            name={backend.backendRef?.name ?? ''}
          />
        </td>
        <td>
          {backend.schema?.name} · {backend.schema?.prefix || '/v1'}
        </td>
        {created}
      </>
    );
  }
  const route = resource as HTTPRoute | ModelRoute | MCPRoute;
  const parents = (
    <td className="mono">
      {route.parentRefs?.map((parent) => (
        <div key={parent.name + ':' + (parent.sectionName ?? '')}>
          <ResourceName namespace={resource.namespace} kind="gateways" name={parent.name} />
          {parent.sectionName ? ' · ' + parent.sectionName : ' · 全部入口'}
        </div>
      ))}
    </td>
  );
  if (kind === 'http-routes') {
    const http = resource as HTTPRoute;
    return (
      <>
        {name}
        {parents}
        <td>
          {http.rules?.map((rule, index) => (
            <div className="mono" key={index}>
              {rule.matches?.length
                ? rule.matches
                    .map(
                      (match) =>
                        `${match.path?.type === 'PATH_MATCH_TYPE_EXACT' ? '精确' : '前缀'} ${match.path?.value || '/'}`,
                    )
                    .join(' / ')
                : '前缀 /'}
            </div>
          ))}
        </td>
        <td className="mono">
          {http.rules?.map((rule, index) => (
            <div key={index}>
              {rule.backendRefs?.map((ref) => (
                <ResourceName
                  key={ref.name}
                  namespace={resource.namespace}
                  kind="backends"
                  name={ref.name}
                />
              ))}
            </div>
          ))}
        </td>
        {created}
      </>
    );
  }
  if (kind === 'model-routes') {
    const model = resource as ModelRoute;
    return (
      <>
        {name}
        {parents}
        <td>
          {model.rules?.map((rule, index) => (
            <div className="mono" key={index}>
              {rule.matches?.length
                ? rule.matches
                    .map((match) =>
                      match.headers?.length
                        ? match.headers.map((header) => header.value).join(' & ')
                        : '所有模型',
                    )
                    .join(' / ')
                : '所有模型'}
            </div>
          ))}
        </td>
        <td>
          {model.rules?.map((rule, index) => (
            <div className="mono" key={index}>
              {rule.backendRefs?.map((ref) => (
                <div key={ref.name}>
                  <ResourceName namespace={resource.namespace} kind="ai-backends" name={ref.name} />
                  {ref.modelNameOverride ? ' → ' + ref.modelNameOverride : ''}
                </div>
              ))}
            </div>
          ))}
        </td>
        {created}
      </>
    );
  }
  const mcp = resource as MCPRoute;
  return (
    <>
      {name}
      {parents}
      <td className="mono">{mcp.path ?? '/mcp'}</td>
      <td className="mono">
        {mcp.backendRefs?.map((ref) => (
          <div key={ref.name}>
            <ResourceName namespace={resource.namespace} kind="backends" name={ref.name} /> ·{' '}
            {ref.path ?? '/mcp'}
          </div>
        ))}
      </td>
      {created}
    </>
  );
}
