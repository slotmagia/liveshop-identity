param(
  [switch]$Fresh,
  [switch]$Register
)

$ErrorActionPreference = 'Stop'
if ($PSVersionTable.PSVersion.Major -lt 7) {
  throw "This deployment requires PowerShell 7. Run: pwsh -File $PSCommandPath"
}

$tools = $PSScriptRoot
$compose = [IO.Path]::GetFullPath((Join-Path $tools '..\deploy\compose.local.yml'))

function Invoke-Native {
  param([Parameter(Mandatory)][scriptblock]$Command, [string]$FailureMessage)
  $previous = $ErrorActionPreference
  $ErrorActionPreference = 'Continue'
  try { & $Command } finally { $ErrorActionPreference = $previous }
  if ($LASTEXITCODE -ne 0 -and $FailureMessage) { throw $FailureMessage }
}

function Ensure-LocalNetwork {
  $network = Invoke-Native { docker network ls --filter name='^liveshop-local$' --format '{{.Name}}' }
  if ($network -ne 'liveshop-local') {
    Invoke-Native { docker network create liveshop-local | Out-Null } 'Failed to create the shared Docker network liveshop-local.'
  }
}

function Wait-Http([string]$Url, [int]$TimeoutMinutes = 5) {
  $deadline = [DateTime]::UtcNow.AddMinutes($TimeoutMinutes)
  while ([DateTime]::UtcNow -lt $deadline) {
    try {
      $response = Invoke-WebRequest -Uri $Url -TimeoutSec 3 -UseBasicParsing -SkipHttpErrorCheck
      if ($response.StatusCode -ge 200 -and $response.StatusCode -lt 500) { return }
    } catch {}
    Start-Sleep -Milliseconds 500
  }
  throw "Timed out waiting for $Url"
}

function Wait-Ready([string]$Url, [int]$TimeoutMinutes = 5) {
  $deadline = [DateTime]::UtcNow.AddMinutes($TimeoutMinutes)
  while ([DateTime]::UtcNow -lt $deadline) {
    try {
      $response = Invoke-WebRequest -Uri $Url -TimeoutSec 3 -UseBasicParsing -SkipHttpErrorCheck
      if ($response.StatusCode -ge 200 -and $response.StatusCode -lt 300) { return }
    } catch {}
    Start-Sleep -Milliseconds 500
  }
  throw "Timed out waiting for ready service $Url"
}

Ensure-LocalNetwork
$certs = Invoke-Native { docker volume ls -q --filter 'name=^liveshop-grpc-certs$' }
if ("$certs".Trim() -ne 'liveshop-grpc-certs') {
  throw 'Missing liveshop-grpc-certs. Start Registry containers first: liveshop-registry/business/backend/tools/up-local.ps1'
}
Wait-Ready 'http://127.0.0.1:18070/readyz'

if ($Fresh) {
  Invoke-Native { docker compose -f $compose down -v --remove-orphans } 'Failed to reset the local Identity stack.'
}

Invoke-Native { docker compose -f $compose up -d --build } 'Local Identity container deployment failed.'
foreach ($url in @(
  'http://127.0.0.1:18092/readyz',
  'http://127.0.0.1:15201',
  'http://127.0.0.1:15202',
  'http://127.0.0.1:15203/identity-shop.js',
  'http://127.0.0.1:15204/identity-live.js'
)) {
  if ($url.EndsWith('/readyz')) { Wait-Ready $url } else { Wait-Http $url }
}

if ($Register) {
  & (Join-Path $tools 'register-local.ps1') `
    -PlatformUrl 'http://127.0.0.1:18070' `
    -BackendOrigin 'http://identity:18092' `
    -GRPCEndpoint 'dns:///identity:19092' `
    -AdminArtifactUrl 'http://127.0.0.1:15201' `
    -MerchArtifactUrl 'http://127.0.0.1:15202' `
    -ShopArtifactUrl 'http://127.0.0.1:15203/identity-shop.js' `
    -LiveArtifactUrl 'http://127.0.0.1:15204/identity-live.js'
}

Invoke-Native { docker compose -f $compose ps }
Write-Host 'Identity local containers are running: http://127.0.0.1:18092'
