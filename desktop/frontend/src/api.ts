export * from '../wailsjs/go/main/App'
export {backend, serversetup} from '../wailsjs/go/models'
export {EventsOn, EventsOff, BrowserOpenURL} from '../wailsjs/runtime/runtime'

export const OBF_PROFILES = ['none', 'rtpopus', 'rtpopus2', 'rtpopus3'] as const
export const TRANSPORTS = ['tcp', 'udp'] as const
export const MODES = ['udp', 'tcp'] as const
export const DNS_MODES = ['auto', 'plain', 'doh'] as const

// Kept here until native Wails regeneration; avoids editing existing generated work.
export interface ContainerInstance { id: string; name: string; image: string; running: boolean; supported: boolean }
export const SetupAttachContainer = (cfg: import('../wailsjs/go/models').serversetup.SSHConfig, id: string, name: string): Promise<import('../wailsjs/go/models').backend.Server> =>
  (window as any).go.main.App.SetupAttachContainer(cfg, id, name)
export const SetupCreateContainer = (cfg: import('../wailsjs/go/models').serversetup.SSHConfig, draft: import('../wailsjs/go/models').backend.ServerSetupDraft, wgPort: number, name: string): Promise<import('../wailsjs/go/models').backend.Server> =>
  (window as any).go.main.App.SetupCreateContainer(cfg, draft, wgPort, name)
export interface ContainerPreview {
  server: import('../wailsjs/go/models').backend.Server;
  advertisedPeer: string;
  sshPeer: string;
  needsReview: boolean;
}
export const SetupPrepareContainer = (cfg: import('../wailsjs/go/models').serversetup.SSHConfig, id: string, name: string): Promise<ContainerPreview> =>
  (window as any).go.main.App.SetupPrepareContainer(cfg, id, name)
