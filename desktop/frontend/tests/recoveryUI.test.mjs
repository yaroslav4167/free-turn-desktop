import {test} from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'

const read = path => readFileSync(new URL('../src/'+path, import.meta.url), 'utf8')

test('profile draft refresh cannot erase startup connection errors', () => {
  const app=read('App.tsx')
  const draftEffect=app.slice(app.indexOf('const saved = servers.find'),app.indexOf('}, [selectedId, servers])'))
  assert.doesNotMatch(draftEffect,/setConnectError/)
  assert.match(app,/settings\?\.autoConnectError\) setConnectError/)
})

test('log follows updates even when capped and respects manual scrolling', () => {
  const log=read('components/LogConsole.tsx')
  assert.match(log,/\}, \[lines\]\)/)
  assert.doesNotMatch(log,/\[lines.length\]/)
  assert.match(log,/el && followTail.current/)
  assert.match(log,/onScroll=/)
})

test('profiles expose import beside add and an accessible export context menu', () => {
  const sidebar=read('components/Sidebar.tsx')
  assert.match(sidebar,/onContextMenu=/)
  assert.match(sidebar,/role="menuitem"/)
  assert.match(sidebar,/onExport\(menu.id\)/)
  assert.match(sidebar,/onClick=\{onImport\}/)
  assert.ok(sidebar.indexOf('onClick={onImport}') > sidebar.indexOf('onClick={onAdd}'))
  assert.match(sidebar,/className="btn-add profile-import"/)
  assert.match(read('style.css'),/\.sidebar-footer \.btn-add\s*\{[^}]*white-space: nowrap/)
  assert.match(sidebar,/e.key === 'Escape'/)
})

test('connection progress is rendered below power button and stale polling is cancelled', () => {
  const connection=read('components/Connection.tsx')
  assert.ok(connection.indexOf('<h2 aria-live')>connection.indexOf('className={\'power-button'))
  assert.match(connection,/ConnectionProgress\(\)/)
  assert.match(connection,/active = false; clearTimeout/)
  assert.match(connection,/progressLabel\(stage\)/)
})
