import {useState} from 'react'
import {createPortal} from 'react-dom'
import {desktop} from '../desktop'
import {tr} from '../language'
import {useDialogFocus} from '../useDialogFocus'
import Icon from './Icon'

export default function DiagnosticsDialog({recovery, onClose, onRestored}: {recovery:string; onClose:()=>void; onRestored:()=>Promise<void>}) {
  const dialog = useDialogFocus(onClose)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState(recovery)
  const run = async (action:()=>Promise<void>) => {
    setBusy(true); setError('')
    try { await action() } catch(e) { setError(String(e)) } finally { setBusy(false) }
  }
  return createPortal(<div className="modal-backdrop" onClick={onClose}>
    <div ref={dialog} tabIndex={-1} className="modal diagnostics-dialog" role="dialog" aria-modal="true" aria-labelledby="diagnostics-dialog-title" onClick={e=>e.stopPropagation()}>
      <div className="modal-heading">
        <h2 id="diagnostics-dialog-title">{tr('Диагностика и восстановление', 'Diagnostics and recovery')}</h2>
        <button className="btn ghost modal-close" aria-label={tr('Закрыть', 'Close')} onClick={onClose}><Icon name="close" size={18} /></button>
      </div>
      {error && <div className="notice error" role="alert">{error}</div>}
      {notice && <p className="notice info" role="status">{notice === 'restored' ? tr('Профили восстановлены из резервной копии.', 'Profiles restored from backup.') : notice === 'recovered' ? tr('Профили восстановлены из резервной копии. Повреждённый файл сохранён отдельно.', 'Profiles recovered from backup. The damaged file was preserved separately.') : tr('Файл профилей повреждён; исходные данные сохранены. Импортируйте профили или восстановите резервную копию.', 'The profile file is damaged; original data was preserved. Import profiles or restore a backup.')}</p>}
      <section className="diagnostics-section">
        <h3>{tr('Диагностика', 'Diagnostics')}</h3>
        <p>{tr('Отчёт не содержит ключей, адресов серверов, ссылок captcha или исходного журнала. Проверка реального VPN не выполняется.', 'The report excludes keys, server addresses, captcha URLs and raw logs. It does not perform a live VPN test.')}</p>
        <button className="btn" disabled={busy} onClick={()=>run(()=>desktop().ExportDiagnostics())}><Icon name="download" size={17} /> {tr('Сохранить диагностику', 'Save diagnostics')}</button>
      </section>
      <section className="diagnostics-section">
        <h3>{tr('Восстановление профилей', 'Profile recovery')}</h3>
        <p>{tr('При повреждении файла профили восстанавливаются автоматически из исправной копии. Ручное восстановление — запасной откат после нежелательных изменений или удаления профилей.', 'Damaged profiles are automatically recovered from a valid backup. Manual recovery is a fallback for unwanted changes or deleted profiles.')}</p>
        <button className="btn danger" disabled={busy} onClick={()=>{
          if (!window.confirm(tr('Заменить список профилей последней резервной копией? Сначала отключите соединение.', 'Replace profiles with the last backup? Disconnect first.'))) return
          run(async()=>{await desktop().RestoreProfilesBackup(); await onRestored(); setNotice(await desktop().ProfileRecoveryNotice())})
        }}>{tr('Восстановить профили', 'Restore profiles')}</button>
      </section>
    </div>
  </div>, document.body)
}
