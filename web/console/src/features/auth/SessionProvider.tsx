import { createContext, FormEvent, ReactNode, useContext, useEffect, useMemo, useState } from 'react';
import { LogIn } from 'lucide-react';
import { getSession, login, logout, Session } from './session';

interface SessionContextValue {
  session: Session;
  signOut: () => Promise<void>;
}

const SessionContext = createContext<SessionContextValue | null>(null);

export function SessionProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<Session | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    void getSession()
      .then(setSession)
      .catch(() => setSession({ authenticated: false, username: '' }))
      .finally(() => setLoading(false));

    const handleUnauthorized = () => setSession({ authenticated: false, username: '' });
    window.addEventListener('ingate:unauthorized', handleUnauthorized);
    return () => window.removeEventListener('ingate:unauthorized', handleUnauthorized);
  }, []);

  const value = useMemo<SessionContextValue | null>(() => session?.authenticated ? {
    session,
    signOut: async () => {
      await logout();
      setSession({ authenticated: false, username: '' });
    },
  } : null, [session]);

  if (loading) return <SessionLoading />;
  if (!session?.authenticated) return <LoginPage onAuthenticated={setSession} />;
  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

export function useSession(): SessionContextValue {
  const value = useContext(SessionContext);
  if (!value) throw new Error('SessionProvider is required');
  return value;
}

function LoginPage({ onAuthenticated }: { onAuthenticated: (session: Session) => void }) {
  const [username, setUsername] = useState('admin');
  const [password, setPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setError('');
    try {
      onAuthenticated(await login(username.trim(), password));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '登录失败');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="login-page">
      <section className="login-card">
        <BrandMark />
        <div className="login-heading">
          <strong>Ingate</strong>
          <span>API 与 AI 网关</span>
        </div>
        <form onSubmit={submit}>
          <label>
            <span>用户名</span>
            <input autoComplete="username" value={username} onChange={(event) => setUsername(event.target.value)} required />
          </label>
          <label>
            <span>密码</span>
            <input type="password" autoComplete="current-password" value={password} onChange={(event) => setPassword(event.target.value)} required autoFocus />
          </label>
          {error ? <p role="alert">{error}</p> : null}
          <button type="submit" disabled={submitting}>
            <LogIn />
            {submitting ? '正在登录' : '登录控制台'}
          </button>
        </form>
      </section>
    </main>
  );
}

function SessionLoading() {
  return <main className="login-page"><div className="session-loading">正在连接 Ingate</div></main>;
}

function BrandMark() {
  return (
    <svg className="login-brand-mark" viewBox="0 0 40 40" fill="none" aria-hidden="true">
      <rect width="40" height="40" rx="11" fill="#e9edff" />
      <path d="M12 12.5h16v5.3H17.4v4.4H28v5.3H12v-15Z" fill="#3047c7" />
      <path d="m28 12.5-5.3 5.3H28v-5.3Z" fill="#ef8058" />
    </svg>
  );
}
