// Local preparation (saving, resolving routes, elevation/helper setup) starts
// before the backend necessarily publishes a connecting snapshot.
export function serverDetailState(serverId: string, status: {serverId?: string; state: string}, pendingServerId: string | null) {
  const isActive = status.serverId === serverId
  const preparing = pendingServerId === serverId
  const otherPreparing = pendingServerId !== null && !preparing
  const lifecycle = isActive ? status.state : 'idle'
  const phase = preparing && lifecycle !== 'captcha' ? 'connecting' : lifecycle
  const busy = phase === 'connecting'
  const progressing = preparing || (isActive && (phase === 'connecting' || phase === 'captcha'))
  const connected = isActive && phase === 'connected' && !preparing
  return {phase, busy, progressing, connected, otherPreparing}
}
