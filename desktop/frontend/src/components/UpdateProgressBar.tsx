export default function UpdateProgressBar({value, label}: {value: number | null; label: string}) {
  const known = value !== null && Number.isFinite(value)
  const bounded = known ? Math.max(0, Math.min(100, value)) : undefined
  return <div className={'update-progress' + (known ? '' : ' indeterminate')}
    role="progressbar" aria-label={label} aria-valuemin={0} aria-valuemax={100} aria-valuenow={bounded}>
    <span className="update-progress-fill" aria-hidden="true"
      style={bounded === undefined ? undefined : {transform: `scaleX(${bounded / 100})`}} />
  </div>
}
