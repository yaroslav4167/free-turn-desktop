import {test} from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {configuredLinkCount, streamSettings, changeStreamCount, changeStreamsPerAccount} from '../src/streamSettings.ts'

test('mobile ranges are dependent per link, not on the combined stream total', () => {
  const settings = streamSettings({n: 12, streamsPerCred: 12, vkLinks: ['A', 'B', 'C']})
  assert.equal(settings.nMax, 128)
  assert.equal(settings.perAccountMax, 12)
  assert.equal(settings.total, 36)
  assert.deepEqual(changeStreamsPerAccount({n: 4, streamsPerCred: 2, vkLinks: ['A', 'B', 'C']}, 12), {n: 4, streamsPerCred: 4})
  assert.equal(streamSettings({n: 128, streamsPerCred: 12}).perAccountMax, 50)
})

test('decreasing streams clamps account grouping atomically; increasing does not raise it', () => {
  const input = {n: 12, streamsPerCred: 12, vkLinks: ['A', 'B']}
  assert.deepEqual(changeStreamCount(input, 5), {n: 5, streamsPerCred: 5})
  assert.deepEqual(changeStreamCount({...input, n: 5, streamsPerCred: 3}, 20), {n: 20, streamsPerCred: 3})
  assert.deepEqual(input, {n: 12, streamsPerCred: 12, vkLinks: ['A', 'B']})
})

test('partial cache groups are counted separately for each link', () => {
  const settings = streamSettings({n: 12, streamsPerCred: 5, vkLinks: ['A', 'B']})
  assert.equal(settings.total, 24)
  assert.equal(settings.groupsPerLink, 3)
  assert.equal(settings.groups, 6) // Not ceil(24/5) = 5: links have separate stores.
  assert.deepEqual(changeStreamsPerAccount({n: 13, streamsPerCred: 12}, 5), {n: 13, streamsPerCred: 5})
})

test('editing links changes only the total and follows core link normalization', () => {
  const links = [' ', 'https://vk.ru/call/join/ONE?x=1', '', 'TWO', 'https://vk.ru/call/join/', '\t']
  const original = [...links]
  assert.equal(configuredLinkCount(links), 2)
  assert.equal(configuredLinkCount(['ONE,TWO']), 2)
  assert.equal(configuredLinkCount(['ONE', 'ONE']), 2) // Core also keeps duplicates.
  assert.equal(configuredLinkCount(), 0)
  assert.equal(streamSettings({n: 12, streamsPerCred: 4, vkLinks: []}).total, 0)
  assert.deepEqual(links, original)
  const settings = streamSettings({n: 12, streamsPerCred: 4, vkLinks: links})
  assert.equal(settings.n, 12)
  assert.equal(settings.perAccount, 4)
  assert.equal(settings.total, 24)
})

test('valid imported extended settings are not truncated to mobile UI limits', () => {
  const input = {n: 200, streamsPerCred: 80, vkLinks: ['A']}
  const settings = streamSettings(input)
  assert.equal(settings.n, 200)
  assert.equal(settings.perAccount, 80)
  assert.equal(settings.nMax, 200)
  assert.equal(settings.perAccountMax, 80)
  assert.equal(settings.extended, true)
  assert.deepEqual(changeStreamCount(input, 150, {n: 200, perAccount: 80}), {n: 150, streamsPerCred: 80})
  assert.equal(streamSettings({n: 150, streamsPerCred: 80}, {n: 200, perAccount: 80}).nMax, 200)
  assert.deepEqual(input, {n: 200, streamsPerCred: 80, vkLinks: ['A']})
})

test('slider changes keep positive integers and account streams never exceed per-link streams', () => {
  for (let n = 1; n <= 128; n++) {
    for (let perAccount = 1; perAccount <= 50; perAccount++) {
      const input = {n, streamsPerCred: perAccount}
      const changed = changeStreamsPerAccount(input, perAccount)
      assert.ok(Number.isInteger(changed.n) && changed.n >= 1 && changed.n <= 128)
      assert.ok(changed.streamsPerCred >= 1 && changed.streamsPerCred <= Math.min(n, 50))
      const lowered = changeStreamCount(changed, 1)
      assert.deepEqual(lowered, {n: 1, streamsPerCred: 1})
    }
  }
  assert.equal(changeStreamCount({n: 12, streamsPerCred: 12}, Infinity).n, 12)
  assert.equal(changeStreamCount({n: 12, streamsPerCred: 12}, 9.8).n, 9)
})

test('both labelled keyboard-accessible sliders replace the old numeric fields', () => {
  const component = readFileSync(new URL('../src/components/StreamControls.tsx', import.meta.url), 'utf8')
  const form = readFileSync(new URL('../src/components/ServerForm.tsx', import.meta.url), 'utf8')
  assert.equal((component.match(/type="range"/g) || []).length, 2)
  assert.match(component, /Потоков на аккаунт/)
  assert.match(component, /htmlFor=/)
  assert.match(component, /aria-describedby=/)
  assert.match(component, /max=\{settings.perAccountMax\}/)
  assert.match(form, /<StreamControls key=\{s.id \|\| 'new'\}/)
  assert.doesNotMatch(form, /Потоков на VK-креды|type="number" min=\{1\} value=\{s.n\}/)
})
