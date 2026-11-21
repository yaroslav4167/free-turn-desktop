// Package assets contains bundled branding independent of installation paths.
// Android-derived artwork attribution and licensing: docs/branding.md.
package assets

import _ "embed"

//go:embed icon.svg
var IconSVG []byte

//go:embed icon.png
var IconPNG []byte

//go:embed icon-64.png
var Icon64PNG []byte

//go:embed interface.svg
var InterfaceSVG []byte

//go:embed tray-dark-64.png
var TrayIconPNG []byte

//go:embed tray-light-64.png
var TrayLightIconPNG []byte

//go:embed tray-dark.ico
var TrayIconICO []byte

//go:embed tray-light.ico
var TrayLightIconICO []byte
