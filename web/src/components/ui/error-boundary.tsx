/**
 * error-boundary.tsx — React Error Boundary 组件。
 *
 * 捕获子组件渲染过程中的未预期异常，显示友好的降级 UI：
 * - 工业风全屏/卡片式错误回退
 * - 自动记录 error + errorInfo
 * - "重试"按钮重新挂载子组件
 * - 支持嵌套，上层 boundary 可覆盖下层
 *
 * 用法:
 * ```tsx
 * <ErrorBoundary fallback={<p>出错了</p>}>
 *   <MyPage />
 * </ErrorBoundary>
 *
 * <ErrorBoundary componentName="SessionList">
 *   <SessionList />
 * </ErrorBoundary>
 * ```
 */
"use client";

import { Component, type ErrorInfo, type ReactNode } from "react";
import { OctagonXIcon, RefreshCwIcon, AlertTriangleIcon } from "lucide-react";
import { Button } from "@/components/ui/button";

interface ErrorBoundaryProps {
  children: ReactNode;
  /** 自定义 fallback UI（覆盖默认的工业风错误展示） */
  fallback?: ReactNode | ((error: Error, retry: () => void) => ReactNode);
  /** 可选：出错的组件名称，用于日志和显示 */
  componentName?: string;
  /** 可选：出错时的额外回调（如上报监控） */
  onError?: (error: Error, errorInfo: ErrorInfo) => void;
}

interface ErrorBoundaryState {
  hasError: boolean;
  error: Error | null;
  errorInfo: ErrorInfo | null;
}

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = { hasError: false, error: null, errorInfo: null };
  }

  static getDerivedStateFromError(error: Error): Partial<ErrorBoundaryState> {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
    this.setState({ errorInfo });
    this.props.onError?.(error, errorInfo);

    // 开发环境打印完整错误
    if (process.env.NODE_ENV === "development") {
      console.error(`[ErrorBoundary${this.props.componentName ? `:${this.props.componentName}` : ""}]`, error, errorInfo);
    }
  }

  private handleRetry = (): void => {
    this.setState({ hasError: false, error: null, errorInfo: null });
  };

  render(): ReactNode {
    if (!this.state.hasError) return this.props.children;

    const { error, errorInfo } = this.state;
    const { fallback, componentName } = this.props;

    // 自定义 fallback（函数式）
    if (typeof fallback === "function") {
      return fallback(error!, this.handleRetry);
    }

    // 自定义 fallback（节点）
    if (fallback) return fallback as React.ReactNode;

    // 默认工业风降级 UI
    return (
      <div
        role="alert"
        className="mx-auto flex max-w-md flex-col items-center justify-center px-4 py-16 text-center"
      >
        <div className="relative mb-6">
          <OctagonXIcon className="size-12 text-destructive/60" />
          <AlertTriangleIcon className="absolute -bottom-1 -right-2 size-5 text-warning" />
        </div>

        <h2 className="font-display text-xl font-semibold text-foreground">
          {componentName ? `${componentName} 加载异常` : "页面渲染异常"}
        </h2>

        <p className="mt-2 max-w-sm text-sm text-muted-foreground">
          发生了未预期的错误。请尝试刷新，如果问题持续请联系技术支持。
        </p>

        {/* 开发环境显示错误详情 */}
        {process.env.NODE_ENV === "development" && error && (
          <details className="mt-4 w-full text-left">
            <summary className="cursor-pointer text-xs text-muted-foreground hover:text-foreground">
              错误详情（仅开发环境）
            </summary>
            <pre className="mt-2 max-h-48 overflow-auto rounded bg-muted p-3 text-xs text-muted-foreground">
              {error.name}: {error.message}
              {"\n\n"}
              {error.stack}
              {errorInfo?.componentStack && (
                <>
                  {"\n\n--- Component Stack ---\n"}
                  {errorInfo.componentStack}
                </>
              )}
            </pre>
          </details>
        )}

        <div className="mt-6 flex items-center gap-3">
          <Button variant="outline" onClick={() => window.location.reload()}>
            <RefreshCwIcon className="mr-1.5 size-4" />
            刷新页面
          </Button>
          <Button onClick={this.handleRetry}>
            重试
          </Button>
        </div>
      </div>
    );
  }
}
