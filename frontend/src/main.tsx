import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import './styles.css'

type ErrorBoundaryProps = {
  children: React.ReactNode
}

type ErrorBoundaryState = {
  hasError: boolean
  message: string
}

class RootErrorBoundary extends React.Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props)
    this.state = { hasError: false, message: '' }
  }

  static getDerivedStateFromError(error: unknown): ErrorBoundaryState {
    return {
      hasError: true,
      message: error instanceof Error ? error.message : 'Unknown renderer error',
    }
  }

  componentDidCatch(error: unknown) {
    // Keep stack trace visible in devtools while preserving app shell.
    console.error('Root renderer error', error)
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="min-h-screen bg-[radial-gradient(circle_at_top,_rgba(249,115,22,0.16),_transparent_28%),linear-gradient(180deg,_#020617,_#111827)] px-6 py-12 text-slate-100">
          <div className="mx-auto max-w-2xl rounded-3xl border border-red-400/30 bg-slate-900/80 p-6 shadow-xl shadow-black/20">
            <h1 className="text-xl font-semibold text-red-300">UI runtime error</h1>
            <p className="mt-3 text-sm text-slate-300">The app hit an unexpected renderer exception but stayed running. Restarting the app should recover immediately.</p>
            <pre className="mt-4 overflow-auto rounded-2xl border border-white/10 bg-black/30 p-4 text-xs text-slate-300">{this.state.message}</pre>
          </div>
        </div>
      )
    }

    return this.props.children
  }
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <RootErrorBoundary>
      <App />
    </RootErrorBoundary>
  </React.StrictMode>,
)
