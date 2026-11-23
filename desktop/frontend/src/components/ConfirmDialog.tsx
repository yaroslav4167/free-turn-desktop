import {createPortal} from 'react-dom'
import {useId} from 'react'
import {tr} from '../language'
import {useDialogFocus} from '../useDialogFocus'

export interface Confirmation {
  message: string
  label: string
  danger?: boolean
  resolve: (accepted: boolean) => void
}

export default function ConfirmDialog({request, onAnswer}: {request: Omit<Confirmation, 'resolve'>; onAnswer: (accepted: boolean) => void}) {
  const id = useId()
  const dialog = useDialogFocus(() => onAnswer(false))
  return createPortal(<div className="modal-backdrop profile-confirm-backdrop" onClick={() => onAnswer(false)}>
    <div ref={dialog} className="modal" tabIndex={-1} role="alertdialog" aria-modal="true" aria-labelledby={id} onClick={e => e.stopPropagation()}>
      <p id={id}>{request.message}</p>
      <div className="modal-actions">
        <button type="button" className="btn" onClick={() => onAnswer(false)}>{tr('Отмена', 'Cancel')}</button>
        <button type="button" className={'btn ' + (request.danger ? 'danger' : 'primary')} onClick={() => onAnswer(true)}>{request.label}</button>
      </div>
    </div>
  </div>, document.body)
}
