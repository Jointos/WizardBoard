param(
    [Parameter(Mandatory = $true)]
    [string]$OldRuleset,

    [Parameter(Mandatory = $true)]
    [string]$NewRuleset,

    [string]$Timestamp = (Get-Date -Format 'yyyy-MM-dd_HH-mm-ss'),

    [switch]$Apply,
    [switch]$RemoveSource,
    [switch]$Force
)

$ErrorActionPreference = 'Stop'

if ($OldRuleset -eq $NewRuleset) {
    throw 'OldRuleset and NewRuleset must differ.'
}

$Root = (Resolve-Path (Join-Path $PSScriptRoot '../../../..')).Path
$aiDir = Join-Path $Root 'internal/ai'
$engineArchive = Join-Path $aiDir "archive/$OldRuleset"
$pipelineArchive = Join-Path $engineArchive 'pipeline'
$dataArchive = Join-Path $Root "data/archive/${OldRuleset}_${Timestamp}"
$manifestPath = Join-Path $engineArchive 'MANIFEST.md'
$dataManifestPath = Join-Path $dataArchive 'MANIFEST.md'

$engines = @(Get-ChildItem -Path $aiDir -Directory |
    Where-Object { $_.Name -match '^[a-z0-9]+_v[0-9]+$' } |
    Sort-Object Name)

$pipelineDirs = @(@('cmd/dataset', 'cmd/dataset_v2') | ForEach-Object {
    $p = Join-Path $Root $_
    if (Test-Path $p) { Get-Item $p }
})

if ($engines.Count -eq 0) {
    throw 'No old engine folders found under internal/ai.'
}

$dataTargets = @(
    'data/rankings.json',
    'data/arena_stats.json',
    'data/models',
    'data/reports',
    'data/balance',
    'data/datasets',
    'data/saves'
)

Write-Host "Ruleset archive: $OldRuleset -> $NewRuleset"
Write-Host "Repository:      $Root"
Write-Host "Code archive:    $engineArchive"
Write-Host "Data archive:    $dataArchive"
Write-Host ''
Write-Host 'Engine folders to zip:'
$engines | ForEach-Object { Write-Host "  - $($_.Name)" }
if ($pipelineDirs.Count -gt 0) {
    Write-Host 'Old pipeline commands to zip:'
    $pipelineDirs | ForEach-Object { Write-Host "  - $($_.Name)" }
}
Write-Host ''
Write-Host 'Data to move:'
foreach ($rel in $dataTargets) {
    $p = Join-Path $Root $rel
    if (Test-Path $p) {
        Write-Host "  - $rel"
    } else {
        Write-Host "  - $rel (missing, skipped)"
    }
}
Write-Host ''

if ($RemoveSource -and -not $Apply) {
    if (-not (Test-Path $engineArchive)) {
        throw "Code archive does not exist: $engineArchive. Run -Apply first."
    }
    $missing = @()
    foreach ($engine in $engines) {
        $zip = Join-Path $engineArchive "$($engine.Name).zip"
        if (-not (Test-Path $zip)) { $missing += $zip }
    }
    foreach ($dir in $pipelineDirs) {
        $zip = Join-Path $pipelineArchive "$($dir.Name).zip"
        if (-not (Test-Path $zip)) { $missing += $zip }
    }
    if ($missing.Count -gt 0) {
        throw "Archive is incomplete; missing: $($missing -join ', ')"
    }
    Write-Host 'Removing old sources (archive verified):' -ForegroundColor Yellow
    foreach ($engine in $engines) {
        Remove-Item -Recurse -Force $engine.FullName
        Write-Host "removed source: $($engine.Name)"
    }
    foreach ($dir in $pipelineDirs) {
        Remove-Item -Recurse -Force $dir.FullName
        Write-Host "removed pipeline source: $($dir.Name)"
    }
    Write-Host ''
    Write-Host 'Now update internal/ai/catalog/catalog.go, old cmd/dataset* commands, Makefile targets and defaults, then run go build/test.' -ForegroundColor Yellow
    exit 0
}

if (-not $Apply) {
    Write-Host 'DRY RUN: nothing was changed.' -ForegroundColor Yellow
    Write-Host 'Review the lists, then rerun with -Apply (after the new engines pass tests).'
    exit 0
}

if ((Test-Path $engineArchive) -and -not $Force) {
    throw "Code archive already exists: $engineArchive (use -Force to overwrite)."
}
if ((Test-Path $dataArchive) -and -not $Force) {
    throw "Data archive already exists: $dataArchive (use -Force to overwrite)."
}

New-Item -ItemType Directory -Force -Path $engineArchive | Out-Null
New-Item -ItemType Directory -Force -Path $dataArchive | Out-Null

