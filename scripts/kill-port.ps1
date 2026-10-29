<#
.SYNOPSIS
    Kills whatever process is listening on a TCP port (Windows).

.DESCRIPTION
    Used by `make kill`. Finds the owning PID through Get-NetTCPConnection and
    falls back to parsing netstat when that cmdlet is unavailable.

.EXAMPLE
    powershell -NoProfile -ExecutionPolicy Bypass -File scripts/kill-port.ps1 -Port 8080
#>
param(
    [int]$Port = 8080
)

$ErrorActionPreference = 'SilentlyContinue'

$procIds = @()

$conns = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
if ($conns) {
    $procIds = $conns | Select-Object -ExpandProperty OwningProcess -Unique
}

if (-not $procIds) {
    $procIds = netstat -ano |
        Select-String -Pattern ":$Port\s" |
        ForEach-Object {
            $fields = ($_.ToString().Trim() -split '\s+')
            if ($fields.Length -ge 5 -and $fields[-2] -eq 'LISTENING' -and $fields[-1] -match '^\d+$') {
                [int]$fields[-1]
            }
        } |
        Sort-Object -Unique
}

$procIds = @($procIds | Where-Object { $_ -and $_ -ne 0 } | Sort-Object -Unique)

if ($procIds.Count -eq 0) {
    Write-Host "Port $Port is free (nothing to kill)."
    exit 0
}

foreach ($procId in $procIds) {
    $proc = Get-Process -Id $procId -ErrorAction SilentlyContinue
    $name = if ($proc) { $proc.ProcessName } else { 'unknown' }
    Write-Host "Killing $name (PID $procId) on port $Port..."
    Stop-Process -Id $procId -Force -ErrorAction SilentlyContinue
}

Start-Sleep -Milliseconds 300

$still = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
if ($still) {
    Write-Host "Port $Port still has a listener. Try an elevated terminal (Run as administrator)." -ForegroundColor Yellow
    exit 1
}

Write-Host "Port $Port is free."
exit 0
