import {test} from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {releaseNoteBlocks} from '../src/releaseNotes.ts'

test('release notes format headings, bullet lists, paragraphs and fenced code', () => {
  assert.deepEqual(releaseNoteBlocks('# Release\r\n\r\n## Profiles\r\n- Export\r\n- Import\r\n\r\nFirst line\r\nsecond line\r\n\r\n```sh\r\nexample --help\r\n```'), [
    {kind:'heading',lines:['Release']},
    {kind:'heading',lines:['Profiles']},
    {kind:'list',lines:['Export','Import']},
    {kind:'paragraph',lines:['First line','second line']},
    {kind:'code',lines:['example --help']},
  ])
})

test('release descriptions stay text, not HTML, executable links or remote images', () => {
  const untrusted='<script>alert(1)</script>\n<img src="https://example.test/tracker">\n[run](javascript:alert(1))'
  const blocks=releaseNoteBlocks(untrusted)
  assert.equal(blocks[0].kind,'paragraph')
  assert.equal(blocks[0].lines.join('\n'),untrusted)
  const component=readFileSync(new URL('../src/components/ReleaseNotes.tsx',import.meta.url),'utf8')
  assert.doesNotMatch(component,/dangerouslySetInnerHTML|<img|<iframe|<a\s/)
  assert.match(component,/className="release-notes-body"/)
  assert.doesNotMatch(component,/<details/)
})

test('settings expose history beside the check action even when already up to date', () => {
  const settings=readFileSync(new URL('../src/components/UpdatePreferences.tsx',import.meta.url),'utf8')
  assert.doesNotMatch(settings,/<ReleaseNotes key=/)
  assert.match(settings,/disabled=\{!state\?\.repository\} onClick=\{\(\)=>showHistory\(true\)\}/)
  assert.ok(settings.indexOf("tr('История обновлений'")>settings.indexOf("tr('Проверить обновления'"))
  const history=readFileSync(new URL('../src/components/ReleaseHistoryDialog.tsx',import.meta.url),'utf8')
  assert.match(history,/useDialogFocus\(onClose\)/)
  assert.match(history,/desktop\(\).ReleaseHistory\(\)/)
  assert.match(history,/<time dateTime=\{entry.publishedAt\}/)
  assert.match(history,/<ReleaseNotesContent text=\{entry.notes\}/)
  assert.match(history,/<article className="release-history-entry">/)
  assert.doesNotMatch(history,/<details|<summary/)
  assert.doesNotMatch(history,/InstallUpdate\(|DownloadUpdate\(/)
})

test('empty notes disappear and oversized notes have bounded work', () => {
  assert.deepEqual(releaseNoteBlocks(' \n\n'),[])
  const blocks=releaseNoteBlocks('x'.repeat(1000000))
  assert.ok(blocks[0].lines[0].length<=65540)
})
