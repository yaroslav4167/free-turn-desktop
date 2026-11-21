import {test} from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'

const modal = readFileSync(new URL('../src/components/AddServerModal.tsx', import.meta.url), 'utf8')

test('import preview displays the embedded call link rather than an ignored manual value', () => {
  assert.match(modal, /const includedVKLink = preview\?\.vkLink \|\| ''/)
  assert.match(modal, /value=\{includedVKLink \|\| vkLink\}/)
  assert.match(modal, /readOnly=\{!!includedVKLink\}/)
  assert.match(modal, /Ссылка на звонок VK Calls из профиля/)
  assert.match(modal, /изменить её можно в настройках профиля/)
  assert.match(modal, /if \(vkLink.trim\(\)\) base.vkLinks = \[vkLink.trim\(\)\]/)
})

test('late parsing results cannot restore the previous profile call link or error', () => {
  assert.match(modal, /let active = true/)
  assert.match(modal, /if \(active\) setPreview\(value\)/)
  assert.match(modal, /if \(active\) setError\(String\(e\)\)/)
  assert.match(modal, /active = false; clearTimeout\(t\)/)
})
