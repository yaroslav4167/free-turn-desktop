import {useEffect, useState} from 'react'
import {createPortal} from 'react-dom'
import {BrowserOpenURL} from '../api'
import {desktop, type ReleaseHistoryEntry} from '../desktop'
import {currentLanguage, tr} from '../language'
import {useDialogFocus} from '../useDialogFocus'
import {ReleaseNotesContent} from './ReleaseNotes'

export default function ReleaseHistoryDialog({repository, onClose}: {repository:string; onClose:()=>void}) {
  const dialog=useDialogFocus(onClose)
  const [entries,setEntries]=useState<ReleaseHistoryEntry[]>([])
  const [loading,setLoading]=useState(true)
  const [error,setError]=useState('')
  const [attempt,setAttempt]=useState(0)
  useEffect(()=>{
    let active=true
    setLoading(true);setError('')
    desktop().ReleaseHistory().then(value=>{if(active)setEntries(value)}).catch(e=>{if(active)setError(String(e))}).finally(()=>{if(active)setLoading(false)})
    return()=>{active=false}
  },[attempt])
  return createPortal(<div className="modal-backdrop" onClick={onClose}>
    <div ref={dialog} tabIndex={-1} className="modal update-dialog release-history-dialog" role="dialog" aria-modal="true" aria-labelledby="release-history-title" onClick={e=>e.stopPropagation()}>
      <div className="modal-heading">
        <h2 id="release-history-title">{tr('История обновлений', 'Update history')}</h2>
        <button className="btn ghost modal-close" aria-label={tr('Закрыть', 'Close')} onClick={onClose}>×</button>
      </div>
      {loading && <p role="status">{tr('Загрузка истории…', 'Loading update history…')}</p>}
      {error && <div className="notice error" role="alert">{error} <button className="btn small" onClick={()=>setAttempt(value=>value+1)}>{tr('Повторить', 'Retry')}</button></div>}
      {!loading && !error && !entries.length && <p>{tr('Опубликованных релизов пока нет.', 'No published releases yet.')}</p>}
      {!!entries.length && <div className="release-history-list" tabIndex={0} aria-label={tr('История обновлений', 'Update history')}>
        {entries.map(entry=><HistoryEntry key={entry.version} entry={entry} />)}
      </div>}
      <p className="settings-note">{tr('До 30 последних стабильных релизов из GitHub.', 'Up to 30 recent stable releases from GitHub.')}</p>
      <div className="inline-actions update-actions">
        <button className="btn" onClick={()=>BrowserOpenURL(`https://github.com/${repository}/releases`)}>{tr('Все релизы на GitHub', 'All releases on GitHub')}</button>
        <button className="btn" onClick={onClose}>{tr('Закрыть', 'Close')}</button>
      </div>
    </div>
  </div>,document.body)
}

function HistoryEntry({entry}: {entry:ReleaseHistoryEntry}) {
  const date=new Date(entry.publishedAt)
  return <article className="release-history-entry">
    <header><strong>{entry.version}</strong>{Number.isFinite(date.getTime()) && <time dateTime={entry.publishedAt}>{date.toLocaleDateString(currentLanguage())}</time>}</header>
    {entry.notes.trim() ? <ReleaseNotesContent text={entry.notes} /> : <p className="settings-note">{tr('Описание изменений не опубликовано.', 'Release notes have not been published.')}</p>}
  </article>
}
