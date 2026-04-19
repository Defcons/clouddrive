import React from 'react'

type Props = { children: React.ReactNode }
type State = { error: Error | null }

export default class ErrorBoundary extends React.Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    // eslint-disable-next-line no-console
    console.error('Unhandled React error:', error, info)
  }

  private handleReset = () => {
    this.setState({ error: null })
  }

  private handleReload = () => {
    window.location.reload()
  }

  render() {
    if (this.state.error) {
      return (
        <div className="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-900 px-4">
          <div className="max-w-md w-full bg-white dark:bg-gray-800 rounded-2xl shadow p-8">
            <h1 className="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-2">
              Something went wrong
            </h1>
            <p className="text-sm text-gray-600 dark:text-gray-300 mb-4">
              CloudDrive hit an unexpected error. You can try to recover, or
              reload the page if the error persists.
            </p>
            <details className="text-xs text-gray-500 dark:text-gray-400 mb-4 font-mono bg-gray-50 dark:bg-gray-900 p-3 rounded">
              <summary className="cursor-pointer mb-2">Technical details</summary>
              {this.state.error.message}
              {'\n'}
              {this.state.error.stack}
            </details>
            <div className="flex gap-2">
              <button
                onClick={this.handleReset}
                className="flex-1 min-h-11 px-4 py-2 bg-blue-600 text-white rounded-lg text-sm font-medium hover:bg-blue-700"
              >
                Try again
              </button>
              <button
                onClick={this.handleReload}
                className="flex-1 min-h-11 px-4 py-2 bg-gray-200 dark:bg-gray-700 text-gray-800 dark:text-gray-200 rounded-lg text-sm font-medium hover:bg-gray-300 dark:hover:bg-gray-600"
              >
                Reload page
              </button>
            </div>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}
