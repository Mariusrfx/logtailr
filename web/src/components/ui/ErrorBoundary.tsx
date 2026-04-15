import { Component, type ErrorInfo, type ReactNode } from "react"

interface Props {
  children: ReactNode
  fallback?: ReactNode
  section?: string
}

interface State {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error(`ErrorBoundary [${this.props.section || "app"}]:`, error, info.componentStack)
  }

  handleReset = () => {
    this.setState({ hasError: false, error: null })
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) return this.props.fallback

      return (
        <div className="flex items-center justify-center min-h-[200px] p-8">
          <div className="text-center max-w-sm">
            <div className="text-4xl mb-3">:(</div>
            <h2 className="text-lg font-semibold text-text-primary mb-1">
              Something went wrong
            </h2>
            <p className="text-sm text-text-secondary mb-4">
              {this.props.section
                ? `An error occurred in the ${this.props.section} section.`
                : "An unexpected error occurred."}
            </p>
            {this.state.error && (
              <pre className="text-xs text-error bg-error/5 border border-error/10 rounded p-3 mb-4 text-left overflow-auto max-h-24">
                {this.state.error.message}
              </pre>
            )}
            <button
              onClick={this.handleReset}
              className="px-4 py-2 text-sm rounded-md bg-accent text-white hover:bg-accent/90 transition-colors"
            >
              Try again
            </button>
          </div>
        </div>
      )
    }

    return this.props.children
  }
}
