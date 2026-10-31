import { Component, type ReactNode } from 'react';

interface State {
  error: Error | null;
}

/** Catches rendering errors in its children and shows a recoverable message instead of a blank page. */
export class ErrorBoundary extends Component<{ children: ReactNode }, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  render() {
    if (!this.state.error) return this.props.children;
    return (
      <div className="panel" role="alert">
        <h1>Something went wrong</h1>
        <p>{this.state.error.message}</p>
        <button type="button" className="button" onClick={() => this.setState({ error: null })}>Try again</button>
      </div>
    );
  }
}
