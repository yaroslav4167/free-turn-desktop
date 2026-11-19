# Shared Android/desktop server

The SSH setup wizard checks Docker (including stopped containers) and systemd
before offering installation. Failure to query Docker aborts setup; it is not
treated as an empty host.

Supported reuse is deliberately limited to the official hackdiaz image named
`free-turn-proxy`, host networking, the canonical `/opt/free-turn-proxy/auth`
mount and root-owned, non-writable-by-others `install.sh`/`install.conf` files.
Other matching containers are displayed but never adopted or overwritten.
Select the installation explicitly when several are found.

Reusing a running installation calls its existing protocol-3 `client-add` and
imports its complete official link. Every desktop gets a distinct generated
`desktop-…` name, WireGuard key/address and Client ID. Retrying the same device
name retrieves its configuration rather than creating a duplicate. Server
settings, image and other users are not changed or restarted during attachment.

Container-backed profiles keep their management type in saved SSH settings.
Status, logs, explicit stop/restart and client management target Docker/the
native controller, never the legacy desktop systemd start/reconfigure path.
Removal of the shared installation is deliberately refused from the desktop.
Native controller user names currently require ASCII letters, digits, `.`, `_`
and `-` (up to 32 characters). Unsupported/stopped installations can still be
used by importing an existing link; they are not started implicitly.

On an empty host the wizard creates a Docker installation compatible with
Android, not a parallel legacy service. The v4.2.1 installer is fetched from
immutable commit `11f81346cb398b157975107872aad5d6a4143149`, SHA-256 verified
before execution, and checked again for an existing installation on the VPS.
No failed operation falls back to installing a second server. If installation
is interrupted, re-probe the VPS instead of blindly rerunning installation.

Do not use upstream `apply`/update to manage multiple installations sharing the
same service/container names and prefix: it removes the other runtime. A single
shared installation avoids that collision. Existing deployments are not migrated
or deleted automatically.

Local SSH fixture tests are not a substitute for a native desktop build or a
live provisioning/DTLS/WireGuard test. Read-only detection can be validated on a
production VPS; new installation and client creation should first be tested on
an explicitly designated disposable host.
