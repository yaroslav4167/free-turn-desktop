import {test} from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {connectionState} from '../src/connectionState.ts'

const read = file => readFileSync(new URL('../src/' + file, import.meta.url), 'utf8')

test('captcha action is independent of connection phase and visible on every screen', () => {
  const notice = read('components/CaptchaNotice.tsx')
  assert.match(notice, /if \(!url\) return null/)
  assert.match(notice, /desktop\(\).OpenCaptcha\(\)/)
  assert.match(notice, /desktop\(\).OpenCaptchaExternal\(\)/)
  assert.doesNotMatch(notice, /BrowserOpenURL\(url\)/)
  assert.doesNotMatch(notice, /state === 'captcha'/)
  const app = read('App.tsx')
  assert.match(app, /<CaptchaNotice url=\{status\.captchaUrl \|\| ''\}/)
  assert.ok(app.indexOf('<CaptchaNotice') < app.indexOf("{view === 'connect'"))
  assert.match(app,/blocked=\{showAdd \|\| !!status.captchaUrl \|\| pending\}/)
})

test('connected captcha warning keeps disconnect action and uses amber rather than a disconnected state', () => {
  const state = connectionState('connected', false, true)
  assert.equal(state.connected, true)
  assert.equal(state.action, 'Отключить')
  assert.equal(state.heading, 'VPN включён')
  const component = read('components/Connection.tsx')
  assert.match(component, /verificationRequired = connected && !!status\.captchaUrl/)
  assert.match(component, /verification-required/)
  const css = read('style.css')
  assert.match(css, /\.verification-required \.power-button\s*\{[^}]*--power-fill:\s*var\(--power-warn-fill\)/)
  assert.match(css, /--power-warn-fill: #493f26/)
  assert.match(css, /:root\[data-theme='light'\][^}]*--power-warn-fill: #fff0c7/)
})

test('verification opens automatically in an isolated child before normal desktop startup', () => {
  const main=readFileSync(new URL('../../main.go',import.meta.url),'utf8')
  assert.ok(main.indexOf('captchaview.RunChild(os.Stdin)')<main.indexOf('integration.HandleUpdateProcess()'))
  assert.ok(main.indexOf('captchaview.RunChild(os.Stdin)')<main.indexOf('NewApp()'))
  const engine=readFileSync(new URL('../../backend/engine.go',import.meta.url),'utf8')
  assert.match(engine,/presenter\(url\)/)
  assert.match(engine,/url != "" && presenter == nil/)
  const notice=read('components/CaptchaNotice.tsx')
  assert.match(notice,/onClick=\{\(\) => open\(\)\}/)
  assert.match(notice,/onClick=\{\(\) => open\(true\)\}/)
})

test('manual captcha preference explicitly describes skipping the automatic attempt', () => {
  const form=read('components/ServerForm.tsx')
  assert.match(form,/checked=\{s.manualCaptcha\}/)
  assert.match(form,/Пропускать авто-решение captcha/)
  assert.match(form,/сначала пробует авто-решение и открывает окно только при необходимости/)
})
