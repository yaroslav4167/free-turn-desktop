import {useEffect, useRef, useState} from 'react'
import {ConfigCommand, CopyText} from '../api'
import {tr} from '../language'

// Keyed by profile and saved/draft state in ServerDetail: a late response can
// never reveal the previous profile's command in the newly selected profile.
export default function ProfileCommand({serverId}: {serverId:string}) {
  const [command,setCommand]=useState('')
  const [busy,setBusy]=useState(false)
  const [error,setError]=useState('')
  const [copied,setCopied]=useState(false)
  const active=useRef(false)
  useEffect(()=>{active.current=true;return()=>{active.current=false}},[])
  const generate=async()=>{
    if(command){setCommand('');setCopied(false);setError('');return}
    setBusy(true);setError('')
    try {const value=await ConfigCommand(serverId);if(active.current)setCommand(value)}
    catch(e){if(active.current)setError(String(e))}
    finally{if(active.current)setBusy(false)}
  }
  return <details className="advanced-settings">
    <summary><strong>{tr('Диагностика и CLI', 'Diagnostics and CLI')}</strong></summary>
    <div className="advanced-content">
      <p className="settings-note">{tr('Команда строится по сохранённому профилю и может содержать ключи и ссылки. Не публикуйте её.', 'The command uses the saved profile and may contain keys and links. Do not publish it.')}</p>
      {error&&<div className="notice error" role="alert">{error}</div>}
      <div className="inline-actions">
        <button type="button" className="btn small" disabled={busy} onClick={generate}>{busy?tr('Подготовка…','Preparing…'):command?tr('Скрыть команду','Hide command'):tr('Показать эквивалент CLI','Show equivalent CLI command')}</button>
        {command&&<button type="button" className="btn small" onClick={async()=>{
          setError('')
          try {
            await CopyText(command)
            if(active.current)setCopied(true)
          } catch(e) {
            if(active.current)setError(String(e))
          }
        }}>{copied?tr('Скопировано','Copied'):tr('Копировать','Copy')}</button>}
      </div>
      {command&&<pre className="cli-command mono">{command}</pre>}
    </div>
  </details>
}
