import {backend} from '../api'
import ServerForm from './ServerForm'
import ServerManagement from './ServerManagement'
import PeersPanel from './PeersPanel'
import LogConsole, {LogEntry} from './LogConsole'
import VPNRoutingLists from './VPNRoutingLists'
import {tr} from '../language'
import {serverDetailState} from '../serverDetailState'
import ConnectionLoader from './ConnectionLoader'
import InfoTip from './InfoTip'
import ProfileCommand from './ProfileCommand'
import {bypassLocalNetworks} from '../localNetworkRouting'


interface Props {
  server: backend.Server
  dirty: boolean
  status: backend.Status
  isActive: boolean
  pendingServerId: string | null
  logs: LogEntry[]
  onChange: (s: backend.Server) => void
  onSave: () => void
  onDelete: () => void
  onConnect: () => void
  onDisconnect: () => void
  onClearLogs: () => void
}

export default function ServerDetail({
  server,
  dirty,
  status,
  isActive,
  pendingServerId,
  logs,
  onChange,
  onSave,
  onDelete,
  onConnect,
  onDisconnect,
  onClearLogs,
}: Props) {
  const STATE_LABEL: Record<string, string> = {
    idle: tr('Отключено', 'Disconnected'), connecting: tr('Подключение…', 'Connecting…'),
    connected: tr('Подключено', 'Connected'), captcha: tr('Требуется captcha', 'Captcha required'), error: tr('Ошибка', 'Error'),
  }

  const presentation = serverDetailState(server.id, status, pendingServerId)
  const state = presentation.phase
  const busy = presentation.busy
  const otherActive = !isActive && status.state !== 'idle' && status.state !== 'error'

  const isVPN = server.connMode === 'vpn'

  return (
    <div className="detail">
      <div className="detail-header">
        <h2>{server.name || server.peer || tr('Новый сервер', 'New server')}</h2>
        <span className={'status-pill ' + state} role="status" aria-live="polite">{STATE_LABEL[state] ?? state}</span>
        {presentation.connected && (
          <button className="btn danger" onClick={onDisconnect}>
            {tr('Отключить', 'Disconnect')}
          </button>
        )}
        {!presentation.connected && (
          <button className="btn primary" disabled={presentation.progressing || presentation.otherPreparing || !server.peer} aria-busy={busy} onClick={onConnect}>
            {busy && <span className="compact-loader"><ConnectionLoader active={true} /></span>}
            {presentation.progressing ? state === 'captcha' ? tr('Ожидание проверки', 'Waiting for verification') : state === 'error' ? tr('Ошибка подключения', 'Connection error') : tr('Подключение…', 'Connecting…') : otherActive ? tr('Переключиться', 'Switch') : isVPN ? tr('Включить VPN', 'Enable VPN') : tr('Подключиться', 'Connect')}
          </button>
        )}
        {presentation.progressing && <button className="btn" onClick={onDisconnect}>{tr('Отменить', 'Cancel')}</button>}
        <button className="btn" disabled={!dirty || pendingServerId !== null} onClick={onSave}>
          {tr('Сохранить', 'Save')}
        </button>
        <button className="btn danger" disabled={pendingServerId !== null || (isActive && !['idle','error'].includes(state))} title={isActive && !['idle','error'].includes(state) ? tr('Сначала отключите этот профиль', 'Disconnect this profile first') : tr('Удаляется только профиль в приложении', 'Only the local app profile is removed')} onClick={onDelete}>
          {tr('Удалить профиль', 'Delete profile')}
        </button>
      </div>

      <div className="detail-body">
        {presentation.otherPreparing && <div className="notice info settings-attention">
          <span>{tr('Подключается другой профиль. Дождитесь завершения или отмените подключение.', 'Another profile is connecting. Wait for completion or cancel the attempt.')}</span>
          <button className="btn small" onClick={onDisconnect}>{tr('Отменить', 'Cancel')}</button>
        </div>}
        <div className="field-group">
          <h3>{tr('Тип подключения', 'Connection type')} <InfoTip label={tr('О режиме подключения', 'About connection mode')}>{tr('Встроенный VPN сам создаёт сетевой интерфейс. Режим реле нужен для отдельного WireGuard/AmneziaWG-клиента.', 'Built-in VPN creates the network interface itself. Relay mode is for an external WireGuard/AmneziaWG client.')}</InfoTip></h3>
          {server.wgClientConf ? (
            <div className="inline-actions">
              <button
                className={'btn small' + (isVPN ? ' primary' : '')}
                disabled={presentation.progressing || (isActive && state === 'connected')}
                onClick={() => onChange({...server, connMode: 'vpn'} as backend.Server)}
              >
                {tr('VPN (одна кнопка)', 'VPN (one click)')}
              </button>
              <button
                className={'btn small' + (!isVPN ? ' primary' : '')}
                disabled={presentation.progressing || (isActive && state === 'connected')}
                onClick={() => onChange({...server, connMode: 'relay'} as backend.Server)}
              >
                {tr('Реле (свой WireGuard-клиент)', 'Relay (external WireGuard client)')}
              </button>
            </div>
          ) : (
            <div className="notice info">
              {tr('В профиле нет WireGuard-конфига. Для VPN одной кнопкой импортируйте полную ссылку с конфигурацией; сейчас доступен режим реле.', 'This profile has no WireGuard configuration. Import a complete link with configuration for one-click VPN; relay mode is currently available.')}
            </div>
          )}
        </div>

        {otherActive && (
          <div className="notice info">
            {tr('Сейчас подключён другой сервер — переключение сначала отключит его.', 'Another server is connected — switching will disconnect it first.')}
          </div>
        )}
        {isActive && state === 'error' && status.errMsg && <div className="notice error">{status.errMsg}</div>}
        {isActive && state === 'connected' && isVPN && (
          <div className="notice info">{tr('VPN включён через', 'VPN enabled via')} {server.peer}{server.routingLists?.some((list) => list.enabled) ? tr(' · применены списки маршрутизации', ' · routing lists applied') : ''}.</div>
        )}
        {isActive && state === 'connected' && !isVPN && (
          <div className="notice info">
            {tr('Реле поднято на', 'Relay listening on')} <span className="mono">{server.listen}</span>. {tr('Направьте на этот адрес Endpoint в вашем WireGuard/AmneziaWG-клиенте (релейный конфиг, MTU 1280) и включайте VPN.', 'Set this as the Endpoint in your WireGuard/AmneziaWG client (relay configuration, MTU 1280), then enable VPN.')}
          </div>
        )}

        <ServerForm value={server} onChange={onChange} />
        {isVPN && <details key={`routing-${server.id}`} className="advanced-settings">
          <summary><strong>{tr('Списки маршрутизации VPN', 'VPN routing lists')}</strong><span>{bypassLocalNetworks(server) ? tr('Локальная сеть — напрямую', 'Local networks bypass VPN')+' · ' : ''}{tr('Включено: {count}', {count:server.routingLists?.filter(list=>list.enabled).length||0})}</span></summary>
          <div className="advanced-content"><VPNRoutingLists server={server} onChange={onChange} disabled={presentation.progressing || (isActive && state !== 'idle' && state !== 'error')} /></div>
        </details>}

        {server.ssh && <details key={`management-${server.id}`} className="advanced-settings server-administration">
          <summary><strong>{tr('Управление VPS и пользователями', 'VPS and user management')}</strong><span>SSH</span></summary>
          <div className="advanced-content"><ServerManagement server={server} /><PeersPanel server={server} /></div>
        </details>}

        <ProfileCommand key={`${server.id}-${dirty?'draft':'saved'}`} serverId={server.id} />
      </div>

      <details className="profile-log-panel">
        <summary><strong>{tr('Журнал подключения', 'Connection log')}</strong><span>{tr('Записей: {count}', {count:logs.length})}</span></summary>
        <LogConsole lines={logs} onClear={onClearLogs} />
      </details>
    </div>
  )
}
