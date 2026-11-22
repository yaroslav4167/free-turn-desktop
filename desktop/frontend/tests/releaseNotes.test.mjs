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
  assert.match(component,/<details className="release-notes">/)
  assert.doesNotMatch(component,/<details[^>]*\bopen\b/)
})

test('empty notes disappear and oversized notes have bounded work', () => {
  assert.deepEqual(releaseNoteBlocks(' \n\n'),[])
  const blocks=releaseNoteBlocks('x'.repeat(1000000))
  assert.ok(blocks[0].lines[0].length<=65540)
})
