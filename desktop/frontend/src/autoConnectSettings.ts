// A stored startup profile can outlive its deletion from the live profile list.
// Prefer an explicit valid choice, then the visible profile, then a saved entry.
// This only chooses a UI value; it never persists settings or connects a VPN.
export function resolveStartupProfile(servers: readonly {id: string}[], savedId: string, selectedId?: string): string {
  const exists = (id: string | undefined): id is string => !!id?.trim() && servers.some(server => server.id === id)
  if (exists(savedId)) return savedId
  if (exists(selectedId)) return selectedId
  return servers.find(server => server.id.trim())?.id || ''
}