$zipRows = @()
foreach ($engine in $engines) {
    $zipPath = Join-Path $engineArchive "$($engine.Name).zip"
    if (Test-Path $zipPath) {
        Remove-Item $zipPath -Force
    }
    Compress-Archive -Path $engine.FullName -DestinationPath $zipPath -CompressionLevel Optimal
    $hash = (Get-FileHash -Algorithm SHA256 $zipPath).Hash.ToLower()
    $zipRows += [pscustomobject]@{ Folder = $engine.Name; Zip = (Split-Path $zipPath -Leaf); SHA256 = $hash }
    Write-Host "zipped $($engine.Name) -> $zipPath"
}

$pipelineRows = @()
if ($pipelineDirs.Count -gt 0) {
    New-Item -ItemType Directory -Force -Path $pipelineArchive | Out-Null
    foreach ($dir in $pipelineDirs) {
        $zipPath = Join-Path $pipelineArchive "$($dir.Name).zip"
        if (Test-Path $zipPath) { Remove-Item $zipPath -Force }
        Compress-Archive -Path $dir.FullName -DestinationPath $zipPath -CompressionLevel Optimal
        $hash = (Get-FileHash -Algorithm SHA256 $zipPath).Hash.ToLower()
        $pipelineRows += [pscustomobject]@{ Folder = $dir.Name; Zip = (Split-Path $zipPath -Leaf); SHA256 = $hash }
        Write-Host "zipped $($dir.Name) -> $zipPath"
    }
}

$moved = @()
foreach ($rel in $dataTargets) {
    $src = Join-Path $Root $rel
    if (-not (Test-Path $src)) { continue }
    $name = Split-Path $src -Leaf
    $dst = Join-Path $dataArchive $name
    Move-Item -Path $src -Destination $dst -Force
    $moved += $rel
    Write-Host "moved $rel -> $dst"
}

# Keep the active save/dataset roots available for the new ruleset.
New-Item -ItemType Directory -Force -Path (Join-Path $Root 'data/saves') | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $Root 'data/datasets') | Out-Null

$rulesHash = ''
$rulesPath = Join-Path $Root 'internal/game/rules.go'
if (Test-Path $rulesPath) {
    $rulesHash = (Get-FileHash -Algorithm SHA256 $rulesPath).Hash.ToLower()
}

$engineLines = $zipRows | ForEach-Object {
    "| ``$($_.Folder)`` | ``$($_.Zip)`` | ``$($_.SHA256)`` |"
}
$pipelineLines = $pipelineRows | ForEach-Object {
    "| ``$($_.Folder)`` | ``$($_.Zip)`` | ``$($_.SHA256)`` |"
}
$dataLines = $moved | ForEach-Object { "| ``$_`` | archived |" }

$manifest = @"
# Ruleset archive manifest

- **Archived at:** $Timestamp
- **Old ruleset:** $OldRuleset
- **New ruleset:** $NewRuleset
- **Rules file hash:** ``$rulesHash``

## Code archive

| Engine folder | Zip | SHA256 |
|---|---|---|
$($engineLines -join "`n")

Each zip contains the whole original engine folder (Go, tests, README and
``weights.json`` where applicable).

## Old pipeline archive

| Command folder | Zip | SHA256 |
|---|---|---|
$($pipelineLines -join "`n")

These are the old dataset commands that imported the archived ML packages. Keep
them for reference; the new ruleset gets a fresh ``cmd/dataset_<ruleset>``.

## Data archive

| Path | Status |
|---|---|
$($dataLines -join "`n")

## New engines expected

- ``alphabeta-$NewRuleset-v1``
- ``ml-$NewRuleset-v1`` (additional ML versions only on explicit request)

## Cleanup checklist

- [ ] old engine zips verified
- [ ] ML zip contains weights.json
- [ ] old pipeline zips verified
- [ ] old data/saves moved
- [ ] catalog imports removed
- [ ] old dataset commands/Makefile targets retired
- [ ] defaults/docs updated
- [ ] new league reset to 1200
- [ ] go build ./...
- [ ] go test ./...
- [ ] server/web healthy
"@

$utf8 = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($manifestPath, $manifest, $utf8)
[System.IO.File]::WriteAllText($dataManifestPath, $manifest, $utf8)
Write-Host "manifest: $manifestPath"
Write-Host "manifest: $dataManifestPath"

if ($RemoveSource) {
    foreach ($engine in $engines) {
        Remove-Item -Recurse -Force $engine.FullName
        Write-Host "removed source: $($engine.Name)"
    }
    foreach ($dir in $pipelineDirs) {
        Remove-Item -Recurse -Force $dir.FullName
        Write-Host "removed pipeline source: $($dir.Name)"
    }
    Write-Host ''
    Write-Host 'Now update internal/ai/catalog/catalog.go, old cmd/dataset* commands, Makefile targets and defaults, then run go build/test.' -ForegroundColor Yellow
} else {
    Write-Host ''
    Write-Host 'Source folders kept. Update the catalog/defaults, then rerun with -Apply -RemoveSource to delete them.' -ForegroundColor Yellow
}
