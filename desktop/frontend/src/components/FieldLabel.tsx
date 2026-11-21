import type {ReactNode} from 'react'
import {tr} from '../language'
import InfoTip from './InfoTip'

export default function FieldLabel({htmlFor, label, hint}: {htmlFor: string; label: string; hint: ReactNode}) {
  return <div className="field-label">
    <label htmlFor={htmlFor}>{label}</label>
    <InfoTip label={tr('О поле «{name}»', {name: label})}>{hint}</InfoTip>
  </div>
}
