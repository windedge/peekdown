<#
.SYNOPSIS
Measures an app's memory once idle: its own process and its descendants.

.DESCRIPTION
Starts Exe, waits IdleSec for it to settle, then samples the private working
set (what Task Manager shows as Memory, the figure MyGo's internal/idlemem
reports) of the app, of its descendants, and of both together. An app with a
webview runs WebView2's processes, which count in the total; an app of native
UI has none. The app's peak working set over its life is reported too.

Runs samples the median of Runs runs.

.PARAMETER Exe
The executable to measure.

.PARAMETER AppArgs
Arguments for the app, as one line: a document path, say.

.PARAMETER Runs
Measured runs, 3 by default.

.PARAMETER IdleSec
Seconds an app runs before it is sampled, 6 by default.

.PARAMETER Json
Prints the result as JSON instead of a table.

.EXAMPLE
./Measure-Memory.ps1 -Exe C:/tmp/perf/bin/markdown-chat.exe -Runs 3
#>
param(
    [Parameter(Mandatory = $true)][string]$Exe,
    [string]$AppArgs = "",
    [int]$Runs = 3,
    [int]$IdleSec = 6,
    [switch]$Json
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot/PerfCommon.ps1"

$path = (Resolve-Path -LiteralPath $Exe).Path
$apps = New-Object System.Collections.Generic.List[double]
$totals = New-Object System.Collections.Generic.List[double]
$peaks = New-Object System.Collections.Generic.List[double]
$childCounts = New-Object System.Collections.Generic.List[int]
for ($i = 1; $i -le $Runs; $i++) {
    $p = Start-AppTree -Exe $path -AppArgs $AppArgs
    $title = ""
    $ms = [PerfWin32]::WaitFirstWindow($p.Id, 30000, [ref]$title)
    if ($ms -lt 0) {
        Stop-AppTree -Process $p
        throw "no window appeared within 30 s"
    }
    Start-Sleep -Seconds $IdleSec
    $app = [PerfWin32]::PrivateWorkingSet($p.Id)
    $children = [PerfWin32]::Descendants($p.Id)
    $total = $app
    foreach ($child in $children) {
        $ws = [PerfWin32]::PrivateWorkingSet($child)
        if ($ws -gt 0) { $total += $ws }
    }
    $peak = [PerfWin32]::PeakWorkingSet($p.Id)
    Stop-AppTree -Process $p
    $apps.Add($app / 1MB)
    $totals.Add($total / 1MB)
    $peaks.Add($peak / 1MB)
    $childCounts.Add($children.Count)
    Write-Host ("run {0}: app {1,6:N1} MB, {2,6:N1} MB with {3} descendant(s), peak working set {4,6:N1} MB" -f `
        $i, ($app / 1MB), ($total / 1MB), $children.Count, ($peak / 1MB))
    Start-Sleep -Milliseconds 500
}

$result = [ordered]@{
    exe               = $path
    exeBytes          = (Get-Item -LiteralPath $path).Length
    runs              = $Runs
    idleSeconds       = $IdleSec
    appMB             = [math]::Round((Get-Median -Values $apps.ToArray()), 1)
    totalMB           = [math]::Round((Get-Median -Values $totals.ToArray()), 1)
    peakWorkingSetMB  = [math]::Round((Get-Median -Values $peaks.ToArray()), 1)
    descendants       = (Get-Median -Values ($childCounts.ToArray() | ForEach-Object { [double]$_ }))
    samplesAppMB      = @($apps | ForEach-Object { [math]::Round($_, 1) })
    machine           = Get-MachineInfo
}
if ($Json) {
    $result | ConvertTo-Json -Depth 6
} else {
    Write-Host ""
    Write-Host ("steady: app {0:N1} MB, with descendants {1:N1} MB, peak working set {2:N1} MB (median of {3})" -f `
        $result.appMB, $result.totalMB, $result.peakWorkingSetMB, $Runs)
}
