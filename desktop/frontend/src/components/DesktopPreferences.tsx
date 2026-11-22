import {useEffect, useState, useSyncExternalStore} from 'react'
import {desktop, DesktopSettings} from '../desktop'
import Icon from './Icon'
import UpdatePreferences from './UpdatePreferences'
import {backend} from '../../wailsjs/go/models'
import {availableLanguages, configureLanguage, tr} from '../language'
import FieldLabel from './FieldLabel'
import {configureTheme, subscribeTheme, themeSnapshot} from '../theme'
import ThemePicker from './ThemePicker'
import {resolveStartupProfile} from '../autoConnectSettings'

function PreferenceSwitch({id, title, hint, checked, disabled, onChange}: {
  id:string; title:string; hint:string; checked:boolean; disabled:boolean; onChange:(value:boolean)=>void
}) {
  return <div className="preference-row">
    <FieldLabel htmlFor={id} label={title} hint={hint} />
    <input id={id} type="checkbox" role="switch" checked={checked} disabled={disabled} onChange={e=>onChange(e.target.checked)} />
  </div>
}

export default function DesktopPreferences({version, active, servers, selectedServerId, onProfilesRestored}: {version: string; active: boolean; servers: backend.Server[]; selectedServerId?: string; onProfilesRestored: () => Promise<void>}) {
  const [settings, setSettings] = useState<DesktopSettings | null>(null)
  const theme = useSyncExternalStore(subscribeTheme, themeSnapshot)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [recovery, setRecovery] = useState('')
  const startupProfileId=resolveStartupProfile(servers,settings?.autoConnectServerId||'',selectedServerId)
  const savedStartupAvailable=!!settings?.autoConnectServerId&&servers.some(server=>server.id===settings.autoConnectServerId)
  const missingStartupProfile=!!settings&&!!(settings.autoConnect||settings.autoConnectServerId)&&!savedStartupAvailable
  const refresh = () => desktop().DesktopSettings().then(setSettings)
  useEffect(() => { refresh().catch(e=>setError(String(e))) }, [])
  useEffect(() => { if (active) desktop().ProfileRecoveryNotice().then(setRecovery).catch(() => {}) }, [active])

  const run = async (action: () => Promise<unknown>) => {
    setBusy(true); setError('')
    try {
      await action()
      await refresh()
    }
    catch (e) { setError(String(e)) }
    finally { setBusy(false) }
  }

  return <section className="settings-page" hidden={!active}>
    <header className="page-heading"><div>
      <span className="eyebrow">{tr('ПРИЛОЖЕНИЕ', 'APPLICATION')}</span>
      <h1>{tr('Настройки', 'Settings')}</h1>
    </div></header>
    {error && <div className="notice error" role="alert">{error}</div>}
    {!settings && !error && <p>{tr('Загрузка настроек…', 'Loading settings…')}</p>}
    {settings && <>
      <div className="settings-card">
        <h2><Icon name="settings" /> {tr('Поведение', 'Behavior')}</h2>
        <div className="preference-row language-row">
          <FieldLabel htmlFor="interface-language" label={tr('Язык / Language', 'Language / Язык')} hint={tr('Применяется сразу. Режим «Как в системе» использует язык операционной системы.', 'Applies immediately. System default follows the operating system language.')} />
          <select id="interface-language" aria-label={tr('Язык интерфейса', 'Interface language')} value={settings.language||'system'} disabled={busy} onChange={e=>{
            const language=e.target.value
            run(async()=>{await desktop().SetLanguage(language);configureLanguage(language,settings.systemLanguage)})
          }}>
            <option value="system">{tr('Как в системе', 'System default')}</option>
            {availableLanguages().map(language=><option key={language.code} value={language.code} lang={language.code}>{language.name}</option>)}
          </select>
        </div>
        <div className="preference-row theme-row">
          <FieldLabel htmlFor="interface-theme-system" label={tr('Тема оформления', 'Appearance')} hint={tr('Применяется сразу. «Как в системе» автоматически следует светлой или тёмной теме ОС.', 'Applies immediately. System default automatically follows the light or dark OS theme.')} />
          <ThemePicker value={theme} disabled={busy} onChange={value=>{
            run(async()=>{await desktop().SetTheme(value);configureTheme(value)})
          }} />
        </div>
        <PreferenceSwitch id="start-at-login" title={tr('Запускать вместе с системой', 'Start at login')}
          hint={tr('При входе в систему. Автоподключение настраивается отдельно ниже.', 'When signing in. Automatic connection is configured separately below.')}
          checked={settings.autostart} disabled={busy||!settings.supported} onChange={value=>run(()=>desktop().SetAutostart(value))} />
        <PreferenceSwitch id="connect-at-startup" title={tr('Подключаться при запуске', 'Connect on startup')}
          hint={tr('Одна попытка при запуске приложения. Проверка VK может потребовать действий пользователя; при ошибке подключитесь вручную.', 'One attempt when the app starts. VK verification may require user action; reconnect manually after a failure.')}
          checked={settings.autoConnect} disabled={busy||(!startupProfileId&&!settings.autoConnect)}
          onChange={value=>run(()=>desktop().SetAutoConnect(value,value?startupProfileId:settings.autoConnectServerId))} />
        {(settings.autoConnect || servers.length>0) && <div className="field autoconnect-profile">
          <FieldLabel htmlFor="startup-profile" label={tr('Профиль автоподключения', 'Startup connection profile')} hint={tr('Используется только при запуске приложения. Выключение автоподключения не удаляет выбранный профиль.', 'Used only at app startup. Disabling auto-connect does not remove the selected profile.')} />
          <select id="startup-profile" value={settings.autoConnect&&missingStartupProfile?'':startupProfileId} disabled={busy||!startupProfileId} onChange={e=>run(()=>desktop().SetAutoConnect(settings.autoConnect,e.target.value))}>
            <option value="" disabled>{tr('Выберите профиль', 'Select a profile')}</option>
            {servers.filter(s=>s.id.trim()).map(s=><option key={s.id} value={s.id}>{s.name||s.peer||s.id}</option>)}
          </select>
          {missingStartupProfile&&<p className="settings-note" role="status">{tr('Сохранённый профиль недоступен. Проверьте профиль автоподключения.', 'The saved profile is unavailable. Check the startup connection profile.')}</p>}
        </div>}
        {settings.autoConnectError&&<div className="notice error" role="alert">{settings.autoConnectError}</div>}
        <PreferenceSwitch id="close-to-tray" title={tr('Сворачивать в трей при закрытии', 'Minimize to tray on close')}
          hint={tr('Закрытие окна не прерывает соединение. Выход — через меню трея.', 'Closing the window keeps the connection active. Quit using the tray menu.')}
          checked={settings.closeToTray} disabled={busy||(!settings.trayAvailable&&!settings.closeToTray)} onChange={value=>run(()=>desktop().SetCloseToTray(value))} />
        {!settings.trayAvailable&&<div className="notice info">
          {tr('Системный трей недоступен. Пока трей недоступен, закрытие окна завершает приложение и VPN.', 'System tray unavailable. Without a tray, closing the window exits the app and disconnects the VPN.')}
          {settings.trayError&&<details><summary>{tr('Подробности', 'Details')}</summary><p className="mono">{settings.trayError}</p></details>}
        </div>}
        <button className="btn" disabled={busy||!settings.trayAvailable} onClick={()=>run(()=>desktop().MinimiseToTray())}>
          <Icon name="tray" size={17} /> {tr('Свернуть в трей', 'Minimize to tray')}
        </button>
      </div>
      <div className="settings-card">
        <h2>{tr('Диагностика и восстановление', 'Diagnostics and recovery')}</h2>
        {recovery && <p role="status">{recovery === 'restored' ? tr('Профили восстановлены из резервной копии.', 'Profiles restored from backup.') : recovery === 'recovered' ? tr('Профили восстановлены из резервной копии. Повреждённый файл сохранён отдельно.', 'Profiles recovered from backup. The damaged file was preserved separately.') : tr('Файл профилей повреждён; исходные данные сохранены. Импортируйте профили или восстановите резервную копию.', 'The profile file is damaged; original data was preserved. Import profiles or restore a backup.')}</p>}
        <p className="settings-note">{tr('Отчёт не содержит ключей, адресов серверов, ссылок captcha или исходного журнала. Проверка реального VPN не выполняется.', 'The report excludes keys, server addresses, captcha URLs and raw logs. It does not perform a live VPN test.')}</p>
        <div className="inline-actions">
          <button className="btn" disabled={busy} onClick={()=>run(()=>desktop().ExportDiagnostics())}>{tr('Сохранить диагностику', 'Save diagnostics')}</button>
          <button className="btn" disabled={busy} onClick={()=>{
            if (!window.confirm(tr('Заменить список профилей последней резервной копией? Сначала отключите соединение.', 'Replace profiles with the last backup? Disconnect first.'))) return
            run(async()=>{ await desktop().RestoreProfilesBackup(); await onProfilesRestored(); setRecovery(await desktop().ProfileRecoveryNotice()) })
          }}>{tr('Восстановить профили', 'Restore profiles')}</button>
        </div>
      </div>
      <UpdatePreferences active={active} autoCheck={settings.autoCheckUpdates} disabled={busy} canInstall={settings.canInstallUpdates}
        onAutoCheck={enabled=>run(()=>desktop().SetAutoCheckUpdates(enabled))} />
    </>}
    <footer className="settings-about">
      <img className="app-icon" src="/freeturn.svg" alt="" width="44" height="44" />
      <div><strong>{tr('FreeTurn Desktop', 'FreeTurn Desktop')}</strong><p>Core {version||'…'} · Wails + React</p></div>
      <button className="btn danger" onClick={()=>run(()=>desktop().Quit())}>{tr('Выйти из приложения', 'Quit application')}</button>
    </footer>
  </section>
}
