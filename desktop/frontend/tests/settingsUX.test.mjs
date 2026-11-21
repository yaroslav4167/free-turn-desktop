import {test} from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {serverDetailState} from '../src/serverDetailState.ts'
import {connectionState} from '../src/connectionState.ts'

const read = name => readFileSync(new URL('../src/'+name, import.meta.url), 'utf8')

test('local setup immediately shows connecting before the first backend snapshot', () => {
  const state = serverDetailState('a', {serverId:'', state:'idle'}, 'a')
  assert.equal(state.phase, 'connecting')
  assert.equal(state.busy, true)
  assert.equal(state.progressing, true)
  assert.equal(state.connected, false)
  assert.equal(connectionState('idle', true, true).action, 'Отменить')
  assert.equal(serverDetailState('a',{serverId:'a',state:'error'},'a').busy,true)
})

test('finishing setup does not show connected early or reuse another profile state', () => {
  for (const status of [{serverId:'a',state:'connected'}, {serverId:'old',state:'connected'}]) {
    const state = serverDetailState('a',status,'a')
    assert.equal(state.phase,'connecting')
    assert.equal(state.connected,false)
  }
  assert.equal(serverDetailState('a',{serverId:'a',state:'connected'},null).connected,true)
})

test('captcha waits without spinning; unrelated profiles cannot start a second setup', () => {
  const captcha = serverDetailState('a',{serverId:'a',state:'captcha'},'a')
  assert.equal(captcha.busy,false)
  assert.equal(captcha.progressing,true)
  const other = serverDetailState('b',{serverId:'a',state:'connecting'},'a')
  assert.equal(other.otherPreparing,true)
  assert.equal(other.busy,false)
  assert.equal(other.connected,false)
  const error = serverDetailState('a',{serverId:'a',state:'error'},null)
  assert.equal(error.busy,false)
  assert.equal(error.progressing,false)
})

test('connecting button has a compact loader, cancel action and duplicate-submit protection', () => {
  const detail = read('components/ServerDetail.tsx')
  assert.match(detail,/aria-busy=\{busy\}/)
  assert.match(detail,/<ConnectionLoader active=\{true\}/)
  assert.match(detail,/presentation.progressing \|\| presentation.otherPreparing/)
  assert.match(detail,/presentation.progressing && <button[^>]+onClick=\{onDisconnect\}/)
  assert.match(detail, /<details className="profile-log-panel">/)
  assert.match(detail, /<LogConsole lines=\{logs\}/)
  const app=read('App.tsx')
  assert.match(app,/pendingServerId=\{pendingServerId\}/)
  const connect=app.slice(app.indexOf('const handleConnect'),app.indexOf('const handleDisconnect'))
  assert.ok(connect.indexOf('if (request !== connectionRequest.current) return') < connect.indexOf('await Connect(draft.id)'))
  assert.match(connect,/request === connectionRequest.current/)
})

test('advanced fields remain mounted but folded, with accessible field hints', () => {
  const form=read('components/ServerForm.tsx')
  assert.ok(form.indexOf('<StreamControls') < form.indexOf('<details ref={advanced}'))
  assert.doesNotMatch(form,/<details ref=\{advanced\}[^>]*open=/)
  assert.match(form,/onChange\(\{\.\.\.s, \.\.\.p\}/)
  for (const key of ['obfProfile','obfKey','transport','mode','listen','dnsMode','dnsServers','clientId','manualCaptcha','routes','debug','ackNoDelay']) assert.ok(form.includes(key),key)
  const label=read('components/FieldLabel.tsx')
  assert.match(label,/htmlFor=\{htmlFor\}/)
  assert.match(label,/<InfoTip/)
  assert.match(form,/type=\{showKey\?'text':'password'\}/)
  assert.match(form,/setShowClientID\(false\)/)
})

test('form height is unified without resizing switches, multiline editors or the main power button', () => {
  const css=read('style.css')
  assert.match(css,/--settings-control-height: 36px/)
  assert.match(css,/\.detail \.btn, \.modal \.btn, \.settings-page \.btn/)
  assert.match(css,/input\[type='text'\], input\[type='password'\], input\[type='number'\], select/)
  assert.match(css,/height: var\(--settings-control-height\)/)
  assert.match(css,/\.compact-loader \.connection-loader[^}]*opacity: 1/)
  const prefs=read('components/DesktopPreferences.tsx')
  assert.match(prefs,/settings.autoConnect &&/)
  assert.doesNotMatch(prefs,/setSettings\(null\)/)
})

test('existing-container wizard uses styled text inputs and keeps actions outside the manual fold', () => {
  const wizard=read('components/CreateServerWizard.tsx')
  const existing=wizard.slice(wizard.indexOf("if (step === 'existing')"),wizard.indexOf("if (step === 'config')"))
  assert.match(existing,/type="text" value=\{deviceName\}/)
  assert.ok(existing.lastIndexOf('className="wizard-actions"')>existing.lastIndexOf('</details>'))
  assert.match(existing,/className="container-meta"/)
  const option=existing.match(/containers.map\(c=><option[^\n]+/)[0]
  assert.doesNotMatch(option,/c.image/)
  const review=wizard.slice(wizard.indexOf("if (step === 'review'"),wizard.indexOf("if (step === 'existing')"))
  assert.match(review,/type="text" value=\{existingDraft.peer\}/)
  assert.match(review,/className="endpoint-options"/)
  assert.match(wizard,/node.scrollTop = 0/)
  assert.match(wizard,/\}, \[step\]\)/)
  assert.match(read('style.css'),/\.deployment-modal \.wizard-actions[^}]*position: sticky/)
})

test('modal keyboard focus cannot escape to underlying connection controls', () => {
  const modal=read('components/AddServerModal.tsx'),focus=read('useDialogFocus.ts')
  assert.match(modal,/role="dialog" aria-modal="true"/)
  assert.match(modal,/useDialogFocus\(onClose\)/)
  assert.match(focus,/event.key !== 'Tab'/)
  assert.match(focus,/opener.focus\(\{preventScroll:true\}\)/)
  assert.match(focus,/\[role="tooltip"\]/)
  assert.match(focus,/close.current = onClose/)
})

test('destructive local and remote actions are clearly distinguished', () => {
  assert.match(read('App.tsx'),/Удалить локальный профиль/)
  assert.match(read('components/ServerDetail.tsx'),/Удалить профиль/)
  const management=read('components/ServerManagement.tsx')
  assert.match(management,/server.ssh\?\.manager==='hackdiaz-docker'/)
  assert.match(management,/className="danger-zone"/)
  assert.match(management,/window.confirm/)
  const cli=read('components/ProfileCommand.tsx')
  assert.match(cli,/active.current=false/)
  assert.match(read('components/ServerDetail.tsx'),/ProfileCommand key=\{`\$\{server.id\}-/)
})

test('scrollbars are themed without hiding scrolling or overriding high-contrast mode', () => {
  const css=read('style.css')
  assert.match(css,/scrollbar-color: var\(--scrollbar-thumb\) transparent/)
  assert.match(css,/::-webkit-scrollbar-thumb:hover/)
  assert.match(css,/@media \(forced-colors: active\)/)
  assert.match(css,/scrollbar-color: auto; scrollbar-width: auto/)
  assert.doesNotMatch(css,/scrollbar-width:\s*none/)
})
