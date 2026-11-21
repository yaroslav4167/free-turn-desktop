import {useEffect, useRef} from 'react'

export function dialogFocusTargets(node: HTMLElement) {
  return [...node.querySelectorAll<HTMLElement>('button,input,select,textarea,a[href],summary,[tabindex]')].filter(el => {
    if (el.hasAttribute('disabled') || el.tabIndex < 0 || !el.getClientRects().length) return false
    for (let p = el.parentElement; p && p !== node; p = p.parentElement) {
      if (p instanceof HTMLDetailsElement && !p.open && !p.querySelector('summary')?.contains(el)) return false
    }
    return true
  })
}

// Keep keyboard focus inside the modal and restore the opener on close. The
// callback ref prevents stats polling/re-renders from stealing typing focus.
export function useDialogFocus(onClose: () => void) {
  const dialog = useRef<HTMLDivElement>(null)
  const close = useRef(onClose)
  useEffect(() => { close.current = onClose }, [onClose])
  useEffect(() => {
    const node = dialog.current
    if (!node) return
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null
    const initial = requestAnimationFrame(() => {
      if (node.isConnected) (node.querySelector('textarea') || node.querySelector<HTMLElement>('input,select,button'))?.focus()
    })
    const keydown = (event: KeyboardEvent) => {
      if (event.defaultPrevented) return
      if (event.key === 'Escape') {
        // InfoTip handles its own Escape; do not dismiss the entire dialog.
        if (document.querySelector('[role="tooltip"]')) return
        event.preventDefault()
        close.current()
      }
      if (event.key !== 'Tab' || event.ctrlKey || event.metaKey || event.altKey) return
      const items = dialogFocusTargets(node), first = items[0], last = items.at(-1)
      if (!first || !last) { event.preventDefault(); node.focus(); return }
      if (!node.contains(document.activeElement) || (event.shiftKey && document.activeElement === first) || (!event.shiftKey && document.activeElement === last)) {
        event.preventDefault()
        ;(event.shiftKey ? last : first).focus()
      }
    }
    document.addEventListener('keydown', keydown)
    return () => {
      cancelAnimationFrame(initial)
      document.removeEventListener('keydown', keydown)
      if (opener?.isConnected) opener.focus({preventScroll:true})
    }
  }, [])
  return dialog
}
