import {useCallback, useEffect, useMemo, useRef, useState} from 'react'
import {backend, CopyText, serversetup, ServerPeerAdd, ServerPeerRemove, ServerPeers, ServerPeerShare} from '../api'
import {exactTime, formatBytes, formatLastSeen, presence, Presence} from '../format'
import {tr} from '../language'
import {desktop} from '../desktop'
import ConfirmDialog from './ConfirmDialog'

const REFRESH_MS = 15000
const PRESENCE_ORDER: Record<Presence, number> = {online: 0, recent: 1, idle: 2, never: 3}

interface Props {
  server: backend.Server
}

interface ShareState {
  share: backend.GuestShare
  peerName: string
}

/** WireGuard users of a server deployed by this app: create, share (link / QR), delete, see who is online. */
export default function PeersPanel({server}: Props) {
  const [list, setList] = useState<serversetup.PeerList | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [newName, setNewName] = useState('')
  const [peerToDelete, setPeerToDelete] = useState<serversetup.Peer | null>(null)
  const [shareState, setShareState] = useState<ShareState | null>(null)
  const generation = useRef(0)
  const inflight = useRef<{generation: number} | null>(null)

  const refresh = useCallback(async () => {
    if (inflight.current?.generation === generation.current) return
    const request = {generation: generation.current}
    inflight.current = request
    try {
      const result = await ServerPeers(server.id)
      if (request.generation !== generation.current) return
      setList(result)
      setError('')
    } catch (e) {
      if (request.generation !== generation.current) return
      // Keep showing the last good list: one failed poll shouldn't blank the panel.
      setError(String(e))
    } finally {
      if (inflight.current === request) inflight.current = null
    }
  }, [server.id])

  useEffect(() => {
    setList(null)
    setPeerToDelete(null)
    setShareState(null)
    setError('')
    refresh()
    // Poll for "who is online" - but only while the window is actually visible,
    // so a forgotten open window doesn't keep SSH-ing into the server all night.
    const timer = setInterval(() => {
      if (document.visibilityState === 'visible') refresh()
    }, REFRESH_MS)
    return () => {
      clearInterval(timer)
      generation.current++
      inflight.current = null
    }
  }, [refresh])

  const rows = useMemo(() => {
    if (!list) return []
    const now = list.serverNow
    return [...list.peers].sort((a, b) => {
      if (a.isOwner !== b.isOwner) return a.isOwner ? -1 : 1
      const pa = PRESENCE_ORDER[presence(a.lastHandshake, now)]
      const pb = PRESENCE_ORDER[presence(b.lastHandshake, now)]
      if (pa !== pb) return pa - pb
      if (a.lastHandshake !== b.lastHandshake) return b.lastHandshake - a.lastHandshake
      return (a.name || '').localeCompare(b.name || '')
    })
  }, [list])

  const guarded = async (fn: () => Promise<void>) => {
    setBusy(true)
    setError('')
    try {
      await fn()
    } catch (e) {
      setError(String(e))
    } finally {
      setBusy(false)
    }
  }

  const addUser = () =>
    guarded(async () => {
      const share = await ServerPeerAdd(server.id, newName)
      setNewName('')
      setShareState({share, peerName: share.name})
      await refresh()
    })

  const openShare = (p: serversetup.Peer) =>
    guarded(async () => {
      const share = await ServerPeerShare(server.id, p.pub, p.name, false)
      setShareState({share, peerName: p.name})
    })

  const removeUser = (p: serversetup.Peer) =>
    guarded(async () => {
      if (p.isOwner) return
      await ServerPeerRemove(server.id, p.pub)
      await refresh()
    })

  const now = list?.serverNow ?? 0

  return (
    <div className="field-group">
      <div className="panel-head">
        <h3>{tr('Пользователи WireGuard', 'WireGuard users')}</h3>
        <button className="btn small ghost" disabled={busy} onClick={() => refresh()}>
          {tr('Обновить', 'Refresh')}
        </button>
      </div>
      {error && <div className="notice error">{error}</div>}

      <div className="peer-list">
        {list === null && !error && <div className="peer-empty">{tr('Загрузка…', 'Loading…')}</div>}
        {list !== null && rows.length === 0 && <div className="peer-empty">{tr('Пользователей пока нет.', 'No users yet.')}</div>}
        {rows.map((p) => {
          const pr = presence(p.lastHandshake, now)
          return (
            <div className="peer-row" key={p.pub}>
              <span className={'dot' + (pr === 'online' ? ' connected' : pr === 'recent' ? ' captcha' : '')} />
              <div className="peer-main">
                <div className="peer-name">{p.isOwner ? tr('Вы (владелец)', 'You (owner)') : p.name || tr('Без имени', 'Unnamed')}</div>
                <div className="peer-sub mono">{p.ip}</div>
              </div>
              <div className="peer-activity">
                <div className={'peer-seen ' + pr} title={exactTime(p.lastHandshake)}>
                  {formatLastSeen(p.lastHandshake, now)}
                </div>
                {(p.rx > 0 || p.tx > 0) && (
                  <div className="peer-sub" title={tr('Скачано / отдано пользователем с момента запуска WireGuard на сервере', 'User download / upload since WireGuard started on the server')}>
                    ↓ {formatBytes(p.tx)} ↑ {formatBytes(p.rx)}
                  </div>
                )}
              </div>
              <div className="peer-actions">
                {p.hasConf && (
                  <button className="btn small" disabled={busy} onClick={() => openShare(p)}>
                    {tr('Поделиться', 'Share')}
                  </button>
                )}
                {!p.isOwner && (
                  <button className="btn small danger" disabled={busy} onClick={() => setPeerToDelete(p)}>
                    {tr('Удалить', 'Delete')}
                  </button>
                )}
              </div>
            </div>
          )
        })}
      </div>

      <div className="peer-add">
        <input
          type="text"
          value={newName}
          maxLength={40}
          placeholder={tr('Имя нового пользователя', 'New user name')}
          disabled={busy}
          onChange={(e) => setNewName(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && newName.trim() && !busy && addUser()}
        />
        <button className="btn primary" disabled={busy || !newName.trim()} onClick={addUser}>
          {busy ? tr('Подождите…', 'Please wait…') : tr('Создать пользователя', 'Create user')}
        </button>
      </div>

      {peerToDelete && <ConfirmDialog request={{
        message: tr('Удалить пользователя «{name}»? Его подключение перестанет работать сразу.', {name: peerToDelete.name || peerToDelete.ip}),
        label: tr('Удалить', 'Delete'), danger: true,
      }} onAnswer={accepted => {
        const peer = peerToDelete
        setPeerToDelete(null)
        if (accepted) void removeUser(peer)
      }} />}
      {shareState && (
        <ShareModal
          share={shareState.share}
          peerName={shareState.peerName}
          defaultVKLink={server.vkLinks[0] || ''}
          onClose={() => setShareState(null)}
        />
      )}
    </div>
  )
}

interface ShareModalProps {
  share: backend.GuestShare
  peerName: string
  defaultVKLink: string
  onClose: () => void
}

export function ShareModal({share, peerName, defaultVKLink, onClose}: ShareModalProps) {
  const [copied, setCopied] = useState(false)
  const [includeVK, setIncludeVK] = useState(share.includesVk)
  const [vkLink, setVKLink] = useState(defaultVKLink)
  const [generated, setGenerated] = useState({share, vkLink: share.includesVk ? defaultVKLink.trim() : ''})
  const [error, setError] = useState('')
  const requestedVKLink = includeVK ? vkLink.trim() : ''
  const ready = (!includeVK || !!requestedVKLink) && generated.vkLink === requestedVKLink

  useEffect(() => {
    setCopied(false)
    setError('')
    if (ready || (includeVK && !requestedVKLink)) return
    let active = true
    const timer = window.setTimeout(() => {
      desktop().UpdateGuestShareVKLink(share.link, requestedVKLink).then(next => {
        if (active) setGenerated({share: next, vkLink: requestedVKLink})
      }).catch(e => { if (active) setError(String(e)) })
    }, 300)
    return () => { active = false; window.clearTimeout(timer) }
  }, [share.link, includeVK, requestedVKLink, ready])

  const copy = async () => {
    if (!ready) return
    try {
      await CopyText(generated.share.link)
    } catch {
      await navigator.clipboard?.writeText(generated.share.link)
    }
    setCopied(true)
    setTimeout(() => setCopied(false), 1800)
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h2>{tr('Подключение для «{name}»', {name: peerName || share.ip})}</h2>
        <label className="checkbox-row">
          <input type="checkbox" checked={includeVK} onChange={e => setIncludeVK(e.target.checked)} />
          {tr('Включить ссылку на звонок VK', 'Include a VK call link')}
        </label>
        {includeVK && <div className="field">
          <label htmlFor="share-vk-link">{tr('Ссылка на звонок VK Calls', 'VK Calls link')}</label>
          <input id="share-vk-link" type="text" value={vkLink} onChange={e => setVKLink(e.target.value)} placeholder="https://vk.ru/call/join/..." />
        </div>}
        {error && <div className="notice error" role="alert">{error}</div>}
        {!ready && !error && <div className="notice info" role="status">{includeVK && !requestedVKLink ? tr('Укажите ссылку на звонок VK.', 'Enter a VK call link.') : tr('Подождите…', 'Please wait…')}</div>}
        {ready && (generated.share.qr ? (
          <div className="qr-box">
            <img src={generated.share.qr} alt={tr('QR-код подключения', 'Connection QR code')} />
          </div>
        ) : (
          <div className="notice info">{tr('Ссылка слишком длинная для QR-кода — отправьте её текстом.', 'The link is too long for a QR code — send it as text.')}</div>
        ))}
        <div className="field">
          <label>{tr('Ссылка freeturn://', 'freeturn:// link')}</label>
          <textarea readOnly value={ready ? generated.share.link : ''} onClick={(e) => (e.target as HTMLTextAreaElement).select()} />
        </div>
        <div className="notice info">
          {tr('Отправьте ссылку или покажите QR: пользователь импортирует её в приложении, и сервер добавится сразу с рабочим WireGuard.', 'Send the link or show the QR code: importing it into the app adds the server with a working WireGuard configuration.')}
          {!includeVK && tr(' Свою ссылку на звонок VK он вводит сам.', ' The user will enter their own VK call link.')}
        </div>
        <div className="modal-actions">
          <button className="btn" onClick={onClose}>
            {tr('Закрыть', 'Close')}
          </button>
          <button className="btn primary" disabled={!ready} onClick={copy}>
            {copied ? tr('Скопировано ✓', 'Copied ✓') : tr('Скопировать ссылку', 'Copy link')}
          </button>
        </div>
      </div>
    </div>
  )
}
