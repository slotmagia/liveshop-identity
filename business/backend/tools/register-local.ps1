param(
  [string]$PlatformUrl = 'http://127.0.0.1:18082',
  [string]$BackendOrigin = 'http://identity:18092',
  [string]$GRPCEndpoint = 'dns:///identity:19092',
  [string]$AdminArtifactUrl = 'http://127.0.0.1:15201',
  [string]$MerchArtifactUrl = 'http://127.0.0.1:15202',
  [string]$ShopArtifactUrl = 'http://127.0.0.1:15203/identity-shop.js',
  [string]$LiveArtifactUrl = 'http://127.0.0.1:15204/identity-live.js'
)
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$manifest = [IO.File]::ReadAllText((Join-Path $root 'module.json'), [Text.Encoding]::UTF8) | ConvertFrom-Json
$manifest.spec.backend.origin = $BackendOrigin
$manifest.spec.backend.grpc.endpoint = $GRPCEndpoint

function Get-ArtifactDigest([string]$Uri) {
  $artifact = Invoke-WebRequest -Uri $Uri -TimeoutSec 10
  $bytes = [Text.Encoding]::UTF8.GetBytes([string]$artifact.Content)
  $sha256 = [Security.Cryptography.SHA256]::Create()
  try { $hash = $sha256.ComputeHash($bytes) } finally { $sha256.Dispose() }
  return 'sha256:' + (($hash | ForEach-Object { $_.ToString('x2') }) -join '')
}

foreach ($contribution in $manifest.spec.contributions) {
  if ($contribution.surface -eq 'admin') { $contribution.artifact.entry = $AdminArtifactUrl; $contribution.artifact.integrity = Get-ArtifactDigest $AdminArtifactUrl }
  if ($contribution.surface -eq 'merch') { $contribution.artifact.entry = $MerchArtifactUrl; $contribution.artifact.integrity = Get-ArtifactDigest $MerchArtifactUrl }
  if ($contribution.surface -eq 'shop') { $contribution.artifact.entry = $ShopArtifactUrl; $contribution.artifact.integrity = Get-ArtifactDigest $ShopArtifactUrl }
  if ($contribution.surface -eq 'live') { $contribution.artifact.entry = $LiveArtifactUrl; $contribution.artifact.integrity = Get-ArtifactDigest $LiveArtifactUrl }
}

$env:WORKLOAD_PRIVATE_KEY = 'MEdxJQh5ZzEe9NhL8TQ7G5rCqZ1Cr00n6DVMiCayO_8'
$env:WORKLOAD_KEY_ID = 'ci-workload-dev-1'
$env:WORKLOAD_ISSUER = 'liveshop-workload-identity'
$env:WORKLOAD_SUBJECT = 'module-release-ci'
$env:WORKLOAD_AUDIENCE = 'liveshop-platform-internal'
$kernelRoot = [IO.Path]::GetFullPath((Join-Path $root '..\..\kernel-go'))
$token = & go -C $kernelRoot run ./cmd/workloadtoken
if ($LASTEXITCODE -ne 0 -or -not $token) { throw 'failed to issue local module release identity' }
$headers = @{Authorization="Bearer $token"}
. (Join-Path $PSScriptRoot '..\..\..\..\register-local-release.ps1')
Publish-LocalModuleRelease -PlatformUrl $PlatformUrl -ModuleId 'identity' -Manifest $manifest -Headers $headers
