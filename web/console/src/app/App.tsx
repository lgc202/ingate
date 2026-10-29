import { Component, useEffect, useState, type ReactNode, type FormEvent } from 'react';
import { NavLink, Navigate, Route, Routes, useSearchParams } from 'react-router-dom';
import { Layers3, GitBranch, Server, Folder, CircleUserRound } from 'lucide-react';
import { ResourcePage } from '../features/ResourcePage';
import { APIError, errorMessage, request } from '../api/client';
import { Login } from '../features/Login';
import { StatePanel } from '../components/ui';

class ErrorBoundary extends Component<{ children: ReactNode }, { failed: boolean }> {
  state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  render() {
    if (this.state.failed)
      return (
        <StatePanel
          title="页面无法显示"
          message="请重新载入页面；尚未提交的内容不会自动保存。"
          onRetry={() => window.location.reload()}
        />
      );

    return this.props.children;
  }
}

export function App() {
  const [params, setParams] = useSearchParams();
  const namespace = params.get('namespace') ?? '';
  const [draftNamespace, setDraftNamespace] = useState(namespace);
  const [account, setAccount] = useState('');
  const [checkingAccount, setCheckingAccount] = useState(true);
  const [accountError, setAccountError] = useState('');

  useEffect(() => {
    setDraftNamespace(namespace);
  }, [namespace]);
  useEffect(() => {
    const controller = new AbortController();
    request<{ account: { username: string } }>('/accounts/me', {
      signal: controller.signal,
    })
      .then(
        (reply) => {
          if (!controller.signal.aborted) setAccount(reply.account.username);
        },
        (cause) => {
          if (!controller.signal.aborted && !(cause instanceof APIError && cause.status === 401)) {
            setAccountError(errorMessage(cause));
          }
        },
      )
      .finally(() => {
        if (!controller.signal.aborted) setCheckingAccount(false);
      });

    const expired = () => setAccount('');
    window.addEventListener('ingate:unauthorized', expired);

    return () => {
      controller.abort();
      window.removeEventListener('ingate:unauthorized', expired);
    };
  }, []);

  if (checkingAccount) return <StatePanel title="正在验证会话" />;
  if (accountError)
    return (
      <StatePanel
        title="无法连接管理服务"
        message={accountError}
        onRetry={() => window.location.reload()}
      />
    );
  if (!account) return <Login onLogin={setAccount} />;

  const selectNamespace = (event: FormEvent) => {
    event.preventDefault();
    setParams({ namespace: draftNamespace.trim() });
  };
  const navigation = [
    { path: '/gateways', label: '网关', icon: Layers3 },
    { path: '/routes', label: '路由', icon: GitBranch },
    { path: '/services', label: '服务', icon: Server },
  ];
  const search = params.toString() ? '?' + params.toString() : '';

  return (
    <ErrorBoundary>
      <div className="shell">
        <aside className="sidebar">
          <div className="brand">
            <span className="brand-mark">E</span>
            <strong>Ingate</strong>
          </div>
          <div className="nav-caption">流量配置</div>
          <nav aria-label="流量配置">
            {navigation.map((item) => (
              <NavLink key={item.path} to={item.path + search} className="nav-item">
                <item.icon aria-hidden="true" />
                <span>{item.label}</span>
              </NavLink>
            ))}
          </nav>
          <div className="sidebar-bottom">
            <CircleUserRound aria-hidden="true" />
            <span>{account || '未登录'}</span>
            <button
              className="button"
              onClick={async () => {
                try {
                  await request('/auth/logout', { method: 'POST', body: '{}' });
                  setAccount('');
                } catch (cause) {
                  setAccountError(errorMessage(cause));
                }
              }}
            >
              退出
            </button>
          </div>
        </aside>
        <div className="workspace">
          <header className="topbar">
            <form className="namespace-form" onSubmit={selectNamespace}>
              <Folder aria-hidden="true" />
              <label htmlFor="namespace">Namespace</label>
              <input
                id="namespace"
                value={draftNamespace}
                required
                maxLength={63}
                pattern="[a-z0-9](([a-z0-9]|-)*[a-z0-9])?"
                placeholder="输入已有 Namespace"
                autoComplete="off"
                onChange={(event) => setDraftNamespace(event.target.value)}
              />
              <button className="button" type="submit">
                切换
              </button>
            </form>
            <span className="top-caption">管理控制台</span>
          </header>
          <main className="content">
            <Routes>
              {navigation.map((item) => (
                <Route
                  key={item.path}
                  path={item.path}
                  element={
                    namespace ? (
                      <ResourcePage
                        key={namespace + item.path}
                        namespace={namespace}
                        page={item.path.slice(1) as 'gateways' | 'routes' | 'services'}
                      />
                    ) : (
                      <StatePanel
                        title="请选择 Namespace"
                        message="在上方输入已创建的 Namespace。当前管理 API 没有命名空间列表接口，控制台不会创建或伪造命名空间。"
                      />
                    )
                  }
                />
              ))}
              <Route path="*" element={<Navigate replace to={'/gateways' + search} />} />
            </Routes>
          </main>
        </div>
      </div>
    </ErrorBoundary>
  );
}
