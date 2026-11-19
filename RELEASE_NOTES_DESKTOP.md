# FreeTurn Desktop v3.0.0

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

Older repositories publish a final bridge update: Linux v2.6.6, Windows v1.1.4,
and macOS v0.2.3. Install that update once to switch automatic updates here.
