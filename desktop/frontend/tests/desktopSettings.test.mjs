import {test} from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {createDesktopSettingsCache} from '../src/desktopSettings.ts'

test('bootstrap and settings reuse a snapshot and deduplicate concurrent Windows queries', async () => {
  let calls = 0
  let resolve
  const cache = createDesktopSettingsCache(() => {
    calls++
    return new Promise(done => { resolve = done })
  })
  assert.equal(cache.snapshot(), null)
  const first = cache.load()
  const second = cache.load()
  assert.equal(first, second)
  await Promise.resolve()
  assert.equal(calls, 1)
  const settings = {autostart: true, theme: 'system'}
  resolve(settings)
  await first
  assert.equal(cache.snapshot(), settings)
  assert.equal(await cache.load(), settings)
  assert.equal(calls, 1)
})

test('saving preferences explicitly refreshes the shared snapshot', async () => {
  let value = {autostart: false}
  let calls = 0
  const cache = createDesktopSettingsCache(async () => { calls++; return value })
  await cache.load()
  value = {autostart: true}
  assert.equal(cache.snapshot().autostart, false)
  await cache.load(true)
  assert.equal(cache.snapshot(), value)
  assert.equal(calls, 2)
})

test('a failed bootstrap does not poison the next attempt to open settings', async () => {
  let calls = 0
  const settings = {autostart: true}
  const cache = createDesktopSettingsCache(async () => {
    if (++calls === 1) throw new Error('initial read failed')
    return settings
  })
  await assert.rejects(cache.load(), /initial read failed/)
  assert.equal(cache.snapshot(), null)
  assert.equal(await cache.load(), settings)
  assert.equal(calls, 2)
})

test('late reads cannot overwrite a refreshed snapshot and failures remain retryable', async () => {
  const requests = []
  const cache = createDesktopSettingsCache(() => new Promise((resolve, reject) => requests.push({resolve, reject})))
  const old = cache.load()
  const fresh = cache.load(true)
  await Promise.resolve()
  requests[1].resolve({autostart: true})
  await fresh
  requests[0].resolve({autostart: false})
  await old
  assert.equal(cache.snapshot().autostart, true)
  const failure = cache.load(true)
  await Promise.resolve()
  requests[2].reject(new Error('scheduler unavailable'))
  await assert.rejects(failure, /scheduler unavailable/)
  assert.equal(cache.snapshot().autostart, true)
  const retry = cache.load(true)
  await Promise.resolve()
  requests[3].resolve({autostart: false})
  await retry
  assert.equal(cache.snapshot().autostart, false)
})

test('all three settings consumers share the cache and preferences render the cached snapshot immediately', () => {
  const read = file => readFileSync(new URL('../src/' + file, import.meta.url), 'utf8')
  for (const file of ['LocalizedApp.tsx', 'App.tsx', 'components/DesktopPreferences.tsx']) {
    const source = read(file)
    assert.match(source, /desktopSettings\.load\(\)/)
    assert.doesNotMatch(source, /desktop\(\)\.DesktopSettings\(\)/)
  }
  assert.match(read('components/DesktopPreferences.tsx'), /useState<DesktopSettings \| null>\(\(\) => desktopSettings\.snapshot\(\)\)/)
  assert.match(read('components/DesktopPreferences.tsx'), /desktopSettings\.load\(true\)\.then\(setSettings\)/)
})
