import {useEffect, useId, useRef, useState} from 'react'
import {backend, DNS_MODES, GenerateClientID, GenerateObfKey, MODES, OBF_PROFILES, TRANSPORTS} from '../api'
import {tr} from '../language'
import FieldLabel from './FieldLabel'
import InfoTip from './InfoTip'
import StreamControls from './StreamControls'

interface Props { value: backend.Server; onChange: (s: backend.Server) => void }

export default function ServerForm({value: s, onChange}: Props) {
  const prefix = useId()
  const id = (field: string) => `${prefix}-${field}`
  const advanced = useRef<HTMLDetailsElement>(null)
  const [showKey, setShowKey] = useState(false)
  const [showClientID, setShowClientID] = useState(false)
  useEffect(() => { setShowKey(false); setShowClientID(false); if (advanced.current) advanced.current.open = false }, [s.id])
  const patch = (p: Partial<backend.Server>) => onChange({...s, ...p} as backend.Server)
  const patchKcp = (p: Partial<backend.KCPProfile>) => patch({kcp: {...s.kcp, ...p} as backend.KCPProfile})
  const needsKey = !!s.obfProfile && s.obfProfile !== 'none' && !/^[0-9a-f]{64}$/i.test(s.obfKey || '')

  return <div className="profile-settings">
    <section className="field-group settings-section">
      <h3>{tr('Подключение', 'Connection')}</h3>
      <div className="field-row">
        <div className="field">
          <FieldLabel htmlFor={id('name')} label={tr('Имя', 'Name')} hint={tr('Название только для списка профилей. Оно не меняет настройки сервера.', 'A name for your profile list only. It does not change the server.')} />
          <input id={id('name')} type="text" value={s.name} onChange={e => patch({name:e.target.value})} placeholder={tr('Мой сервер', 'My server')} />
        </div>
        <div className="field">
          <FieldLabel htmlFor={id('peer')} label={tr('Адрес сервера', 'Server address')} hint={tr('Публичный адрес реле и его порт: домен:порт или IP:порт. Для сервера с меняющимся IP лучше использовать домен с актуальной DNS-записью.', 'Public relay address and port: domain:port or IP:port. A domain with an up-to-date DNS record is preferable when the IP can change.')} />
          <input id={id('peer')} type="text" value={s.peer} onChange={e => patch({peer:e.target.value})} placeholder="vpn.example.com:56952" />
        </div>
      </div>
      <div className="field">
        <FieldLabel htmlFor={id('links')} label={tr('Ссылки на звонки VK Calls', 'VK Calls links')} hint={tr('По одной ссылке в строке. Каждая ссылка получает заданное ниже число потоков; пустые строки не учитываются. Не публикуйте ссылки в открытых журналах.', 'One link per line. Each link receives the stream count configured below; blank lines do not count. Do not publish links in public logs.')} />
        <textarea id={id('links')} value={(s.vkLinks ?? []).join('\n')}
          onChange={(e) => patch({vkLinks: e.target.value.split('\n')})} placeholder="https://vk.ru/call/join/..." />
      </div>
    </section>

    <section className="field-group settings-section">
      <h3>{tr('Производительность', 'Performance')}</h3>
      <StreamControls key={s.id || 'new'} value={s} onChange={patch} />
    </section>

    {needsKey && <div className="notice info settings-attention">
      <span>{tr('Маскировка включена: проверьте ключ из профиля сервера.', 'Obfuscation is enabled: check the key from the server profile.')}</span>
      <button type="button" className="btn small" onClick={() => { if (advanced.current) advanced.current.open = true }}>
        {tr('Открыть параметры', 'Open parameters')}
      </button>
    </div>}

    <details ref={advanced} className="advanced-settings" onToggle={e => { if (!e.currentTarget.open) { setShowKey(false); setShowClientID(false) } }}>
      <summary><strong>{tr('Дополнительные параметры', 'Additional settings')}</strong><span>{tr('Маскировка, транспорт, DNS и диагностика', 'Obfuscation, transport, DNS and diagnostics')}</span></summary>
      <div className="advanced-content">
        <section className="field-group">
          <h3>{tr('Обфускация', 'Obfuscation')}</h3>
          <div className="field-row">
            <div className="field">
              <FieldLabel htmlFor={id('obf')} label={tr('Профиль', 'Profile')} hint={tr('Должен совпадать с профилем маскировки на сервере. После импорта ссылки обычно менять не нужно.', 'Must match server obfuscation. It normally needs no changes after importing a link.')} />
              <select id={id('obf')} value={s.obfProfile} onChange={e => patch({obfProfile:e.target.value})}>
                {OBF_PROFILES.map(p => <option key={p} value={p}>{p}</option>)}
              </select>
            </div>
            <div className="field" style={{flex:2}}>
              <FieldLabel htmlFor={id('key')} label={tr('Ключ маскировки', 'Obfuscation key')} hint={tr('64 шестнадцатеричных символа, как на сервере. Для существующего сервера используйте выданный ключ: случайная генерация сама не обновляет сервер.', '64 hexadecimal characters matching the server. Use the issued key for an existing server: generating a random key does not update the server.')} />
              <div className="inline-actions">
                <input id={id('key')} type={showKey?'text':'password'} className="mono" value={s.obfKey} onChange={e => patch({obfKey:e.target.value.trim()})} autoComplete="off" disabled={s.obfProfile==='none'} />
                <button type="button" className="btn small" disabled={s.obfProfile==='none'} aria-pressed={showKey} aria-label={showKey?tr('Скрыть ключ маскировки', 'Hide obfuscation key'):tr('Показать ключ маскировки', 'Show obfuscation key')} onClick={() => setShowKey(!showKey)}>{showKey?tr('Скрыть', 'Hide'):tr('Показать', 'Show')}</button>
                <button type="button" className="btn small" aria-label={tr('Сгенерировать ключ маскировки', 'Generate obfuscation key')} disabled={s.obfProfile==='none'} onClick={() => GenerateObfKey().then(key => patch({obfKey:key}))}>{tr('Сгенерировать', 'Generate')}</button>
              </div>
            </div>
          </div>
          <div className="field">
            <FieldLabel htmlFor={id('timing')} label={tr('Пейсинг обфускации, мс (0 = выкл)', 'Obfuscation pacing, ms (0 = off)')} hint={tr('Дополнительная задержка отправки пакетов маскировки. Увеличивает задержку и не гарантирует обход ограничений; 0 отключает её.', 'Additional obfuscated-packet send delay. It adds latency and does not guarantee bypassing restrictions; 0 disables it.')} />
            <input id={id('timing')} type="number" min={0} max={60} value={s.obfTimingMs} onChange={e => patch({obfTimingMs:Number(e.target.value)})} disabled={s.obfProfile==='none'} />
          </div>
        </section>

        <section className="field-group">
          <h3>{tr('Транспорт', 'Transport')}</h3>
          <div className="field-row">
            <div className="field">
              <FieldLabel htmlFor={id('transport')} label={tr('Транспорт до TURN', 'TURN transport')} hint={tr('Соединение клиента с TURN-сервером: TCP или UDP. Это не выбор транспорта WireGuard и не порт SSH.', 'Client-to-TURN connection: TCP or UDP. This does not select WireGuard transport or the SSH port.')} />
              <select id={id('transport')} value={s.transport} onChange={e => patch({transport:e.target.value})}>{TRANSPORTS.map(t => <option key={t} value={t}>{t}</option>)}</select>
            </div>
            <div className="field">
              <FieldLabel htmlFor={id('mode')} label={tr('Режим туннеля', 'Tunnel mode')} hint={tr('Должен совпадать с режимом реле на сервере. В режиме tcp используются дополнительные параметры KCP.', 'Must match the server relay mode. TCP mode uses the additional KCP settings.')} />
              <select id={id('mode')} value={s.mode} onChange={e => patch({mode:e.target.value})}>{MODES.map(m => <option key={m} value={m}>{m}</option>)}</select>
            </div>
            <div className="field">
              <FieldLabel htmlFor={id('listen')} label={tr('Локальный listen', 'Local listen address')} hint={tr('Локальный адрес реле для отдельного WireGuard-клиента. Встроенный VPN использует внутренний канал, а не этот UDP-порт.', 'Local relay address for an external WireGuard client. Built-in VPN uses an internal channel instead of this UDP port.')} />
              <input id={id('listen')} type="text" value={s.listen} onChange={e => patch({listen:e.target.value})} />
            </div>
          </div>
        </section>

        {s.mode==='tcp' && <section className="field-group">
          <h3>{tr('KCP (ARQ для tcp-режима)', 'KCP (ARQ for TCP mode)')}</h3>
          <div className="field-row kcp-grid">
            {([
              ['noDelay','nodelay',tr('Режим задержки KCP. Меняйте только при осознанной настройке транспорта.', 'KCP delay mode. Change only when deliberately tuning transport.')],
              ['interval','interval',tr('Интервал обновления KCP в миллисекундах.', 'KCP update interval in milliseconds.')],
              ['resend','resend',tr('Порог быстрого повторного отправления KCP.', 'KCP fast retransmit threshold.')],
              ['nc','nc',tr('Параметр контроля перегрузки KCP.', 'KCP congestion-control setting.')],
              ['sndWnd','sndwnd',tr('Размер окна отправки KCP в пакетах.', 'KCP send-window size in packets.')],
              ['rcvWnd','rcvwnd',tr('Размер окна приёма KCP в пакетах.', 'KCP receive-window size in packets.')],
              ['mtu','mtu',tr('Размер пакета KCP, не MTU интерфейса WireGuard.', 'KCP packet size, not the WireGuard interface MTU.')],
            ] as const).map(([key,label,hint]) => <div className="field" key={key}>
              <FieldLabel htmlFor={id(key)} label={label} hint={hint} />
              <input id={id(key)} type="number" value={s.kcp[key]} onChange={e => patchKcp({[key]:Number(e.target.value)})} />
            </div>)}
          </div>
          <div className="toggle-setting"><label className="checkbox-row"><input type="checkbox" checked={s.kcp.ackNoDelay} onChange={e => patchKcp({ackNoDelay:e.target.checked})} />ack no delay</label><InfoTip label="ack no delay">{tr('Ускоренная отправка подтверждений KCP.', 'Send KCP acknowledgements without delay.')}</InfoTip></div>
        </section>}

        <section className="field-group">
          <h3>DNS</h3>
          <div className="field-row">
            <div className="field">
              <FieldLabel htmlFor={id('dns-mode')} label={tr('Резолвер клиента', 'Client resolver')} hint={tr('DNS-режим реле и подготовки VPN. При подготовке в Auto используется системный DNS или заданные резолверы, при сбое — DoH. В plain и doh резервный режим не включается. Не заменяет DNS из конфигурации WireGuard.', 'DNS mode for the relay and VPN preparation. During preparation, Auto uses system DNS or custom resolvers, with DoH on failure. Plain and doh do not switch modes. Does not replace DNS in the WireGuard configuration.')} />
              <select id={id('dns-mode')} value={s.dnsMode} onChange={e => patch({dnsMode:e.target.value})}>{DNS_MODES.map(m => <option key={m} value={m}>{m}</option>)}</select>
            </div>
            <div className="field" style={{flex:2}}>
              <FieldLabel htmlFor={id('dns-servers')} label={tr('Свои DNS-серверы (через запятую)', 'Custom DNS servers (comma-separated)')} hint={tr('Необязательный список резолверов клиента. Пустое поле оставляет стандартное поведение ядра.', 'Optional client resolvers. Leaving this empty keeps the core defaults.')} />
              <input id={id('dns-servers')} type="text" value={(s.dnsServers??[]).join(', ')} onChange={e => patch({dnsServers:e.target.value.split(',').map(v=>v.trim()).filter(Boolean)})} placeholder="1.1.1.1, 1.0.0.1" />
            </div>
          </div>
        </section>

        <section className="field-group">
          <h3>{tr('Доступ и диагностика', 'Access and diagnostics')}</h3>
          <div className="field">
            <FieldLabel htmlFor={id('client-id')} label="Client ID" hint={tr('Идентификатор доступа к реле. Он должен быть разрешён на сервере; генерация нового не регистрирует его автоматически.', 'Relay access identifier. It must be authorized on the server; generating a new one does not register it automatically.')} />
            <div className="inline-actions">
              <input id={id('client-id')} type={showClientID?'text':'password'} className="mono" value={s.clientId} autoComplete="off" onChange={e => patch({clientId:e.target.value.trim()})} />
              <button type="button" className="btn small" aria-pressed={showClientID} aria-label={showClientID?tr('Скрыть Client ID', 'Hide Client ID'):tr('Показать Client ID', 'Show Client ID')} onClick={() => setShowClientID(!showClientID)}>{showClientID?tr('Скрыть', 'Hide'):tr('Показать', 'Show')}</button>
              <button type="button" className="btn small" aria-label={tr('Сгенерировать Client ID', 'Generate Client ID')} onClick={() => GenerateClientID().then(clientId => patch({clientId}))}>{tr('Сгенерировать', 'Generate')}</button>
            </div>
          </div>
          <div className="toggle-setting"><label className="checkbox-row"><input type="checkbox" checked={s.manualCaptcha} onChange={e=>patch({manualCaptcha:e.target.checked})} />{tr('Ручная VK captcha', 'Manual VK captcha')}</label><InfoTip label={tr('О ручной проверке VK', 'About manual VK verification')}>{tr('Передаёт подтверждение VK пользователю. Подтверждение отдельной ссылки не должно прерывать работающие потоки.', 'Lets the user handle VK verification. Verification for one link should not interrupt working streams.')}</InfoTip></div>
          <div className="toggle-setting"><label className="checkbox-row"><input type="checkbox" checked={s.routes} onChange={e=>patch({routes:e.target.checked})} />{tr('Автоматические маршруты к TURN-серверам (нужны права администратора)', 'Automatic TURN server routes (administrator authorization required)')}</label><InfoTip label={tr('О маршрутах к TURN', 'About TURN routes')}>{tr('Служебные маршруты транспорта реле. Это не списки маршрутизации сайтов ниже.', 'Relay transport service routes. These are not the website routing lists below.')}</InfoTip></div>
          <div className="toggle-setting"><label className="checkbox-row"><input type="checkbox" checked={s.debug} onChange={e=>patch({debug:e.target.checked})} />{tr('Подробные debug-логи', 'Verbose debug logs')}</label><InfoTip label={tr('О диагностических логах', 'About debug logs')}>{tr('Включайте для диагностики. Перед публикацией удаляйте из журнала ключи, Client ID, ссылки и идентификаторы устройства.', 'Enable for diagnostics. Remove keys, Client IDs, call links and device identifiers before sharing logs.')}</InfoTip></div>
        </section>
      </div>
    </details>
  </div>
}
