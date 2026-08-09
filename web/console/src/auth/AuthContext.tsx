import { createContext, useContext, useEffect, useRef, useState } from 'react';
import type { PropsWithChildren } from 'react';
import { LogIn, ShieldCheck } from 'lucide-react';
import { UserManager, WebStorageStateStore } from 'oidc-client-ts';
import type { User } from 'oidc-client-ts';
import {
  getAuthenticationConfiguration,
  getCurrentPrincipal,
} from '@/api/authentication';
import type {
  AuthenticationConfiguration,
  CurrentPrincipal,
} from '@/api/authentication';
import { configureAuthentication } from '@/api/client';

interface AuthState {
  loading: boolean;
  enabled: boolean;
  user?: User;
  principal?: CurrentPrincipal;
  error?: string;
}

interface AuthContextValue extends AuthState {
  canWriteConfiguration: boolean;
  isAdmin: boolean;
  signIn: () => Promise<void>;
  signOut: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: PropsWithChildren) {
  const [state, setState] = useState<AuthState>({ loading: true, enabled: false });
  const manager = useRef<UserManager | null>(null);

  useEffect(() => {
    let active = true;
    let userManager: UserManager | null = null;
    let handleTokenExpired: (() => void) | undefined;
    let handleUserLoaded: ((user: User) => void) | undefined;

    const clearUser = () => {
      if (active) setState({ loading: false, enabled: true });
    };

    const loadPrincipal = async (user: User) => {
      try {
        const principal = await getCurrentPrincipal();
        if (active) setState({ loading: false, enabled: true, user, principal });
      } catch (error) {
        if (active) {
          setState({
            loading: false,
            enabled: true,
            error: error instanceof Error ? error.message : '无法读取当前账号权限',
          });
        }
      }
    };

    const initialize = async () => {
      try {
        const config = await getAuthenticationConfiguration();
        if (!active) return;
        if (!config.enabled) {
          configureAuthentication(async () => undefined, () => undefined);
          setState({
            loading: false,
            enabled: false,
            principal: {
              subject: 'authentication-disabled',
              name: '本地管理员',
              email: '',
              role: 'admin',
            },
          });
          return;
        }

        userManager = createUserManager(config);
        manager.current = userManager;
        configureAuthentication(
          async () => {
            const user = await userManager?.getUser();
            return user && !user.expired ? user.access_token : undefined;
          },
          () => {
            void userManager?.removeUser();
            clearUser();
          },
        );

        handleTokenExpired = () => {
          void userManager?.removeUser();
          clearUser();
        };
        handleUserLoaded = (user) => {
          void loadPrincipal(user);
        };
        userManager.events.addAccessTokenExpired(handleTokenExpired);
        userManager.events.addUserLoaded(handleUserLoaded);

        let user: User | null;
        if (window.location.pathname === '/auth/callback') {
          user = await userManager.signinRedirectCallback();
          const returnTo = readReturnPath(user.state);
          window.history.replaceState({}, document.title, returnTo);
        } else if (window.location.pathname === '/auth/silent-callback') {
          await userManager.signinSilentCallback();
          return;
        } else {
          user = await userManager.getUser();
        }

        if (!user || user.expired) {
          clearUser();
          return;
        }
        await loadPrincipal(user);
      } catch (error) {
        if (active) {
          setState({
            loading: false,
            enabled: true,
            error: error instanceof Error ? error.message : '登录初始化失败',
          });
        }
      }
    };

    void initialize();
    return () => {
      active = false;
      if (userManager && handleTokenExpired) {
        userManager.events.removeAccessTokenExpired(handleTokenExpired);
      }
      if (userManager && handleUserLoaded) {
        userManager.events.removeUserLoaded(handleUserLoaded);
      }
      userManager?.stopSilentRenew();
      if (manager.current === userManager) manager.current = null;
    };
  }, []);

  const signIn = async () => {
    const userManager = manager.current;
    if (!userManager) return;
    const returnTo = `${window.location.pathname}${window.location.search}${window.location.hash}`;
    await userManager.signinRedirect({ state: { returnTo } });
  };

  const signOut = async () => {
    const userManager = manager.current;
    if (!userManager) return;
    try {
      await userManager.signoutRedirect();
    } catch {
      await userManager.removeUser();
      setState({ loading: false, enabled: true });
    }
  };

  if (state.loading) {
    return <AuthMessage title="正在连接身份服务" message="正在恢复安全登录状态…" />;
  }
  if (state.enabled && !state.principal) {
    return (
      <LoginPage
        error={state.error}
        onSignIn={() => {
          void signIn();
        }}
      />
    );
  }

  return (
    <AuthContext.Provider value={{
      ...state,
      canWriteConfiguration: state.principal?.role === 'operator' || state.principal?.role === 'admin',
      isAdmin: state.principal?.role === 'admin',
      signIn,
      signOut,
    }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error('useAuth must be used inside AuthProvider');
  return value;
}

function createUserManager(config: AuthenticationConfiguration) {
  const origin = window.location.origin;
  return new UserManager({
    authority: config.issuer,
    client_id: config.clientID,
    redirect_uri: `${origin}/auth/callback`,
    silent_redirect_uri: `${origin}/auth/silent-callback`,
    post_logout_redirect_uri: origin,
    response_type: 'code',
    scope: config.scopes.join(' '),
    automaticSilentRenew: true,
    monitorSession: true,
    userStore: new WebStorageStateStore({ store: window.sessionStorage }),
  });
}

function readReturnPath(state: unknown) {
  if (typeof state !== 'object' || state === null) return '/gateways';
  const returnTo = (state as { returnTo?: unknown }).returnTo;
  return typeof returnTo === 'string' && returnTo.startsWith('/') ? returnTo : '/gateways';
}

function LoginPage({ error, onSignIn }: { error?: string; onSignIn: () => void }) {
  return (
    <main className="min-h-screen bg-slate-950 text-slate-100 grid place-items-center px-6">
      <section className="w-full max-w-md rounded-2xl border border-slate-800 bg-slate-900/90 p-8 shadow-2xl shadow-slate-950/60">
        <div className="mb-8 flex h-12 w-12 items-center justify-center rounded-xl bg-blue-600 text-white shadow-lg shadow-blue-950/40">
          <ShieldCheck className="h-6 w-6" />
        </div>
        <p className="mb-2 text-xs font-semibold uppercase tracking-[0.22em] text-blue-400">Ingate Console</p>
        <h1 className="text-2xl font-semibold tracking-tight text-white">登录网关管理台</h1>
        <p className="mt-3 text-sm leading-6 text-slate-400">
          使用企业身份账号登录。权限由身份服务中的 Ingate 角色统一管理。
        </p>
        {error && (
          <div className="mt-5 rounded-lg border border-red-900/70 bg-red-950/50 px-4 py-3 text-sm text-red-300">
            {error}
          </div>
        )}
        <button
          type="button"
          onClick={onSignIn}
          className="mt-7 flex w-full cursor-pointer items-center justify-center gap-2 rounded-lg bg-blue-600 px-4 py-3 text-sm font-semibold text-white transition hover:bg-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-400 focus:ring-offset-2 focus:ring-offset-slate-900"
        >
          <LogIn className="h-4 w-4" />
          使用企业账号登录
        </button>
      </section>
    </main>
  );
}

function AuthMessage({ title, message }: { title: string; message: string }) {
  return (
    <main className="min-h-screen bg-slate-950 text-slate-100 grid place-items-center px-6">
      <div className="text-center">
        <div className="mx-auto mb-5 h-8 w-8 animate-spin rounded-full border-2 border-slate-700 border-t-blue-500" />
        <h1 className="text-base font-semibold">{title}</h1>
        <p className="mt-2 text-sm text-slate-500">{message}</p>
      </div>
    </main>
  );
}
