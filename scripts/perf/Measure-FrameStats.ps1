<#
.SYNOPSIS
Turns a MYGO_FRAME_STATS log into frame time statistics.

.DESCRIPTION
MyGo logs a frame with MYGO_FRAME_STATS set: "1" for those over 8 ms, a
number for another threshold in milliseconds, "all" for every frame. Each
line tells how long the frame took and how much of that was building,
laying out, painting and presenting it, so a log of a scrolling session is
the frame time distribution of that session.

Pass -DurationSeconds, the length of the session the log covers, to have the
frames per second drawn. The log's own timestamps have whole seconds, too
coarse to time frames by.

.PARAMETER Path
The log file.

.PARAMETER BudgetMs
The frame budget: 16.7 for 60 Hz, 8.3 for 120 Hz.

.PARAMETER DurationSeconds
How long the measured session lasted, for frames per second.

.PARAMETER Json
Prints the result as JSON instead of a table.

.EXAMPLE
$env:MYGO_FRAME_STATS = "all"
./app.exe 2> C:/tmp/frames.log
./Measure-FrameStats.ps1 -Path C:/tmp/frames.log -DurationSeconds 4
#>
param(
    [Parameter(Mandatory = $true)][string]$Path,
    [double]$BudgetMs = 16.7,
    [double]$DurationSeconds = 0,
    [switch]$Json
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot/PerfCommon.ps1"

# The path a frame was drawn by is in parentheses, which hold parentheses of
# their own when the GPU renderer says what it drew: take all of it.
# After build comes the path it was drawn by in parentheses, or "(N passes)" or
# "(painted again)": take all of it, or frames drawn again would be dropped.
$re = [regex]'mygo: frame (\d+) took ([0-9.]+) ms: build ([0-9.]+)(?: \(([^)]*)\))?, layout ([0-9.]+), paint ([0-9.]+), present ([0-9.]+) \((.*)\), other ([0-9.]+); meanwhile the process made about ([0-9]+) allocations, ([0-9.]+) KB, ([0-9]+) text layouts'
$total = New-Object System.Collections.Generic.List[double]
$build = New-Object System.Collections.Generic.List[double]
$layout = New-Object System.Collections.Generic.List[double]
$paint = New-Object System.Collections.Generic.List[double]
$present = New-Object System.Collections.Generic.List[double]
$gc = 0
$first = -1
$last = -1
foreach ($line in [System.IO.File]::ReadLines((Resolve-Path -LiteralPath $Path).Path)) {
    $m = $re.Match($line)
    if (-not $m.Success) { continue }
    $n = [int]$m.Groups[1].Value
    if ($first -lt 0) { $first = $n }
    $last = $n
    $total.Add([double]$m.Groups[2].Value)
    $build.Add([double]$m.Groups[3].Value)
    $layout.Add([double]$m.Groups[5].Value)
    $paint.Add([double]$m.Groups[6].Value)
    $present.Add([double]$m.Groups[7].Value)
    if ($line -match "GC: \d+ cycles ended") { $gc++ }
}
if ($total.Count -eq 0) { throw "no frame lines in $Path" }

$t = $total.ToArray()
$over = @($t | Where-Object { $_ -gt $BudgetMs })
$over2 = @($t | Where-Object { $_ -gt 2 * $BudgetMs })
$result = [ordered]@{
    log              = (Resolve-Path -LiteralPath $Path).Path
    frames           = $t.Count
    framesGaps       = ($last - $first + 1) - $t.Count
    budgetMs         = $BudgetMs
    totalMs          = [ordered]@{
        p50     = [math]::Round((Get-Percentile -Values $t -P 0.5), 2)
        p90     = [math]::Round((Get-Percentile -Values $t -P 0.9), 2)
        p99     = [math]::Round((Get-Percentile -Values $t -P 0.99), 2)
        max     = [math]::Round((($t | Measure-Object -Maximum).Maximum), 2)
        mean    = [math]::Round((($t | Measure-Object -Average).Average), 2)
    }
    overBudget       = $over.Count
    overBudgetShare  = [math]::Round(100 * $over.Count / $t.Count, 1)
    overTwiceBudget  = $over2.Count
    phasesMs         = [ordered]@{
        build   = [math]::Round((Get-Median -Values $build.ToArray()), 2)
        layout  = [math]::Round((Get-Median -Values $layout.ToArray()), 2)
        paint   = [math]::Round((Get-Median -Values $paint.ToArray()), 2)
        present = [math]::Round((Get-Median -Values $present.ToArray()), 2)
    }
    framesWithGC     = $gc
}
if ($DurationSeconds -gt 0) {
    $result.framesPerSecond = [math]::Round($t.Count / $DurationSeconds, 1)
}
if ($Json) {
    $result | ConvertTo-Json -Depth 6
} else {
    Write-Host ("{0} frames over {1} (median {2:N2} ms, p90 {3:N2}, p99 {4:N2}, max {5:N2})" -f `
        $result.frames, $Path, $result.totalMs.p50, $result.totalMs.p90, $result.totalMs.p99, $result.totalMs.max)
    Write-Host ("over {0} ms: {1} frames ({2}%), over {3} ms: {4}" -f `
        $BudgetMs, $result.overBudget, $result.overBudgetShare, (2 * $BudgetMs), $result.overTwiceBudget)
    Write-Host ("phases (median): build {0:N2} ms, layout {1:N2}, paint {2:N2}, present {3:N2}; {4} frames saw GC" -f `
        $result.phasesMs.build, $result.phasesMs.layout, $result.phasesMs.paint, $result.phasesMs.present, $gc)
    if ($result.Contains("framesPerSecond")) {
        Write-Host ("frames drawn per second: {0:N1}" -f $result.framesPerSecond)
    }
}
