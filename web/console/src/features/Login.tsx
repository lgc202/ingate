import { useState, type FormEvent } from 'react';
import { request, errorMessage } from '../api/client';

export function Login({ onLogin }: { onLogin: (username: string) => void }) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError('');
    try {
      const reply = await request<{ account: { username: string } }>('/auth/login', {
        method: 'POST',
        body: JSON.stringify({ username, password }),
      });
      setPassword('');
      onLogin(reply.account.username);
    } catch (cause) {
      setPassword('');
      setError(errorMessage(cause));
    } finally {
      setBusy(false);
    }
  };

  return (
    <main className="login-page">
      <form className="login-card" onSubmit={submit}>
        <div className="brand">
          <span className="brand-mark">E</span>
          <strong>Ingate</strong>
        </div>
        <h1>登录管理控制台</h1>
        <label htmlFor="login-username">用户名</label>
        <input
          id="login-username"
          autoComplete="username"
          required
          maxLength={64}
          value={username}
          onChange={(event) => setUsername(event.target.value)}
          disabled={busy}
        />
        <label htmlFor="login-password">密码</label>
        <input
          id="login-password"
          type="password"
          autoComplete="current-password"
          required
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          disabled={busy}
        />
        {error && (
          <div className="notice error" role="alert">
            {error}
          </div>
        )}
        <button className="button primary" disabled={busy}>
          {busy ? '正在登录…' : '登录'}
        </button>
      </form>
    </main>
  );
}
