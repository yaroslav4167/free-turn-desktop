# FreeTurn Desktop v3.0.2

## Safer existing-container setup

- Matching link/SSH hosts now save immediately, without an extra choice.
  Domain-name case/trailing dots and equivalent IP literals are recognized.
- Review differing relay endpoints before saving a local profile. Stale
  `PUBLIC_HOST` values in a VPS configuration no longer get selected silently.
- Choose the address from the server link, the SSH host with the relay port,
  or enter a custom endpoint. For a mismatch, saving is disabled until an
  address is selected. No DNS lookups are used to guess equivalence.
- Different SSH/public addresses can be intentional (NAT or bastion); the
  client does not overwrite them or rewrite server configuration automatically.
- SSH setup now explicitly accepts a domain and recommends DNS names for
  servers whose IP can change. Existing saved profiles are not rewritten.
- Imported keys, Client ID, AmneziaWG settings and DTLS pinning are preserved.

For an already saved profile with an old IP, edit only its relay `peer` to
the correct domain and port, save, and reconnect. This desktop update does not
restart the VPS, change WireGuard keys, or regenerate users.

## Unified desktop release

One repository and release stream for Linux (amd64), Windows (amd64), and
macOS (Apple Silicon and Intel): https://github.com/yaroslav4167/free-turn-desktop.

- Shared UI and core, with native Linux VPN helper, Windows Wintun, and macOS
  utun/launchd helper preserved.
- Automatic updates select only the current platform/architecture and verify
  the archive against the combined SHA-256 manifest.
- Existing profiles and settings retain their platform-specific storage paths.
- SSH setup detects supported hackdiaz Docker servers and issues a separate
  desktop device profile without reinstalling or restarting the shared server.
- New empty VPS deployments use the shared Android-compatible Docker layout.
- TURN readiness, DTLS stream accounting, packet-buffer bounds and imported
  AmneziaWG parameters are preserved by the shared core.

Choose the archive matching your OS. macOS builds are ad-hoc signed, not
Developer ID notarized. Releases require all four native build/test jobs to
pass; this does not certify a live VPN connection on every user's network.

## Direct upgrade from the old repositories

The old repositories distribute the verified v3.0.0 snapshots, which already
point automatic updates here. Clients on v3.0.0/3.0.1 can install v3.0.2 directly
from this repository. Profiles and preferences remain in their existing
locations. Older v2.6.6/v1.1.4/v0.2.3 bridge downloads are not required.
