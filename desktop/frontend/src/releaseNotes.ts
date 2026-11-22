export type ReleaseNoteBlock = {kind: 'heading' | 'paragraph' | 'list' | 'code'; lines: string[]}

// Deliberately small Markdown subset. No HTML, embedded images, remote assets
// or executable links: all content is rendered as React text nodes.
export function releaseNoteBlocks(source: string): ReleaseNoteBlock[] {
  const blocks: ReleaseNoteBlock[] = []
  let fenced = false
  for (const line of source.slice(0, 65540).replace(/\r\n?/g, '\n').split('\n')) {
    if (/^\s*```/.test(line)) {
      fenced = !fenced
      if (fenced) blocks.push({kind: 'code', lines: []})
      continue
    }
    if (fenced) { blocks[blocks.length-1].lines.push(line); continue }
    const text = line.trim()
    if (!text) { blocks.push({kind:'paragraph', lines:[]}); continue }
    const heading = /^#{1,6}\s+(.+)$/.exec(text)
    if (heading) { blocks.push({kind:'heading', lines:[heading[1]]}); continue }
    const item = /^(?:[-*+] |\d+[.)] )(.+)$/.exec(text)
    const kind = item ? 'list' : 'paragraph'
    const last = blocks[blocks.length-1]
    if (last?.kind === kind) last.lines.push(item ? item[1] : text)
    else blocks.push({kind, lines:[item ? item[1] : text]})
  }
  return blocks.filter(block => block.lines.length > 0)
}
