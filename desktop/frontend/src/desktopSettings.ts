import {desktop, type DesktopSettings} from './desktop.ts'

// Bootstrap, profile selection and the preferences screen need the same data.
// Reuse the completed snapshot (and any in-flight request) instead of asking
// Windows to query Task Scheduler again just to display the settings page.
export function createDesktopSettingsCache(loader: () => Promise<DesktopSettings>) {
  let cached: DesktopSettings | null = null
  let pending: Promise<DesktopSettings> | null = null
  let revision = 0
  const load = (refresh = false): Promise<DesktopSettings> => {
    if (!refresh && cached) return Promise.resolve(cached)
    if (!refresh && pending) return pending
    const current = ++revision
    const request = Promise.resolve().then(loader).then(settings => {
      if (current === revision) cached = settings
      return settings
    }).finally(() => {
      if (pending === request) pending = null
    })
    pending = request
    return request
  }
  return {snapshot: () => cached, load}
}

export const desktopSettings = createDesktopSettingsCache(() => desktop().DesktopSettings())
