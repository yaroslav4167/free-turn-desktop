import {useEffect, useState} from 'react'
import {tr} from '../language'
import ServerForm from './ServerForm'
import {
  backend,
  EventsOn,
  GenerateClientID,
  GenerateObfKey,
  OBF_PROFILES,
  serversetup,
  SetupDetectRootMode,
  SetupProbe,
  SetupPrepareContainer,
  ContainerPreview,
  SetupCreateContainer,
  ContainerInstance,
  NewServer,
  SaveServer,
} from '../api'

interface Props {
  onCreated: (s: backend.Server) => void
}

type Step = 'ssh' | 'existing' | 'review' | 'config' | 'progress' | 'done'
type TaskState = 'pending' | 'running' | 'done' | 'error'


function randomPort(min: number, max: number): number {
  return Math.floor(min + Math.random() * (max - min))
}

function newSshDraft(): serversetup.SSHConfig {
  return {
    ip: '', port: 22, username: 'root', password: '', authType: 'PASSWORD', sshKey: '',
    hostFingerprint: '', rootMode: 'ROOT', sudoPassword: '',
  }
}

export default function CreateServerWizard({onCreated}: Props) {
  const TASK_LABEL: Record<string, string> = {
    install: tr('Установка free-turn-proxy', 'Installing free-turn-proxy'),
    wgSetup: tr('Настройка WireGuard', 'Configuring WireGuard'),
    start: tr('Запуск сервера', 'Starting server'), save: tr('Сохранение профиля', 'Saving profile'),
  }
  const [step, setStep] = useState<Step>('ssh')
  const [ssh, setSsh] = useState<serversetup.SSHConfig>(newSshDraft())
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [existing, setExisting] = useState<serversetup.ProbeResult | null>(null)
  const [existingDraft, setExistingDraft] = useState<backend.Server | null>(null)
	const [containerPreview, setContainerPreview] = useState<ContainerPreview | null>(null)
	const [containers, setContainers] = useState<ContainerInstance[]>([])
	const [containerId, setContainerId] = useState('')
	const [deviceName, setDeviceName] = useState('')

  const [name, setName] = useState('')
  const [vkLink, setVkLink] = useState('')
  const [obfProfile, setObfProfile] = useState('rtpopus3')
  const [obfKey, setObfKey] = useState('')
  const [listenPort, setListenPort] = useState(() => randomPort(56000, 57000))
  const [wgPort, setWgPort] = useState(() => randomPort(51000, 52000))
  const [wgPortExisting, setWgPortExisting] = useState(false)
  const [clientId, setClientId] = useState('')

  const [tasks, setTasks] = useState<Record<string, {state: TaskState; msg?: string}>>({})
  const [result, setResult] = useState<{server: backend.Server; wgConf: string} | null>(null)
  // What the server-side script is doing right now (apt busy, downloading, ...):
  // without it a slow step looks like a hang.
  const [progressLines, setProgressLines] = useState<string[]>([])

  useEffect(() => {
    if (step !== 'progress') return
    return EventsOn('setup:progress', (line: string) =>
      setProgressLines((prev) => [...prev.slice(-40), line])
    )
  }, [step])

  const submitSsh = async () => {
    setBusy(true)
    setError('')
    try {
      const withRoot = await SetupDetectRootMode(ssh)
      const probe = await SetupProbe(withRoot)
      const pinned: serversetup.SSHConfig = {...withRoot, hostFingerprint: probe.hostFingerprint || withRoot.hostFingerprint}
      setSsh(pinned)
      setExisting(probe)
		const found: ContainerInstance[] = (probe as typeof probe & {containers?: ContainerInstance[]}).containers || []
		setContainers(found)
		setContainerId(found.length === 1 && found[0].supported && found[0].running ? found[0].id : '')
		if (!deviceName) setDeviceName('desktop-' + (await GenerateClientID()).replace(/[^a-zA-Z0-9]/g, '').slice(0, 16))
      setWgPortExisting(probe.wgPort > 0)
      if (probe.installed || probe.running) {
        const draft = await NewServer()
        draft.name = ssh.ip
        // Do not attach SSH management to an unknown installation layout.
        setExistingDraft(draft)
        setStep('existing')
        return
      }
      if (probe.wgPort > 0) {
        setWgPort(probe.wgPort)
        setWgPortExisting(true)
      }
      const [key, cid] = await Promise.all([GenerateObfKey(), GenerateClientID()])
      setObfKey(key)
      setClientId(cid)
      setStep('config')
    } catch (e) {
      setError(String(e))
    } finally {
      setBusy(false)
    }
  }

  const runProvision = async () => {
    if (existing?.installed || existing?.running) return
    setStep('progress')
    setError('')
    setProgressLines([])
    const localTasks: Record<string, {state: TaskState; msg?: string}> = {
      install: {state: 'pending'}, wgSetup: {state: 'pending'}, start: {state: 'pending'}, save: {state: 'pending'},
    }
    setTasks({...localTasks})
    const mark = (key: string, state: TaskState, msg?: string) => {
      localTasks[key] = {state, msg}
      setTasks({...localTasks})
    }

    try {
      mark('install', 'running')
      // Recheck just before the first remote mutation (also on retries).
      const fresh = await SetupProbe(ssh)
      if (fresh.installed || fresh.running) {
        setExisting(fresh)
			setContainers((fresh as typeof fresh & {containers?: ContainerInstance[]}).containers || [])
        setExistingDraft(await NewServer())
        setStep('existing')
        return
      }
      const draft: backend.ServerSetupDraft = {
        name: name.trim() || ssh.ip, vkLink: vkLink.trim(), obfProfile, obfKey, obfTimingMs: 0,
        listenPort, clientId,
      }
      const saved = await SetupCreateContainer(ssh, draft, wgPort, deviceName)
      mark('install', 'done')
      mark('wgSetup', 'done')
      mark('start', 'done')
      mark('save', 'done')

      setResult({server: saved, wgConf: saved.wgClientConf || ''})
      setStep('done')
    } catch (e) {
      const failed = Object.entries(localTasks).find(([, v]) => v.state === 'running')?.[0]
      if (failed) mark(failed, 'error', String(e))
      setError(String(e))
    }
  }

  if (step === 'ssh') {
    return (
      <>
        {error && <div className="notice error">{error}</div>}
        <div className="field-row">
          <div className="field" style={{flex: 2}}>
            <label>{tr('SSH-хост (домен или IP)', 'SSH host (domain or IP)')}</label>
            <input type="text" value={ssh.ip} onChange={(e) => setSsh({...ssh, ip: e.target.value.trim()})} placeholder="vpn.example.com" />
          </div>
          <div className="field">
            <label>{tr('SSH порт', 'SSH port')}</label>
            <input type="number" value={ssh.port} onChange={(e) => setSsh({...ssh, port: Number(e.target.value)})} />
          </div>
        </div>
        <p className="settings-note">{tr('Для сервера с меняющимся IP используйте домен и поддерживайте его DNS-запись актуальной.', 'For a server with a changing IP, use a domain and keep its DNS record up to date.')}</p>
        <div className="field-row">
          <div className="field">
            <label>{tr('Пользователь', 'Username')}</label>
            <input type="text" value={ssh.username} onChange={(e) => setSsh({...ssh, username: e.target.value})} />
          </div>
          <div className="field">
            <label>{tr('Способ входа', 'Authentication method')}</label>
            <select value={ssh.authType} onChange={(e) => setSsh({...ssh, authType: e.target.value})}>
              <option value="PASSWORD">{tr('Пароль', 'Password')}</option>
              <option value="SSH_KEY">{tr('SSH-ключ', 'SSH key')}</option>
            </select>
          </div>
        </div>
        {ssh.authType === 'SSH_KEY' ? (
          <>
            <div className="field">
              <label>{tr('Приватный ключ (OpenSSH)', 'Private key (OpenSSH)')}</label>
              <textarea value={ssh.sshKey} onChange={(e) => setSsh({...ssh, sshKey: e.target.value})} placeholder="-----BEGIN OPENSSH PRIVATE KEY-----" />
            </div>
            <div className="field">
              <label>{tr('Passphrase ключа (если есть)', 'Key passphrase (if any)')}</label>
              <input type="password" value={ssh.password} onChange={(e) => setSsh({...ssh, password: e.target.value})} />
            </div>
          </>
        ) : (
          <div className="field">
            <label>{tr('Пароль', 'Password')}</label>
            <input type="password" value={ssh.password} onChange={(e) => setSsh({...ssh, password: e.target.value})} />
          </div>
        )}
        <div className="field">
          <label>{tr('Пароль sudo (если отличается от пароля входа)', 'Sudo password (if different from login password)')}</label>
          <input type="password" value={ssh.sudoPassword} onChange={(e) => setSsh({...ssh, sudoPassword: e.target.value})} />
        </div>
        <div className="modal-actions">
          <button
            className="btn primary"
            disabled={busy || !ssh.ip || (ssh.authType === 'SSH_KEY' ? !ssh.sshKey : !ssh.password)}
            onClick={submitSsh}
          >
            {busy ? tr('Подключение…', 'Connecting…') : tr('Далее', 'Next')}
          </button>
        </div>
      </>
    )
  }

  if (step === 'review' && containerPreview && existingDraft) {
    return <>
      <div className="notice info">{tr('Проверьте адрес реле перед сохранением профиля. Адрес из конфигурации сервера может быть устаревшим.', 'Review the relay address before saving the profile. The server configuration may contain an outdated address.')}</div>
      {containerPreview.needsReview && <div className="notice warning">
        {tr('Адрес в ссылке отличается от адреса SSH. Это может быть намеренно (NAT или отдельный публичный адрес); выберите нужный вариант.', 'The link address differs from the SSH address. This can be intentional (NAT or a separate public address); choose the appropriate endpoint.')}
      </div>}
      {error && <div className="notice error">{error}</div>}
      <div className="field">
        <label>{tr('Адрес сервера (peer), host:port', 'Server address (peer), host:port')}</label>
        <input value={existingDraft.peer} onChange={e => setExistingDraft({...existingDraft, peer: e.target.value} as backend.Server)} />
      </div>
      <div className="modal-actions">
        <button className="btn" disabled={busy} onClick={() => setExistingDraft({...existingDraft, peer: containerPreview.advertisedPeer} as backend.Server)}>
          {tr('Адрес из ссылки', 'Link address')}: {containerPreview.advertisedPeer}
        </button>
        <button className="btn" disabled={busy} onClick={() => setExistingDraft({...existingDraft, peer: containerPreview.sshPeer} as backend.Server)}>
          {tr('Адрес SSH-сервера', 'SSH server address')}: {containerPreview.sshPeer}
        </button>
      </div>
      <p>{tr('Меняется только локальный профиль. Параметры контейнера и ключи не изменяются.', 'Only the local profile is changed. Container settings and keys are not modified.')}</p>
      <div className="modal-actions">
        <button className="btn" disabled={busy} onClick={() => setStep('existing')}>{tr('Назад', 'Back')}</button>
        <button className="btn primary" disabled={busy || !existingDraft.peer} onClick={async () => {
          setBusy(true); setError('')
          try { onCreated(await SaveServer(existingDraft)) }
          catch (e) { setError(String(e)) }
          finally { setBusy(false) }
        }}>{tr('Сохранить', 'Save')}</button>
      </div>
    </>
  }

  if (step === 'existing') {
    return <>
		{containers.length > 0 && <>
			<div className="notice info">{tr('Найден существующий контейнер FreeTurn. Добавим отдельный профиль устройства без переустановки, обновления и перезапуска сервера.', 'Existing FreeTurn container found. Add a separate device profile without reinstalling, updating or restarting the server.')}</div>
			<div className="field"><label>{tr('Контейнер', 'Container')}</label>
				<select value={containerId} onChange={e => setContainerId(e.target.value)}>
					<option value="">{tr('Выберите экземпляр', 'Select an instance')}</option>
					{containers.map(c => <option key={c.id} value={c.id} disabled={!c.supported || !c.running}>{c.name} · {c.image} · {c.running ? tr('Запущен', 'Running') : tr('Остановлен', 'Stopped')}{!c.supported ? ' · ' + tr('Только ручной импорт', 'Manual import only') : ''}</option>)}
				</select>
			</div>
			<div className="field"><label>{tr('Имя устройства (desktop- и латиница, цифры, . _ -)', 'Device name (desktop- followed by letters, digits, . _ -)')}</label><input value={deviceName} maxLength={32} onChange={e => setDeviceName(e.target.value)} /></div>
			<button className="btn primary" disabled={busy || !containerId || !/^desktop-[a-zA-Z0-9._-]{1,24}$/.test(deviceName)} onClick={async () => {
				setBusy(true); setError('')
				try {
          const preview = await SetupPrepareContainer(ssh, containerId, deviceName)
          if (!preview.needsReview) {
            onCreated(await SaveServer(preview.server))
            return
          }
          setContainerPreview(preview)
          // No endpoint is selected silently: an imported public host can be
          // stale, while SSH can legitimately use a separate bastion/NAT host.
          setExistingDraft({...preview.server, peer: ''} as backend.Server)
          setStep('review')
        }
				catch (e) { setError(String(e)) }
				finally { setBusy(false) }
			}}>{busy ? tr('Подключение…', 'Connecting…') : tr('Использовать контейнер', 'Use container')}</button>
		</>}
      <details open={containers.length === 0}>
      <summary>{tr('Ручное подключение', 'Manual connection')}</summary>
      <div className="notice info">
        {tr('Обнаружен установленный сервер. Повторная установка и перенастройка отключены.', 'An existing server was detected. Reinstallation and reconfiguration are disabled.')}
        {' '}{existing?.existingService} {existing?.version}
        {' · '}{existing?.running ? tr('Запущен', 'Running') : tr('Остановлен', 'Stopped')}
      </div>
      <p>{tr('Укажите параметры существующего подключения и его клиентский конфиг. Сохранение добавит только локальный профиль: настройки сервера и список клиентов не изменятся.', 'Enter the existing connection settings and its client configuration. Saving only adds a local profile: server settings and authorized clients will not change.')}</p>
      {error && <div className="notice error">{error}</div>}
      {existingDraft && <ServerForm value={existingDraft} onChange={setExistingDraft} />}
      {existingDraft && <div className="field">
        <label>{tr('Клиентский конфиг WireGuard/AmneziaWG (для режима VPN)', 'WireGuard/AmneziaWG client configuration (for VPN mode)')}</label>
        <textarea value={existingDraft.wgClientConf || ''} onChange={e => setExistingDraft({
          ...existingDraft, wgClientConf: e.target.value, connMode: e.target.value.trim() ? 'vpn' : 'relay',
        } as backend.Server)} placeholder="[Interface]" />
      </div>}
      <div className="modal-actions">
        <button className="btn" disabled={busy} onClick={() => setStep('ssh')}>{tr('Назад', 'Back')}</button>
        <button className="btn primary" disabled={busy || !existingDraft?.peer} onClick={async () => {
          if (!existingDraft) return
          setBusy(true)
          setError('')
          try { onCreated(await SaveServer(existingDraft)) }
          catch (e) { setError(String(e)) }
          finally { setBusy(false) }
        }}>{tr('Сохранить', 'Save')}</button>
      </div>
      </details>
    </>
  }

  if (step === 'config') {
    return (
      <>
        {error && <div className="notice error">{error}</div>}
        <div className="field">
          <label>{tr('Имя сервера', 'Server name')}</label>
          <input type="text" value={name} onChange={(e) => setName(e.target.value)} placeholder={ssh.ip} />
        </div>
        <div className="field">
          <label>{tr('Ваша ссылка на звонок VK Calls (можно добавить позже)', 'Your VK Calls link (can be added later)')}</label>
          <input type="text" value={vkLink} onChange={(e) => setVkLink(e.target.value)} placeholder="https://vk.ru/call/join/..." />
        </div>
        <div className="field-row">
          <div className="field">
            <label>{tr('Профиль обфускации', 'Obfuscation profile')}</label>
            <select value={obfProfile} onChange={(e) => setObfProfile(e.target.value)}>
              {OBF_PROFILES.filter((p) => p !== 'none').map((p) => (
                <option key={p} value={p}>
                  {p}
                </option>
              ))}
            </select>
          </div>
          <div className="field">
            <label>{tr('Внешний порт (-listen)', 'External port (-listen)')}</label>
            <input type="number" value={listenPort} onChange={(e) => setListenPort(Number(e.target.value))} />
          </div>
          <div className="field">
            <label>{tr('Порт WireGuard', 'WireGuard port')}{wgPortExisting ? tr(' (уже настроен)', ' (already configured)') : ''}</label>
            <input type="number" value={wgPort} disabled={wgPortExisting} onChange={(e) => setWgPort(Number(e.target.value))} />
          </div>
        </div>
        <div className="notice info">
          {tr('Развернём free-turn-proxy и интерфейс WireGuard (ft-wg0) на сервере, запустим их с обфускацией и добавим вас первым разрешённым клиентом.', 'We will deploy free-turn-proxy and the WireGuard interface (ft-wg0), start them with obfuscation and add you as the first authorized client.')}
        </div>
        <div className="modal-actions">
          <button className="btn" onClick={() => setStep('ssh')}>
            {tr('Назад', 'Back')}
          </button>
          <button className="btn primary" onClick={runProvision}>
            {tr('Развернуть', 'Deploy')}
          </button>
        </div>
      </>
    )
  }

  if (step === 'progress') {
    return (
      <>
        {Object.entries(tasks).map(([key, t]) => (
          <div key={key} className="checkbox-row">
            <span
              className={'dot' + (t.state === 'done' ? ' connected' : t.state === 'error' ? ' error' : t.state === 'running' ? ' connecting' : '')}
            />
            {TASK_LABEL[key]}
            {t.state === 'error' && t.msg && <span className="task-error">{t.msg}</span>}
          </div>
        ))}
        {progressLines.length > 0 && (
          <pre
            className="setup-log"
            ref={(el) => {
              if (el) el.scrollTop = el.scrollHeight
            }}
          >
            {progressLines.join('\n')}
          </pre>
        )}
        {error && (
          <div className="modal-actions">
            <button className="btn" onClick={() => setStep('config')}>
              {tr('Назад', 'Back')}
            </button>
            <button className="btn primary" onClick={runProvision}>
              {tr('Повторить', 'Retry')}
            </button>
          </div>
        )}
      </>
    )
  }

  // done
  return (
    <>
      <div className="notice info">{tr('Сервер развёрнут и запущен.', 'Server deployed and running.')}</div>
      {result && (
        <>
          <div className="field">
            <label>{tr('Конфиг WireGuard/AmneziaWG для отдельного клиента (Endpoint уже указывает на локальный релей)', 'WireGuard/AmneziaWG configuration for an external client (Endpoint already points to the local relay)')}</label>
            <textarea readOnly value={result.wgConf} onClick={(e) => (e.target as HTMLTextAreaElement).select()} />
          </div>
        </>
      )}
      <div className="modal-actions">
        <button className="btn primary" onClick={() => result && onCreated(result.server)}>
          {tr('Готово', 'Done')}
        </button>
      </div>
    </>
  )
}
