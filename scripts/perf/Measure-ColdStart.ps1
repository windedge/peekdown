<#
.SYNOPSIS
Measures the time a Windows app takes from CreateProcess to its first window.

.DESCRIPTION
Each run starts Exe, times the window, ends the process tree and waits for
the system to settle. Time is what PerfWin32.WaitFirstWindow sees: the first
visible, uncloaked, non-empty top-level window of the process. A MyGo window
of native UI is shown on the main thread's turn after NewWindow returns and
draws its first frame there, so this is a few milliseconds more than the
window's OnReadyToShow, and includes the loader and the Go runtime's start.

Warmup runs are dropped: the first start of an executable reads it and the
DLLs from disk. Reboot, or clear the standby list, to report a cold start
rather than a warm one; report both.

.PARAMETER Exe
The executable to measure.

.PARAMETER AppArgs
Arguments for the app, as one line: "-scene list -lines 10000".

.PARAMETER Runs
Measured runs, 10 by default.

.PARAMETER Warmup
Runs before those, dropped, 2 by default.

.PARAMETER TimeoutSec
How long to wait for a window before giving up a run.

.PARAMETER Json
Prints the result as JSON instead of a table.

.EXAMPLE
./Measure-ColdStart.ps1 -Exe C:/tmp/perf/bin/counter-native.exe -Runs 10
#>
param(
    [Parameter(Mandatory = $true)][string]$Exe,
    [string]$AppArgs = "",
    [ValidateRange(1, 1000)][int]$Runs = 10,
    [ValidateRange(0, 100)][int]$Warmup = 2,
    [ValidateRange(1, 600)][int]$TimeoutSec = 30,
    [switch]$Json
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot/PerfCommon.ps1"

$path = (Resolve-Path -LiteralPath $Exe).Path
$samples = New-Object System.Collections.Generic.List[double]
$firstWindowPeak = New-Object System.Collections.Generic.List[long]
for ($i = 0; $i -lt ($Warmup + $Runs); $i++) {
    $p = Start-AppTree -Exe $path -AppArgs $AppArgs
    $title = ""
    $ms = [PerfWin32]::WaitFirstWindow($p.Id, $TimeoutSec * 1000, [ref]$title)
    if ($ms -lt 0) {
        Stop-AppTree -Process $p
        throw "no window appeared within $TimeoutSec s"
    }
    $peak = [PerfWin32]::PeakWorkingSet($p.Id)
    Stop-AppTree -Process $p
    if ($i -ge $Warmup) {
        $samples.Add($ms)
        $firstWindowPeak.Add($peak)
        Write-Host ("run {0,2}: {1,8:N1} ms  (window {2})" -f ($i - $Warmup + 1), $ms, $title)
    } else {
        Write-Host ("warmup {0}: {1,8:N1} ms  (dropped)" -f $i, $ms)
    }
    Start-Sleep -Milliseconds 300
}

$values = $samples.ToArray()
$result = [ordered]@{
    exe           = $path
    exeBytes      = (Get-Item -LiteralPath $path).Length
    runs          = $Runs
    warmup        = $Warmup
    firstWindowMs = [ordered]@{
        min    = [math]::Round((($values | Measure-Object -Minimum).Minimum), 1)
        p50    = [math]::Round((Get-Median -Values $values), 1)
        p90    = [math]::Round((Get-Percentile -Values $values -P 0.9), 1)
        max    = [math]::Round((($values | Measure-Object -Maximum).Maximum), 1)
        mean   = [math]::Round((($values | Measure-Object -Average).Average), 1)
        samples = @($values | ForEach-Object { [math]::Round($_, 1) })
    }
    peakWorkingSetMB = [math]::Round((Get-Median -Values $firstWindowPeak.ToArray()) / 1MB, 1)
    machine       = Get-MachineInfo
}
if ($Json) {
    $result | ConvertTo-Json -Depth 6
} else {
    Write-Host ""
    Write-Host ("first window: p50 {0:N1} ms, p90 {1:N1} ms, min {2:N1}, max {3:N1} over {4} runs" -f `
        $result.firstWindowMs.p50, $result.firstWindowMs.p90, $result.firstWindowMs.min, $result.firstWindowMs.max, $Runs)
    Write-Host ("peak working set at first window: {0:N1} MB" -f $result.peakWorkingSetMB)
}
