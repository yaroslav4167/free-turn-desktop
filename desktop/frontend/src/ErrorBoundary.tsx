import {Component, ErrorInfo, ReactNode} from 'react'
import {tr} from './language'

interface Props {
  children: ReactNode
}

interface State {
  error: Error | null
}

export default class ErrorBoundary extends Component<Props, State> {
  state: State = {error: null}

  static getDerivedStateFromError(error: Error): State {
    return {error}
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('UI crashed:', error, info.componentStack)
  }

  render() {
    if (this.state.error) {
      return (
        <div style={{padding: 24, color: 'var(--text)', fontFamily: 'Segoe UI, sans-serif'}}>
          <h2 style={{color: 'var(--err)'}}>{tr('Интерфейс упал с ошибкой', 'The interface encountered an error')}</h2>
          <p style={{color: 'var(--text-dim)'}}>{this.state.error.message}</p>
          <pre style={{whiteSpace: 'pre-wrap', fontSize: 11, color: 'var(--text-dim)', maxHeight: 300, overflow: 'auto'}}>
            {this.state.error.stack}
          </pre>
          <button
            style={{marginTop: 12, padding: '7px 14px', borderRadius: 7, border: '1px solid var(--border)', background: 'var(--bg-elevated)', color: 'var(--text)', cursor: 'pointer'}}
            onClick={() => this.setState({error: null})}
          >
            {tr('Попробовать снова', 'Try again')}
          </button>
        </div>
      )
    }
    return this.props.children
  }
}
