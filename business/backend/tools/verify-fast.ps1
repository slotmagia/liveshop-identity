$ErrorActionPreference = 'Stop'
$backend = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$env:GOWORK = 'off'
go -C $backend mod tidy
if ($LASTEXITCODE -ne 0) { throw 'go mod tidy failed' }
# gofmt -l reports without rewriting. `go fmt` would fix the very thing it is
# meant to detect, so a failure would disappear on the next run and the check
# would silently dirty the working tree.
$unformatted = gofmt -l $backend
if ($LASTEXITCODE -ne 0) { throw 'gofmt failed' }
if ($unformatted) { throw "unformatted files:`n$($unformatted -join "`n")" }
go -C $backend vet ./...
if ($LASTEXITCODE -ne 0) { throw 'go vet failed' }
# The layering gate runs before the tests: a boundary violation is a design
# defect, and letting the suite pass first hides it behind a green run.
go -C $backend run ./cmd/archcheck
if ($LASTEXITCODE -ne 0) { throw 'archcheck failed' }
go -C $backend test ./...
if ($LASTEXITCODE -ne 0) { throw 'go test failed' }
Write-Output 'Fast verification passed.'
