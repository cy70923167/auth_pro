param(
  [string]$Version = "1.0.0"
)

$ErrorActionPreference = 'Stop'

$Root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$FrontendDir = Join-Path $Root 'frontend'
$BackendDir = Join-Path $Root 'backend'
$DistName = "auth_pro-full-v$Version"
$ReleaseRoot = Join-Path $Root 'release'
$PackageDir = Join-Path $ReleaseRoot $DistName
$PackagesDir = Join-Path $ReleaseRoot 'packages'
$PackagePath = Join-Path $PackagesDir "$DistName.tar.gz"
$ChecksumPath = "$PackagePath.sha256"
$LatestPath = Join-Path $PackagesDir 'latest.json'
$ReleasesPath = Join-Path $PackagesDir 'releases.json'
$UpdatePackageBaseUrl = if ($env:AUTO_PRO_UPDATE_PACKAGE_BASE_URL) { $env:AUTO_PRO_UPDATE_PACKAGE_BASE_URL } else { 'https://e.91ani.cn/packages' }
$UpdateReleasesUrl = if ($env:AUTO_PRO_UPDATE_RELEASES_URL) { $env:AUTO_PRO_UPDATE_RELEASES_URL } else { 'https://e.91ani.cn/releases.json' }
$UpdateManifestUrl = if ($env:AUTO_PRO_UPDATE_MANIFEST_URL) { $env:AUTO_PRO_UPDATE_MANIFEST_URL } else { 'https://e.91ani.cn/latest.json' }

if ($PackageDir -notlike "$Root*") {
  throw "Invalid package directory: $PackageDir"
}
if (Test-Path $PackageDir) {
  Remove-Item -LiteralPath $PackageDir -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $PackageDir, $PackagesDir | Out-Null

Write-Host "[1/5] Building frontend..."
Push-Location $FrontendDir
$PreviousViteVersion = $env:VITE_VERSION
try {
  $env:VITE_VERSION = $Version
  pnpm run build
} finally {
  if ($null -eq $PreviousViteVersion) {
    Remove-Item Env:VITE_VERSION -ErrorAction SilentlyContinue
  } else {
    $env:VITE_VERSION = $PreviousViteVersion
  }
  Pop-Location
}

$FrontendDist = Join-Path $FrontendDir 'dist'
if (-not (Test-Path (Join-Path $FrontendDist 'index.html'))) {
  throw "Frontend dist/index.html not found"
}
if (-not (Test-Path (Join-Path $FrontendDist 'version.json'))) {
  throw "Frontend dist/version.json not found"
}

Write-Host "[2/5] Preparing package directories..."
$PackageBackendDir = Join-Path $PackageDir 'backend'
New-Item -ItemType Directory -Force -Path $PackageBackendDir | Out-Null
Copy-Item -Path (Join-Path $FrontendDist '*') -Destination $PackageDir -Recurse -Force

$BackendStaticDir = Join-Path $BackendDir 'static'
if ($BackendStaticDir -notlike "$Root*") {
  throw "Invalid backend static directory: $BackendStaticDir"
}
New-Item -ItemType Directory -Force -Path $BackendStaticDir | Out-Null
Get-ChildItem -LiteralPath $BackendStaticDir -Force | Remove-Item -Recurse -Force
Copy-Item -Path (Join-Path $FrontendDist '*') -Destination $BackendStaticDir -Recurse -Force

Write-Host "[3/5] Building Linux backend..."
$BuildTime = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$BackendBinary = Join-Path $PackageBackendDir 'auth_pro'
Push-Location $BackendDir
try {
  $env:GOOS = 'linux'
  $env:GOARCH = 'amd64'
  $env:CGO_ENABLED = '0'
  go build -trimpath -ldflags "-s -w -X auto_pro/config.AppVersion=$Version -X auto_pro/config.BuildTime=$BuildTime -X auto_pro/config.DefaultUpdateManifestURL=$UpdateManifestUrl" -o $BackendBinary .
} finally {
  Remove-Item Env:GOOS -ErrorAction SilentlyContinue
  Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
  Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
  Pop-Location
}

Write-Host "[4/5] Writing manifest..."
$Manifest = @{
  version = $Version
  frontendDir = '.'
  backendFile = 'backend/auth_pro'
  requiredFiles = @()
} | ConvertTo-Json -Depth 4
$ManifestPath = Join-Path $PackageDir 'manifest.json'
$Utf8NoBom = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($ManifestPath, $Manifest + [Environment]::NewLine, $Utf8NoBom)

Write-Host "[5/5] Creating tar.gz package and latest.json..."
if (Test-Path $PackagePath) {
  Remove-Item -LiteralPath $PackagePath -Force
}
tar -czf $PackagePath -C $PackageDir .

$Hash = (Get-FileHash -LiteralPath $PackagePath -Algorithm SHA256).Hash.ToLower()
$PackageFileName = [System.IO.Path]::GetFileName($PackagePath)
[System.IO.File]::WriteAllText($ChecksumPath, "$Hash  $PackageFileName" + [Environment]::NewLine, $Utf8NoBom)
$Size = (Get-Item -LiteralPath $PackagePath).Length
$PackageUrl = "$($UpdatePackageBaseUrl.TrimEnd('/'))/$DistName.tar.gz"
node (Join-Path $Root 'scripts/write-release-manifests.mjs') `
  $LatestPath `
  $ReleasesPath `
  $Version `
  $BuildTime `
  "$DistName.tar.gz" `
  $PackageUrl `
  $Hash `
  $Size `
  $UpdateReleasesUrl
if ($LASTEXITCODE -ne 0) {
  throw "Failed to write release manifests"
}

Write-Host ""
Write-Host "Release package: $PackagePath"
Write-Host "Checksum file: $ChecksumPath"
Write-Host "Latest manifest: $LatestPath"
Write-Host "Release history: $ReleasesPath"
Write-Host "SHA256: $Hash"
Write-Host "Size: $Size bytes"
