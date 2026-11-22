import {test} from 'node:test'
import assert from 'node:assert/strict'
import {updateNoticeId, readDismissedUpdate, saveDismissedUpdate} from '../src/updateNotice.ts'
import {readFileSync} from 'node:fs'

test('dismissed update stays hidden only for the same repository and version', () => {
  const values = new Map()
  const storage = {getItem: (key) => values.get(key) ?? null, setItem: (key, value) => values.set(key, value)}
  const release = {repository: 'example/fork', latestVersion: 'v2.1.0'}
  const id = updateNoticeId(release)
  assert.equal(readDismissedUpdate(storage), '')
  saveDismissedUpdate(id, storage)
  assert.equal(readDismissedUpdate(storage), id)
  assert.notEqual(updateNoticeId({...release, latestVersion: 'v2.2.0'}), id)
  assert.notEqual(updateNoticeId({...release, repository: 'other/fork'}), id)
})

test('unavailable storage does not break the update notification', () => {
  const storage = {getItem: () => {throw new Error('denied')}, setItem: () => {throw new Error('full')}}
  assert.equal(readDismissedUpdate(storage), '')
  assert.doesNotThrow(() => saveDismissedUpdate('release', storage))
})

test('update detection opens a dismissible dialog with visible changes and explicit update actions', () => {
  const component=readFileSync(new URL('../src/components/UpdatePreferences.tsx',import.meta.url),'utf8')
  assert.match(component,/blocked \|\| !state\?\.available \|\| dismissed === updateNoticeId\(state\)/)
  assert.match(component,/useDialogFocus\(close\)/)
  assert.match(component,/<ReleaseNotesContent text=\{state.releaseNotes\}/)
  assert.match(component,/run\('installing',\(\)=>desktop\(\).InstallUpdate\(\)\)/)
  assert.match(component,/run\('downloading',\(\)=>desktop\(\).DownloadUpdate\(\)\)/)
  assert.match(component,/saveDismissedUpdate\(id\)/)
  assert.match(component,/tr\('Позже', 'Later'\)/)
  assert.match(component,/state.downloadedPath && !busy/)
})

test('large changelogs scroll inside the dialog while action buttons remain outside', () => {
  const css=readFileSync(new URL('../src/style.css',import.meta.url),'utf8')
  assert.match(css,/\.release-notes-body\s*\{[^}]*overflow-y: auto/)
  assert.match(css,/\.update-dialog \.release-notes-body\s*\{[^}]*max-height: min\(45vh, 360px\)/)
  assert.match(css,/\.update-dialog > :not\(\.release-notes-body\)\s*\{[^}]*flex-shrink: 0/)
})

test('completed checks update the UI immediately without a stale poll overwriting the event', () => {
  const component=readFileSync(new URL('../src/components/UpdatePreferences.tsx',import.meta.url),'utf8')
  assert.match(component,/EventsOn\('desktop:update'/)
  assert.match(component,/revision\+\+/)
  assert.match(component,/active && startedAt === revision/)
  assert.match(component,/offUpdate\(\)/)
})
