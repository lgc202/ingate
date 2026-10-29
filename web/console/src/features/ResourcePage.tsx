import { useEffect, useRef, useState, type FormEvent } from 'react';
import { Plus, RefreshCw, Search } from 'lucide-react';
import { waitForProjection, type SavedChange } from '../api/projection';
import { useResourcePage } from '../api/useResource';
import { deleteResource, getResource, type ListQuery } from '../api/resources';
import { errorMessage } from '../api/client';
import { labels, title, type Kind, type AnyResource } from '../domain/resources';
import { Confirm, StatePanel } from '../components/ui';
import { RowActions } from '../components/RowActions';
import { ResourcePicker } from '../components/ResourcePicker';
import { ResourceEditor } from './ResourceEditor';
import { ResourceDetail } from './ResourceDetail';
import { ResourceCells } from './ResourceCells';

const pageLabels = { gateways: '网关', routes: '路由', services: '服务' };
const pageKinds: Record<keyof typeof pageLabels, Kind[]> = {
  gateways: ['gateways'],
  routes: ['http-routes', 'mcp-routes', 'model-routes'],
  services: ['backends', 'ai-backends'],
};

export function ResourcePage({
  namespace,
  page,
}: {
  namespace: string;
  page: keyof typeof pageLabels;
}) {
  const [kind, setKind] = useState<Kind>(pageKinds[page][0]);
  const [draft, setDraft] = useState({ displayName: '', gatewayName: '' });
  const [query, setQuery] = useState<ListQuery>({ pageNum: 1, pageSize: 20 });
  const [revision, setRevision] = useState(0);
  const [notice, setNotice] = useState('');
  const [noticeTone, setNoticeTone] = useState<'success' | 'warning' | 'error'>('success');
  const [syncing, setSyncing] = useState(false);
  const [opening, setOpening] = useState(false);
  const [editor, setEditor] = useState<{ resource?: AnyResource }>();
  const [detail, setDetail] = useState<AnyResource>();
  const [candidate, setCandidate] = useState<AnyResource>();
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState('');
  const openingRequest = useRef<AbortController | undefined>(undefined);
  const projectionRequest = useRef<AbortController | undefined>(undefined);
  const result = useResourcePage(namespace, kind, query, revision);
  const label = pageLabels[page];

  useEffect(
    () => () => {
      openingRequest.current?.abort();
      projectionRequest.current?.abort();
    },
    [],
  );

  useEffect(() => {
    if (!notice || noticeTone !== 'success' || syncing) return;
    const timer = setTimeout(() => setNotice(''), 5000);
    return () => clearTimeout(timer);
  }, [notice, noticeTone, syncing]);

  const refreshAfterChange = async (change: SavedChange, message: string) => {
    projectionRequest.current?.abort();
    const controller = new AbortController();
    projectionRequest.current = controller;
    setSyncing(true);
    setNoticeTone('success');
    setNotice(message);
    try {
      const converged = await waitForProjection(namespace, kind, change, controller.signal);
      if (controller.signal.aborted) return;
      if (!converged) {
        setNoticeTone('warning');
        setNotice(message + '，查询视图同步较慢，请稍后刷新。');
      }
    } catch (cause) {
      if (controller.signal.aborted) return;
      setNoticeTone('warning');
      setNotice(message + '，列表刷新失败：' + errorMessage(cause));
    } finally {
      if (!controller.signal.aborted) {
        setSyncing(false);
        setRevision((value) => value + 1);
      }
    }
  };

  const apply = (event: FormEvent) => {
    event.preventDefault();
    setQuery({
      ...query,
      pageNum: 1,
      displayName: draft.displayName.trim(),
      gatewayName: page === 'routes' ? draft.gatewayName : undefined,
    });
    setRevision((value) => value + 1);
  };
  const changeKind = (next: Kind) => {
    openingRequest.current?.abort();
    projectionRequest.current?.abort();
    setSyncing(false);
    setNotice('');
    setOpening(false);
    setKind(next);
    setQuery({ pageNum: 1, pageSize: query.pageSize });
    setDraft({ displayName: '', gatewayName: '' });
    setDetail(undefined);
  };
  const open = async (resource: AnyResource, editing: boolean) => {
    openingRequest.current?.abort();
    const controller = new AbortController();
    openingRequest.current = controller;
    setOpening(true);
    setNotice('');
    try {
      const current = await getResource(namespace, kind, resource.name, controller.signal);
      if (controller.signal.aborted) return;
      if (editing) setEditor({ resource: current });
      else setDetail(current);
    } catch (error) {
      if (!controller.signal.aborted) {
        setNoticeTone('error');
        setNotice(errorMessage(error));
      }
    } finally {
      if (!controller.signal.aborted) setOpening(false);
    }
  };
  const remove = async () => {
    if (deleting || !candidate) return;
    setDeleting(true);
    setDeleteError('');
    try {
      await deleteResource(namespace, kind, candidate.name);
      const deleted = candidate.name;
      setCandidate(undefined);
      if (result.page?.items.length === 1 && query.pageNum > 1)
        setQuery((current) => ({ ...current, pageNum: current.pageNum - 1 }));
      void refreshAfterChange({ deleted }, '已删除「' + title(candidate) + '」');
    } catch (error) {
      setDeleteError(errorMessage(error));
    } finally {
      setDeleting(false);
    }
  };
  const columns =
    page === 'gateways'
      ? ['网关名称', '监听入口', '主机名', '创建时间']
      : page === 'services'
        ? [
            '服务名称',
            '类型',
            kind === 'backends' ? '服务地址' : '网络服务引用',
            '连接 / 协议',
            '创建时间',
          ]
        : ['路由名称', '入口网关', '请求匹配', '转发目标', '创建时间'];

  return (
    <>
      <header className="page-heading">
        <h1>{label}</h1>
        <button
          className="button primary"
          disabled={opening || syncing}
          onClick={() => {
            setNotice('');
            setEditor({});
          }}
        >
          <Plus />
          创建{labels[kind]}
        </button>
      </header>
      {notice && (
        <div className={'notice ' + noticeTone} role={noticeTone === 'error' ? 'alert' : 'status'}>
          {notice}
          <button aria-label="关闭提示" onClick={() => setNotice('')}>
            ×
          </button>
        </div>
      )}
      <section className="panel">
        <form className="filter-panel" onSubmit={apply}>
          <div className="filter-fields">
            <label>
              {label}名称
              <input
                aria-label={label + '名称查询'}
                placeholder={'搜索' + label + '名称'}
                value={draft.displayName}
                onChange={(event) => setDraft({ ...draft, displayName: event.target.value })}
              />
            </label>
            {page !== 'gateways' && (
              <label>
                {label}类型
                <select
                  aria-label={label + '类型'}
                  value={kind}
                  onChange={(event) => changeKind(event.target.value as Kind)}
                >
                  {pageKinds[page].map((value) => (
                    <option key={value} value={value}>
                      {labels[value]}
                    </option>
                  ))}
                </select>
              </label>
            )}
            {page === 'routes' && (
              <div className="filter-reference">
                <span>入口网关</span>
                <ResourcePicker
                  namespace={namespace}
                  kind="gateways"
                  label="入口网关"
                  required={false}
                  value={draft.gatewayName}
                  onChange={(gatewayName) => setDraft({ ...draft, gatewayName })}
                />
              </div>
            )}
            <div className="filter-actions">
              <button
                type="button"
                className="button"
                onClick={() => {
                  setDraft({ displayName: '', gatewayName: '' });
                  setQuery({ pageNum: 1, pageSize: query.pageSize });
                  setRevision((value) => value + 1);
                }}
              >
                重置
              </button>
              <button type="submit" className="button primary">
                <Search />
                查询
              </button>
            </div>
          </div>
        </form>
        <header className="list-heading">
          <strong>{labels[kind]}列表</strong>
          <div className="list-tools">
            {(syncing || (result.loading && result.page)) && (
              <span className="refresh-status" role="status">
                正在更新列表…
              </span>
            )}
            <button
              className="icon-button"
              aria-label="刷新列表"
              disabled={result.loading || opening || syncing}
              onClick={() => {
                setNotice('');
                setRevision((value) => value + 1);
              }}
            >
              <RefreshCw className={result.loading || syncing ? 'spinning' : ''} />
            </button>
          </div>
        </header>
        {result.error !== undefined && result.page && (
          <div className="notice error" role="alert">
            刷新失败：{errorMessage(result.error)}
          </div>
        )}
        {!result.page && result.loading ? (
          <StatePanel title="正在加载…" />
        ) : !result.page && result.error !== undefined ? (
          <StatePanel
            title="加载失败"
            message={errorMessage(result.error)}
            onRetry={() => setRevision((value) => value + 1)}
          />
        ) : !result.page?.items.length ? (
          <StatePanel
            title={query.displayName || query.gatewayName ? '没有匹配的资源' : '暂无资源'}
          />
        ) : (
          <div
            className={'table-scroll' + (opening || syncing ? ' opening' : '')}
            aria-busy={opening || syncing || result.loading}
          >
            <table>
              <thead>
                <tr>
                  {columns.map((column) => (
                    <th key={column}>{column}</th>
                  ))}
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {result.page.items.map((resource) => (
                  <tr key={resource.name}>
                    <ResourceCells
                      key={revision}
                      kind={kind}
                      resource={resource}
                      disabled={opening || syncing}
                      onDetail={() => void open(resource, false)}
                    />
                    <td>
                      <RowActions
                        disabled={opening || syncing}
                        onDetail={() => void open(resource, false)}
                        onEdit={() => void open(resource, true)}
                        onDelete={() => {
                          setDeleteError('');
                          setCandidate(resource);
                        }}
                      />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {result.page && (
          <footer className="pagination">
            <span>共 {result.page.total} 条</span>
            <div>
              <label>
                每页{' '}
                <select
                  aria-label="每页数量"
                  value={query.pageSize}
                  onChange={(event) =>
                    setQuery({
                      ...query,
                      pageNum: 1,
                      pageSize: Number(event.target.value),
                    })
                  }
                >
                  {[20, 50, 100].map((size) => (
                    <option key={size}>{size}</option>
                  ))}
                </select>{' '}
                条
              </label>
              <button
                className="button"
                disabled={result.loading || query.pageNum === 1}
                onClick={() => setQuery({ ...query, pageNum: query.pageNum - 1 })}
              >
                上一页
              </button>
              <span>{query.pageNum}</span>
              <button
                className="button"
                disabled={result.loading || query.pageNum * query.pageSize >= result.page.total}
                onClick={() => setQuery({ ...query, pageNum: query.pageNum + 1 })}
              >
                下一页
              </button>
            </div>
          </footer>
        )}
      </section>
      {editor && (
        <ResourceEditor
          key={kind + (editor.resource?.name ?? 'new')}
          namespace={namespace}
          kind={kind}
          observed={editor.resource}
          onClose={() => setEditor(undefined)}
          onSaved={(resource) => {
            setEditor(undefined);
            if (!editor.resource) {
              setDraft({ displayName: '', gatewayName: '' });
              setQuery({ pageNum: 1, pageSize: query.pageSize });
            }
            void refreshAfterChange(
              { saved: resource },
              (editor.resource ? '已保存「' : '已创建「') + title(resource) + '」',
            );
          }}
        />
      )}
      {detail && (
        <ResourceDetail
          namespace={namespace}
          kind={kind}
          resource={detail}
          onClose={() => setDetail(undefined)}
          onEdit={() => {
            const resource = detail;
            setDetail(undefined);
            setEditor({ resource });
          }}
        />
      )}
      {candidate && (
        <Confirm
          title={'删除' + labels[kind]}
          busy={deleting}
          error={deleteError}
          onClose={() => setCandidate(undefined)}
          onConfirm={() => void remove()}
        >
          <p>确认删除「{title(candidate)}」？</p>
          <p className="mono muted">
            {namespace} · {candidate.name}
          </p>
          <p className="muted">只删除当前资源，不会自动级联删除其他资源。</p>
        </Confirm>
      )}
    </>
  );
}
