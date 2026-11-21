import {tr} from '../language'
import type {Theme} from '../theme'

export default function ThemePicker({value, disabled, onChange}: {
  value: Theme
  disabled: boolean
  onChange: (theme: Theme) => void
}) {
  const choices: {value: Theme; label: string}[] = [
    {value:'system', label:tr('Как в системе', 'System default')},
    {value:'light', label:tr('Светлая', 'Light')},
    {value:'dark', label:tr('Тёмная', 'Dark')},
  ]
  return <div className="theme-options" role="radiogroup" aria-label={tr('Тема оформления', 'Appearance')}>
    {choices.map(choice => <label className="theme-option" key={choice.value} title={choice.label}>
      <input id={`interface-theme-${choice.value}`} className="sr-only" type="radio" name="interface-theme"
        value={choice.value} aria-label={choice.label} checked={value===choice.value} disabled={disabled} onChange={()=>onChange(choice.value)} />
      <span className="theme-choice">
        <svg className="theme-swatch" viewBox="0 0 32 32" width="24" height="24" aria-hidden="true">
          <circle cx="16" cy="16" r="14" fill={choice.value==='dark'?'#1b1c24':'#f5f6fa'} />
          {choice.value==='system'&&<path d="M16 2a14 14 0 0 1 0 28Z" fill="#1b1c24" />}
          <circle cx="16" cy="16" r="14" fill="none" stroke="currentColor" />
        </svg>
      </span>
    </label>)}
  </div>
}
