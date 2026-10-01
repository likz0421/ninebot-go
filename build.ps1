# build.ps1 — cross-compile ninecli for all supported platforms.
# Usage: .\build.ps1            # build all platforms into dist\
#        .\build.ps1 current    # build only for the current platform
param(
    [string]$Target = "all"
)

$ErrorActionPreference = "Stop"
$root = $PSScriptRoot
$dist = Join-Path $root "dist"
New-Item -ItemType Directory -Path $dist -Force | Out-Null

# Keep module/build caches inside the project to avoid polluting the user profile.
$env:GOCACHE = Join-Path $env:TEMP "ninecli-gocache"
$env:GOPROXY = "https://goproxy.cn,direct"

$targets = @(
    @{ OS = "windows"; ARCH = "amd64"; EXT = ".exe" },
    @{ OS = "windows"; ARCH = "arm64"; EXT = ".exe" },
    @{ OS = "linux";   ARCH = "amd64"; EXT = "" },
    @{ OS = "linux";   ARCH = "arm64"; EXT = "" },
    @{ OS = "darwin";  ARCH = "amd64"; EXT = "" },
    @{ OS = "darwin";  ARCH = "arm64"; EXT = "" }
)

if ($Target -eq "current") {
    $goos = $env:GOOS; if (-not $goos) { $goos = "windows" }
    $goarch = $env:GOARCH; if (-not $goarch) { $goarch = "amd64" }
    $targets = @(@{ OS = $goos; ARCH = $goarch; EXT = if ($goos -eq "windows") { ".exe" } else { "" } })
}

foreach ($t in $targets) {
    $env:GOOS = $t.OS
    $env:GOARCH = $t.ARCH
    $env:CGO_ENABLED = "0"
    $out = Join-Path $dist ("ninecli-{0}-{1}{2}" -f $t.OS, $t.ARCH, $t.EXT)
    Write-Host "building $out"
    go build -trimpath -ldflags "-s -w" -o $out .
    if ($LASTEXITCODE -ne 0) { throw "build failed for $($t.OS)/$($t.ARCH)" }
}

Write-Host "done -> $dist"
