# Builds static tooltrack binaries into dist\ without needing make.
# Usage: .\scripts\build.ps1 [-Targets linux-amd64,windows-amd64,darwin-amd64,darwin-arm64]
param(
    [string[]]$Targets = @('linux-amd64', 'windows-amd64', 'darwin-amd64', 'darwin-arm64')
)

$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

$AppName = 'tooltrack'
$DistDir = 'dist'
$Valid = @('linux-amd64', 'windows-amd64', 'darwin-amd64', 'darwin-arm64')

$Version = $env:VERSION
if (-not $Version) {
    $Version = (git describe --tags --always --dirty 2>$null)
    if (-not $Version) { $Version = 'dev' }
}

New-Item -ItemType Directory -Force -Path $DistDir | Out-Null

$saved = @{ CGO = $env:CGO_ENABLED; OS = $env:GOOS; ARCH = $env:GOARCH }
try {
    foreach ($target in $Targets) {
        if ($Valid -notcontains $target) {
            throw "unknown target: $target (valid: $($Valid -join ', '))"
        }
        $goos, $goarch = $target -split '-', 2
        $out = Join-Path $DistDir "$AppName-$target"
        if ($goos -eq 'windows') { $out += '.exe' }

        Write-Host "building $out ($Version)"
        $env:CGO_ENABLED = '0'
        $env:GOOS = $goos
        $env:GOARCH = $goarch
        go build -ldflags "-s -w -X 'main.Version=$Version'" -o $out ./cmd/cli
        if ($LASTEXITCODE -ne 0) { throw "go build failed for $target" }
    }
}
finally {
    $env:CGO_ENABLED = $saved.CGO
    $env:GOOS = $saved.OS
    $env:GOARCH = $saved.ARCH
}
