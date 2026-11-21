import {useEffect, useId, useState} from 'react'
import {tr} from '../language'
import {changeStreamCount, changeStreamsPerAccount, streamSettings} from '../streamSettings'
import type {StreamInput} from '../streamSettings'
import InfoTip from './InfoTip'

export default function StreamControls({value, onChange}: {
  value: StreamInput
  onChange: (patch: {n: number; streamsPerCred: number}) => void
}) {
  const id = useId()
  const initial = streamSettings(value)
  // Imported/custom profiles are not silently truncated to the mobile UI
  // limits. Keep their larger range stable while dragging; reset on profile key.
  const [ceilings, setCeilings] = useState({n: initial.nMax, perAccount: initial.perAccountMax})
  useEffect(() => {
    setCeilings(old => {
      const next = {n: Math.max(old.n, initial.nMax), perAccount: Math.max(old.perAccount, initial.perAccountMax)}
      return next.n === old.n && next.perAccount === old.perAccount ? old : next
    })
  }, [initial.nMax, initial.perAccountMax])
  const settings = streamSettings(value, ceilings)

  return <div className="stream-controls">
    <div className="field-row">
      <div className="field stream-control">
        <div className="stream-label">
          <label htmlFor={`${id}-n`}>{tr('TURN-потоков (-n)', 'TURN streams (-n)')}</label>
          <InfoTip label={tr('Как считаются TURN-потоки', 'How TURN streams are counted')}>
            {tr('Число потоков задаётся на каждую ссылку VK Calls. Общее число потоков равно этому значению, умноженному на число непустых ссылок. Увеличение не гарантирует прирост скорости: действуют лимиты TURN и VK.', 'Streams are configured per VK Calls link. The total is this value multiplied by the number of non-empty links. Increasing it does not guarantee more speed: TURN and VK limits still apply.')}
            <p>{tr('Изменения применяются после сохранения профиля и переподключения.', 'Changes take effect after saving the profile and reconnecting.')}</p>
          </InfoTip>
          <output htmlFor={`${id}-n`} aria-live="off">{settings.n}</output>
        </div>
        <input id={`${id}-n`} type="range" className="stream-slider" min={1} max={settings.nMax} step={1}
          value={settings.n} aria-describedby={`${id}-n-hint ${id}-summary`}
          onChange={e => onChange(changeStreamCount(value, Number(e.target.value), ceilings))} />
        <small id={`${id}-n-hint`}>{tr('На каждую ссылку · рекомендуется 8–12', 'Per link · 8–12 recommended')}</small>
      </div>
      <div className="field stream-control">
        <div className="stream-label">
          <label htmlFor={`${id}-account`}>{tr('Потоков на аккаунт', 'Streams per account')}</label>
          <InfoTip label={tr('Как распределяются потоки по аккаунтам', 'How streams are grouped by account')}>
            {tr('На каждой ссылке потоки независимо делятся на группы с общими TURN-доступами (сессиями VK). Это значение не может превышать число потоков на ссылку. Последняя группа может быть неполной; несколько ссылок не увеличивают этот предел.', 'Each link independently groups streams sharing TURN credentials (VK sessions). This value cannot exceed streams per link. The last group may be partial; adding links does not increase this limit.')}
            <p>{tr('Расчётных групп на ссылку: {perLink}; всего: {total}. Это не счётчик реально авторизованных аккаунтов.', {perLink: settings.groupsPerLink, total: settings.groups})}</p>
          </InfoTip>
          <output htmlFor={`${id}-account`} aria-live="off">{settings.perAccount}</output>
        </div>
        <input id={`${id}-account`} type="range" className="stream-slider" min={1} max={settings.perAccountMax} step={1}
          value={settings.perAccount} aria-describedby={`${id}-account-hint`}
          onChange={e => onChange(changeStreamsPerAccount(value, Number(e.target.value), ceilings))} />
        <small id={`${id}-account-hint`}>{tr('Не больше потоков на одну ссылку', 'Cannot exceed streams per link')}</small>
      </div>
    </div>
    <p id={`${id}-summary`} className="stream-summary">
      {settings.links > 0
        ? tr('Всего TURN-потоков: {streams} × {links} = {total}', {streams: settings.n, links: settings.links, total: settings.total})
        : tr('Добавьте ссылку VK Calls для расчёта общего числа потоков.', 'Add a VK Calls link to calculate the total stream count.')}
    </p>
    {settings.extended && <p className="stream-summary">
      {tr('Сохранённые расширенные значения не уменьшаются при открытии профиля.', 'Saved extended values are not reduced when opening the profile.')}
    </p>}
  </div>
}
