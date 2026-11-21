// -n is per link, not a global total (internal/session/session.go).
// multi.Provider assigns each link its own 1-based stream IDs; the VK cache
// groups those IDs independently by streamsPerCred.
export const MOBILE_STREAM_MAX = 128
export const MOBILE_ACCOUNT_MAX = 50

export interface StreamInput {
  n?: number
  streamsPerCred?: number
  vkLinks?: readonly string[]
}

export interface StreamCeilings { n: number; perAccount: number }

function positive(value: number | undefined, fallback: number): number {
  return Number.isFinite(value) && value! > 0 ? Math.max(1, Math.trunc(value!)) : fallback
}

// Match config.normalizeVKLinks, including raw call codes and comma-separated
// inputs. Do not trim/rewrite the textarea while the user is typing or dedupe
// entries which the core itself does not deduplicate.
export function configuredLinkCount(links: readonly string[] = []): number {
  return links.join(',').split(',').filter(item => {
    const value = item.trim()
    if (!value) return false
    const code = value.split('join/').at(-1)!.split(/[/?#]/, 1)[0]
    return code.length > 0
  }).length
}

export function streamSettings(input: StreamInput, ceilings?: StreamCeilings) {
  const n = positive(input.n, 12)
  const savedPerAccount = positive(input.streamsPerCred, 12)
  const perAccount = Math.min(n, savedPerAccount)
  const links = configuredLinkCount(input.vkLinks)
  return {
    n, perAccount, links, total: n * links,
    groupsPerLink: Math.ceil(n / perAccount),
    groups: Math.ceil(n / perAccount) * links,
    nMax: Math.max(MOBILE_STREAM_MAX, ceilings?.n ?? 0, n),
    perAccountMax: Math.min(n, Math.max(MOBILE_ACCOUNT_MAX, ceilings?.perAccount ?? 0, savedPerAccount)),
    extended: n > MOBILE_STREAM_MAX || perAccount > MOBILE_ACCOUNT_MAX,
  }
}

export function changeStreamCount(input: StreamInput, value: number, ceilings?: StreamCeilings) {
  const current = streamSettings(input, ceilings)
  const n = Math.min(current.nMax, positive(value, current.n))
  return {n, streamsPerCred: Math.min(n, current.perAccount)}
}

export function changeStreamsPerAccount(input: StreamInput, value: number, ceilings?: StreamCeilings) {
  const current = streamSettings(input, ceilings)
  return {n: current.n, streamsPerCred: Math.min(current.perAccountMax, positive(value, current.perAccount))}
}
