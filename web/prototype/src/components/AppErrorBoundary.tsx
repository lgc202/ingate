import { Component, type ErrorInfo, type ReactNode } from "react";

interface AppErrorBoundaryState {
  failed: boolean;
}

export class AppErrorBoundary extends Component<
  { children: ReactNode },
  AppErrorBoundaryState
> {
  state: AppErrorBoundaryState = { failed: false };

  static getDerivedStateFromError(): AppErrorBoundaryState {
    return { failed: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("prototype rendering failed", error, info);
  }

  render() {
    if (!this.state.failed) return this.props.children;

    return (
      <main className="app-error">
        <span>页面加载失败</span>
        <h1>控制台暂时无法显示</h1>
        <p>演示数据没有丢失，重新加载后即可继续。</p>
        <button type="button" onClick={() => window.location.reload()}>
          重新加载
        </button>
      </main>
    );
  }
}
