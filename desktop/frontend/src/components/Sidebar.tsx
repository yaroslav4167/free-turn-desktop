import {backend} from '../api'
import {tr} from '../language'
import {useEffect, useRef, useState} from 'react'
import {createPortal} from 'react-dom'
import Icon from './Icon'

interface Props {
  servers: backend.Server[]
  selectedId: string | null
  activeState: string
  activeServerId: string
  onSelect: (id: string) => void
  onAdd: () => void
  onImport: () => void
  onExport: (id: string) => void
}

export default function Sidebar({servers, selectedId, activeState, activeServerId, onSelect, onAdd, onImport, onExport}: Props) {
  const [menu, setMenu] = useState<{id: string; x: number; y: number} | null>(null)
  const menuRef = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement | null>(null)
  useEffect(() => {
    if (!menu) return
    menuRef.current?.querySelector('button')?.focus()
    const close = () => setMenu(null)
    const outside = (e: PointerEvent) => { if (!menuRef.current?.contains(e.target as Node)) close() }
    const key = (e: KeyboardEvent) => { if (e.key === 'Escape') { close(); trigger.current?.focus() } }
    document.addEventListener('pointerdown', outside)
    document.addEventListener('keydown', key)
    window.addEventListener('resize', close)
    window.addEventListener('blur', close)
    return () => { document.removeEventListener('pointerdown', outside); document.removeEventListener('keydown', key); window.removeEventListener('resize', close); window.removeEventListener('blur', close) }
  }, [menu])
  return (
    <div className="sidebar">
      <div className="sidebar-header">
        <h1>{tr('Серверы', 'Servers')}</h1>
        <span className="version">{tr('Профили подключения', 'Connection profiles')} · {servers.length}</span>
      </div>
      <div className="server-list">
        {servers.map((s) => {
          const isActive = s.id === activeServerId && activeState !== 'idle'
          return (
            <button
              type="button"
              key={s.id}
              className={'server-item' + (s.id === selectedId ? ' active' : '')}
              aria-current={s.id === selectedId ? 'true' : undefined}
              title={`${s.name || s.peer} · ${s.peer}`}
              onClick={() => onSelect(s.id)}
              onContextMenu={e => {
                e.preventDefault(); trigger.current = e.currentTarget
                const rect = e.currentTarget.getBoundingClientRect()
                setMenu({id:s.id, x:Math.min(e.clientX || rect.left, Math.max(8, window.innerWidth-240)), y:Math.min(e.clientY || rect.bottom, Math.max(8, window.innerHeight-64))})
              }}
            >
              <span className={'dot' + (isActive ? ' ' + activeState : '')} />
              <span className="name">
                {s.name || s.peer || tr('Без имени', 'Unnamed')}
                <span className="peer">{s.peer}</span>
              </span>
            </button>
          )
        })}
        {servers.length === 0 && (
          <div style={{color: 'var(--text-dim)', padding: '10px', fontSize: '12px'}}>{tr('Серверов пока нет', 'No servers yet')}</div>
        )}
      </div>
      <div className="sidebar-footer">
        <button className="btn-add" onClick={onAdd}>
          + {tr('Добавить сервер', 'Add server')}
        </button>
        <button className="btn-add profile-import" onClick={onImport} title={tr('Импортировать', 'Import')} aria-label={tr('Импортировать', 'Import')}><Icon name="upload" size={18} /></button>
      </div>
      {menu && createPortal(<div ref={menuRef} className="profile-context-menu" role="menu" style={{position:'fixed', left:menu.x, top:menu.y, zIndex:10000}}>
        <button className="btn" role="menuitem" onClick={() => { onExport(menu.id); setMenu(null); trigger.current?.focus() }}>{tr('Экспортировать профиль…', 'Export profile…')}</button>
      </div>, document.body)}
    </div>
  )
}
