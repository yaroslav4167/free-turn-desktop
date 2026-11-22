import {useMemo} from 'react'
import {tr} from '../language'
import {releaseNoteBlocks} from '../releaseNotes'

function inline(text: string) {
  return text.split(/(\*\*[^*]+\*\*|`[^`]+`)/g).map((part, i) =>
    part.startsWith('**') && part.endsWith('**') ? <strong key={i}>{part.slice(2,-2)}</strong>
      : part.startsWith('`') && part.endsWith('`') ? <code key={i}>{part.slice(1,-1)}</code> : part)
}

export function ReleaseNotesContent({text}: {text:string}) {
  const blocks = useMemo(() => releaseNoteBlocks(text), [text])
  if (!blocks.length) return null
  return <div className="release-notes-body" tabIndex={0} aria-label={tr('Описание релиза', 'Release notes')}>
      {blocks.map((block, i) => block.kind === 'heading' ? <h3 key={i}>{inline(block.lines.join(' '))}</h3>
        : block.kind === 'list' ? <ul key={i}>{block.lines.map((line,j)=><li key={j}>{inline(line)}</li>)}</ul>
          : block.kind === 'code' ? <pre key={i}>{block.lines.join('\n')}</pre>
            : <p key={i}>{inline(block.lines.join(' '))}</p>)}
    </div>
}
