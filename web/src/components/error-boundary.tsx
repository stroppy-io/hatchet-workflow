import { Component, type ErrorInfo, type ReactNode } from "react";

import { Button } from "@/components/ui/button";

type Props = { children: ReactNode };
type State = { error: Error | null };

// Top-level boundary so a render crash shows a recoverable fallback instead of
// a blank page. Catches only render/lifecycle errors (not async/event-handler
// errors — those go through errorMessage()/toast).
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("Unhandled UI error:", error, info);
  }

  render() {
    if (this.state.error) {
      return (
        <div className="flex min-h-screen flex-col items-center justify-center gap-4 bg-background p-8 text-center">
          <div>
            <h1 className="text-lg font-semibold">Something went wrong</h1>
            <p className="mt-1 text-sm text-muted-foreground">
              {this.state.error.message}
            </p>
          </div>
          <Button onClick={() => window.location.reload()}>Reload</Button>
        </div>
      );
    }
    return this.props.children;
  }
}
