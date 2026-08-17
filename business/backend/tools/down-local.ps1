param([switch]$Volumes)

$ErrorActionPreference = 'Stop'
$compose = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\deploy\compose.local.yml'))
$args = @('-f', $compose, 'down', '--remove-orphans')
if ($Volumes) { $args += '-v' }
$previous = $ErrorActionPreference
$ErrorActionPreference = 'Continue'
try { docker compose @args } finally { $ErrorActionPreference = $previous }
if ($LASTEXITCODE -ne 0) { throw 'Failed to stop the local Identity containers.' }
if ($Volumes) {
  Write-Output 'Identity containers and named volumes were removed.'
} else {
  Write-Output 'Identity containers stopped. Named volumes were preserved.'
}
