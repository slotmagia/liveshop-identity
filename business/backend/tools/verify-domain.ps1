# Fails while the domain contracts are still the generated worksheet. A fresh
# skeleton is legitimately empty, so verify-fast stays green; this gate is what
# must pass before the module implements a capability or ships.
$ErrorActionPreference = 'Stop'
$domain = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../docs/domain'))

# TODO(domain) marks a document nobody wrote; <!-- EXAMPLE --> marks one that
# was read but never replaced. Both mean the same thing: no decision was made.
# Both markers are ASCII so this check does not depend on the file encoding.
$markers = @('TODO(domain)', '<!-- EXAMPLE -->')

$pending = @()
foreach ($document in Get-ChildItem -LiteralPath $domain -Filter *.md) {
    $content = Get-Content -LiteralPath $document.FullName -Raw
    foreach ($marker in $markers) {
        if ($content.Contains($marker)) {
            $pending += "  $($document.Name): still contains $marker"
        }
    }
}

if ($pending.Count -gt 0) {
    Write-Output 'Domain contracts are incomplete:'
    $pending | ForEach-Object { Write-Output $_ }
    Write-Output ''
    Write-Output 'Facts, invariants, state machines, transactions and external contracts are'
    Write-Output 'inputs to the implementation, not documentation written after it.'
    throw 'domain contracts are incomplete'
}

Write-Output 'Domain contracts are complete.'
