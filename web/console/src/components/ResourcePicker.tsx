import { useEffect, useState } from 'react';
import { getResource } from '../api/resources';
import { errorMessage } from '../api/client';
import { useResourcePage } from '../api/useResource';
import { title, type Kind, type AnyResource, type Gateway } from '../domain/resources';

export function ResourcePicker({
  namespace,
  kind,
  label,
  value,
  required = true,
  onChange,
}: {
  namespace: string;
  kind: Kind;
  label: string;
  value: string;
  required?: boolean;
  onChange: (value: string) => void;
}) {
  const [query, setQuery] = useState('');
  const [search, setSearch] = useState('');
  const [pageNum, setPageNum] = useState(1);
  const [selected, setSelected] = useState<AnyResource>();
  const result = useResourcePage(
    namespace,
    kind,
    { pageNum, pageSize: 20, displayName: search },
    0,
  );

  useEffect(() => {
    setSelected(undefined);
    if (!value) return;
    const controller = new AbortController();
    getResource(namespace, kind, value, controller.signal)
      .then((resource) => {
        if (!controller.signal.aborted) setSelected(resource);
      })
      .catch(() => {
        /* 缺失引用仍保留原稳定标识，不能默默选成其他对象。 */
      });

    return () => controller.abort();
  }, [namespace, kind, value]);

  const items = result.page?.items ?? [];

  return (
    <div className="resource-picker">
      <div className="picker-search">
        <input
          aria-label={label + '名称查询'}
          placeholder="按名称查找"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              event.preventDefault();
              setPageNum(1);
              setSearch(query.trim());
            }
          }}
        />
        <button
          type="button"
          className="button"
          onClick={() => {
            setPageNum(1);
            setSearch(query.trim());
          }}
        >
          查找
        </button>
      </div>
      <select
        aria-label={label}
        required={required}
        value={value}
        onChange={(event) => onChange(event.target.value)}
      >
        <option value="">{result.loading ? '正在加载…' : '请选择'}</option>
        {value && !items.some((item) => item.name === value) && (
          <option value={value}>
            {selected ? title(selected) : '引用资源不在当前列表'} · {value}
          </option>
        )}
        {items.map((item) => (
          <option key={item.name} value={item.name}>
            {title(item)} · {item.name}
          </option>
        ))}
      </select>
      {result.error !== undefined && (
        <p className="error" role="alert">
          {errorMessage(result.error)}
        </p>
      )}
      {(pageNum > 1 || (result.page?.total ?? 0) > 20) && (
        <div className="picker-pages">
          <button
            type="button"
            disabled={result.loading || pageNum <= 1}
            onClick={() => setPageNum((value) => value - 1)}
          >
            上一页
          </button>
          <span>第 {pageNum} 页</span>
          <button
            type="button"
            disabled={result.loading || pageNum * 20 >= (result.page?.total ?? 0)}
            onClick={() => setPageNum((value) => value + 1)}
          >
            下一页
          </button>
        </div>
      )}
    </div>
  );
}

export function ListenerPicker({
  namespace,
  gatewayName,
  value,
  onChange,
}: {
  namespace: string;
  gatewayName: string;
  value: string;
  onChange: (value: string) => void;
}) {
  const [gateway, setGateway] = useState<Gateway>();
  const [error, setError] = useState<unknown>();

  useEffect(() => {
    setGateway(undefined);
    setError(undefined);
    if (!gatewayName) return;
    const controller = new AbortController();
    getResource(namespace, 'gateways', gatewayName, controller.signal).then(
      (resource) => {
        if (!controller.signal.aborted) setGateway(resource);
      },
      (cause) => {
        if (!controller.signal.aborted) setError(cause);
      },
    );

    return () => controller.abort();
  }, [namespace, gatewayName]);

  return (
    <div>
      <select
        aria-label="监听入口"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      >
        <option value="">全部合格入口</option>
        {value && !gateway?.listeners?.some((listener) => listener.name === value) && (
          <option value={value}>{value} · 未在当前网关中找到</option>
        )}
        {gateway?.listeners?.map((listener) => (
          <option key={listener.name} value={listener.name}>
            {listener.name} · :{listener.port} · {listener.hostname || '全部主机名'}
          </option>
        ))}
      </select>
      {error !== undefined && <p className="error">{errorMessage(error)}</p>}
    </div>
  );
}
