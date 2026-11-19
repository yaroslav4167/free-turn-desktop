$ErrorActionPreference = 'Stop'
$root = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
Set-Location $root
$version = if ($env:RELEASE_TAG) { $env:RELEASE_TAG } else { 'v3.0.0' }
$repository = if ($env:UPDATE_REPOSITORY) { $env:UPDATE_REPOSITORY } else { 'yaroslav4167/free-turn-desktop' }
if ($version -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+$') { throw 'Invalid release tag' }
if ($repository -notmatch '^[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+$') { throw 'Invalid update repository' }
go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
if ($LASTEXITCODE) { throw 'Wails installation failed' }
npm --prefix desktop/frontend ci
if ($LASTEXITCODE) { throw 'npm ci failed' }
$config = Get-Content desktop/wails.json -Raw | ConvertFrom-Json
$config.info.productVersion = $version.TrimStart('v')
$config | ConvertTo-Json -Depth 20 | Set-Content desktop/wails.json -Encoding utf8NoBOM
Push-Location desktop
try {
    wails build -ldflags "-checklinkname=0 -s -w -X main.desktopVersion=$version -X main.updateRepository=$repository" -trimpath
    if ($LASTEXITCODE) { throw 'Wails build failed' }
} finally { Pop-Location }
New-Item -ItemType Directory -Force dist/package | Out-Null
Copy-Item desktop/build/bin/freeturn-desktop.exe dist/package/
Copy-Item scripts/windows/install.ps1,scripts/windows/uninstall.ps1 dist/package/
Copy-Item scripts/windows/Install.cmd,scripts/windows/Uninstall.cmd,scripts/windows/Run_Admin.bat dist/package/
Copy-Item README.md,LICENSE dist/package/
Copy-Item docs -Destination dist/package/docs -Recurse
$archive = "FreeTurn-Windows-amd64-$version.zip"
Compress-Archive -Path dist/package/* -DestinationPath "dist/$archive"
$hash = (Get-FileHash "dist/$archive" -Algorithm SHA256).Hash.ToLower()
"$hash  $archive" | Set-Content "dist/$archive.sha256" -Encoding utf8NoBOM
