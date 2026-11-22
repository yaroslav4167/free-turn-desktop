import {useEffect, useRef} from 'react'
import {currentLanguage, tr} from '../language'

export interface LogEntry {
  level: string
  msg: string
  atMs: number
}

interface Props {
  lines: LogEntry[]
  onClear: () => void
}

export default function LogConsole({lines, onClear}: Props) {
  const bodyRef = useRef<HTMLDivElement>(null)
  const followTail = useRef(true)

  useEffect(() => {
    const el = bodyRef.current
    if (el && followTail.current) el.scrollTop = el.scrollHeight
  }, [lines])

  return (
    <div className="log-console">
      <div className="log-console-header">
        <span>{tr('Логи', 'Logs')}</span>
        <button className="btn small ghost" onClick={onClear}>
          {tr('Очистить', 'Clear')}
        </button>
      </div>
      <div className="log-lines" ref={bodyRef} onScroll={e => {
        const el = e.currentTarget
        followTail.current = el.scrollHeight - el.scrollTop - el.clientHeight < 32
      }}>
        {lines.map((l, i) => (
          <div key={i} className={'log-line' + (l.level === 'error' ? ' error' : l.level === 'warn' ? ' warn' : '')}>
            <span className="ts">{new Date(l.atMs).toLocaleTimeString(currentLanguage())}</span>
            {l.msg}
          </div>
        ))}
      </div>
    </div>
  )
}
