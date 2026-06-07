param(
  [string]$StackDir = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
)

$ErrorActionPreference = 'Stop'

$envPath = Join-Path $StackDir '.env'
if (-not (Test-Path -LiteralPath $envPath)) {
  Copy-Item -LiteralPath (Join-Path $StackDir '.env.example') -Destination $envPath
}

$runtimeDirs = @(
  'runtime/new-api/data',
  'runtime/new-api/logs',
  'runtime/redis',
  'runtime/cliproxyapi/auths',
  'runtime/cliproxyapi/logs',
  'runtime/caddy/data',
  'runtime/caddy/config'
)

foreach ($dir in $runtimeDirs) {
  New-Item -ItemType Directory -Force -Path (Join-Path $StackDir $dir) | Out-Null
}

$configPath = Join-Path $StackDir 'runtime/cliproxyapi/config.yaml'
if (-not (Test-Path -LiteralPath $configPath)) {
  $envPath = Join-Path $StackDir '.env'
  $envValues = @{}
  Get-Content -LiteralPath $envPath | ForEach-Object {
    if ($_ -match '^\s*#' -or $_ -notmatch '=') { return }
    $key, $value = $_ -split '=', 2
    $envValues[$key.Trim()] = $value.Trim()
  }
  $content = Get-Content -Raw -LiteralPath (Join-Path $StackDir 'cliproxyapi.config.example.yaml')
  $content = $content.Replace('${CLIPROXYAPI_MANAGEMENT_KEY}', [string]$envValues['CLIPROXYAPI_MANAGEMENT_KEY'])
  $content = $content.Replace('${CLIPROXYAPI_API_KEY}', [string]$envValues['CLIPROXYAPI_API_KEY'])
  Set-Content -LiteralPath $configPath -Value $content -Encoding UTF8
}

Write-Output "Runtime initialized at $StackDir"
Write-Output "Edit $envPath and runtime/cliproxyapi/config.yaml before first deploy."
