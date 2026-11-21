import {test} from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {normalizeTheme, resolveTheme} from '../src/theme.ts'

test('theme choices normalize and resolve system appearance', () => {
  for (const value of [undefined, null, '', 'auto', 'LIGHT', 'system']) assert.equal(normalizeTheme(value), 'system')
  for (const value of ['light', 'dark']) {
    assert.equal(normalizeTheme(value), value)
    assert.equal(resolveTheme(value, true), value)
    assert.equal(resolveTheme(value, false), value)
  }
  assert.equal(resolveTheme('system', true), 'dark')
  assert.equal(resolveTheme('system', false), 'light')
})

test('theme applies before rendering, follows OS changes only in system mode and tolerates unavailable storage', async () => {
  const oldWindow = globalThis.window, oldDocument = globalThis.document
  const handlers = []
  const query = {matches: true, addEventListener: (name, listener) => handlers.push(listener)}
  const data = new Map([['freeturn:theme', 'light']])
  globalThis.document = {documentElement: {dataset: {}}}
  globalThis.window = {
    localStorage: {getItem: key => data.get(key), setItem: (key, value) => data.set(key, value)},
    matchMedia: queryText => {assert.equal(queryText, '(prefers-color-scheme: dark)'); return query},
  }
  try {
    const {initializeTheme, configureTheme, themeSnapshot, subscribeTheme} = await import('../src/theme.ts?lifecycle')
    initializeTheme()
    assert.equal(document.documentElement.dataset.theme, 'light')
    initializeTheme()
    assert.equal(handlers.length, 1)
    let notifications = 0
    const unsubscribe = subscribeTheme(() => notifications++)
    configureTheme('system')
    assert.equal(themeSnapshot(), 'system')
    assert.equal(document.documentElement.dataset.theme, 'dark')
    query.matches = false
    handlers[0]()
    assert.equal(document.documentElement.dataset.theme, 'light')
    configureTheme('dark')
    handlers[0]()
    assert.equal(document.documentElement.dataset.theme, 'dark')
    assert.equal(data.get('freeturn:theme'), 'dark')
    unsubscribe()
    window.localStorage.setItem = () => {throw Error('disabled')}
    configureTheme('light')
    assert.equal(document.documentElement.dataset.theme, 'light')
    assert.equal(notifications, 2)
    configureTheme('invalid')
    assert.equal(themeSnapshot(), 'system')
  } finally {
    if (oldWindow === undefined) delete globalThis.window
    else globalThis.window = oldWindow
    if (oldDocument === undefined) delete globalThis.document
    else globalThis.document = oldDocument
  }
})

test('both palettes cover surfaces, semantic states and native controls', () => {
  const css = readFileSync(new URL('../src/style.css', import.meta.url), 'utf8')
  const dark = css.match(/:root\s*\{([^}]+)\}/)[1]
  const light = css.match(/:root\[data-theme='light'\]\s*\{([^}]+)\}/)[1]
  for (const variable of [...light.matchAll(/(--[\w-]+):/g)].map(m => m[1])) assert.ok(dark.includes(variable+':'), variable)
  assert.match(light, /color-scheme: light/)
  for (const variable of ['--bg-card', '--bg-log', '--text', '--err', '--power-connected-text', '--scrollbar-thumb']) assert.ok(light.includes(variable+':'))
  assert.match(css, /:root\[data-theme='light'\] \.app-icon \{ content: url\('\/freeturn-light.svg'\)/)
  assert.match(readFileSync(new URL('../src/LocalizedApp.tsx', import.meta.url), 'utf8'), /configureTheme\(settings.theme\)/)
  assert.match(readFileSync(new URL('../src/components/DesktopPreferences.tsx', import.meta.url), 'utf8'), /await desktop\(\).SetTheme\(value\);configureTheme\(value\)/)
})

test('text, actions and connection states retain readable contrast in both palettes', () => {
  const css = readFileSync(new URL('../src/style.css', import.meta.url), 'utf8')
  const luminance = hex => {
    const rgb = hex.slice(1).match(/../g).map(c => parseInt(c,16)/255).map(c => c <= 0.04045 ? c/12.92 : ((c+0.055)/1.055)**2.4)
    return rgb[0]*0.2126+rgb[1]*0.7152+rgb[2]*0.0722
  }
  for (const rule of [/:root\s*\{([^}]+)\}/, /:root\[data-theme='light'\]\s*\{([^}]+)\}/]) {
    const palette = Object.fromEntries([...css.match(rule)[1].matchAll(/(--[\w-]+): (#[\da-f]{6});/g)].map(m => [m[1],m[2]]))
    for (const [foreground,background] of [
      ['--text','--bg'], ['--text-dim','--bg-card'], ['--on-accent','--accent'],
      ['--err','--bg-card'], ['--warn','--bg-card'], ['--power-connected-text','--power-connected-fill'],
      ['--power-warn-text','--power-warn-fill'], ['--info-text','--toast-info'], ['--error-text','--toast-error'],
    ]) {
      const a=luminance(palette[foreground]), b=luminance(palette[background])
      const contrast=(Math.max(a,b)+0.05)/(Math.min(a,b)+0.05)
      assert.ok(contrast>=4.5, `${foreground} on ${background}: ${contrast}`)
    }
  }
})
