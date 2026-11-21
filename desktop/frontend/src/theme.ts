export type Theme = 'system' | 'light' | 'dark'

const storageKey = 'freeturn:theme'
const listeners = new Set<() => void>()
let preference: Theme = 'system'
let media: MediaQueryList | undefined
let initialized = false

export function normalizeTheme(value: unknown): Theme {
  return value === 'light' || value === 'dark' ? value : 'system'
}

export function resolveTheme(value: Theme, systemDark: boolean): 'light' | 'dark' {
  return value === 'system' ? systemDark ? 'dark' : 'light' : value
}

function apply() {
  document.documentElement.dataset.theme = resolveTheme(preference, media?.matches ?? false)
}

// The local cache is only for first paint; desktop.json remains authoritative.
// Keep one OS listener for the application lifetime, not one per settings mount.
export function initializeTheme() {
  if (initialized) return
  initialized = true
  try { preference = normalizeTheme(window.localStorage.getItem(storageKey)) } catch { /* storage can be unavailable */ }
  if (typeof window.matchMedia === 'function') {
    media = window.matchMedia('(prefers-color-scheme: dark)')
    const changed = () => { if (preference === 'system') apply() }
    if (media.addEventListener) media.addEventListener('change', changed)
    else media.addListener(changed)
  }
  apply()
}

export function configureTheme(value: unknown) {
  initializeTheme()
  preference = normalizeTheme(value)
  try { window.localStorage.setItem(storageKey, preference) } catch { /* backend preference is still saved */ }
  apply()
  listeners.forEach(listener => listener())
}

export function subscribeTheme(listener: () => void) {
  listeners.add(listener)
  return () => { listeners.delete(listener) }
}

export function themeSnapshot() { return preference }
