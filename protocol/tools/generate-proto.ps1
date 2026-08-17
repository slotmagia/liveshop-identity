$ErrorActionPreference = 'Stop'
# The protocol module owns its own generation; it is a sibling of business.
$protocol = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))

go -C $protocol run github.com/bufbuild/buf/cmd/buf@v1.47.2 lint
if ($LASTEXITCODE -ne 0) { throw 'Identity Proto lint failed.' }

go -C $protocol run github.com/bufbuild/buf/cmd/buf@v1.47.2 generate
if ($LASTEXITCODE -ne 0) { throw 'Identity Proto generation failed.' }

Write-Output 'Identity Proto contracts generated.'
