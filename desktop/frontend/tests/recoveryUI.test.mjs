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

test('diagnostics are collapsed by default but recovery warnings expand the section', () => {
  const settings=read('components/DesktopPreferences.tsx')
  assert.match(settings,/\[diagnosticsOpen, setDiagnosticsOpen\] = useState\(false\)/)
  assert.match(settings,/<details className="settings-card settings-diagnostics" open=\{diagnosticsOpen\}/)
  assert.match(settings,/<summary>\{tr\('Диагностика и восстановление'/)
  assert.match(settings,/recovery === 'damaged' \|\| recovery === 'recovered'\) setDiagnosticsOpen\(true\)/)
  const panel=settings.slice(settings.indexOf('<details className="settings-card settings-diagnostics"'),settings.indexOf('</details>',settings.indexOf('<details className="settings-card settings-diagnostics"')))
  assert.match(panel,/ExportDiagnostics\(\)/)
  assert.match(panel,/RestoreProfilesBackup\(\)/)
})
