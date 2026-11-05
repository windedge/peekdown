<#
.SYNOPSIS
Scrolls an app with the mouse wheel and reports its frame times.

.DESCRIPTION
Starts Exe with MYGO_FRAME_STATS=all, waits for its first window, puts the
pointer at the window's center, and sends wheel notches there for Seconds.
That keeps the app drawing frames the way a wheel does, on the display's
refresh, so the log holds the frame time distribution of a real scroll: GPU,
compositor and display pacing included.

The app ends afterwards, and the log is parsed by Measure-FrameStats.ps1. An
app whose view scrolls nothing draws no frames: point the test at a document
tall enough to fill the seconds of the run.

Wheel input comes from the system, not from the app's automation, so any
build works. Where a scripted scroll has to be exact, drive the app with
mygo-automation instead, in a build made with -tags e2e and the automation
plugin, and parse the same log.

.PARAMETER Exe
The executable to measure.

.PARAMETER AppArgs
Arguments for the app, as one line: a document path, say.

.PARAMETER Seconds
How long to scroll.

.PARAMETER NotchesPerSecond
Wheel notches sent a second, 30 by default: a notch every other frame, which
keeps a smooth scroll animating without flooding the input queue.

.PARAMETER BudgetMs
The frame budget for the report, 16.7 for a 60 Hz display.

.PARAMETER LogPath
Where the frame log goes, the temp directory otherwise.

.EXAMPLE
./Invoke-ScrollTest.ps1 -Exe C:/tmp/perf/bin/peekdown.exe -AppArgs 'C:/tmp/doc 10k.md' -Seconds 4
#>
param(
    [Parameter(Mandatory = $true)][string]$Exe,
    [string]$AppArgs = "",
    [int]$Seconds = 4,
    [int]$NotchesPerSecond = 30,
    [double]$BudgetMs = 16.7,
    [string]$LogPath = ""
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot/PerfCommon.ps1"

if (-not ("PerfInput" -as [type])) {
Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;

public static class PerfInput {
    delegate bool EnumWindowsProc(IntPtr hWnd, IntPtr lParam);

    [DllImport("user32.dll")] static extern bool EnumWindows(EnumWindowsProc cb, IntPtr lParam);
    [DllImport("user32.dll")] static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint pid);
    [DllImport("user32.dll")] static extern bool IsWindowVisible(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern bool GetClientRect(IntPtr h, out RECT r);
    [DllImport("user32.dll")] public static extern bool ClientToScreen(IntPtr h, ref POINT p);
    [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
    [DllImport("user32.dll")] static extern void mouse_event(uint flags, uint dx, uint dy, int data, UIntPtr extra);
    [StructLayout(LayoutKind.Sequential)] public struct RECT { public int Left, Top, Right, Bottom; }
    [StructLayout(LayoutKind.Sequential)] public struct POINT { public int X, Y; }

    const uint MOUSEEVENTF_WHEEL = 0x0800;

    static IntPtr hit;
    static int wanted;

    static bool look(IntPtr hWnd, IntPtr lParam) {
        uint pid;
        GetWindowThreadProcessId(hWnd, out pid);
        if (pid != (uint)wanted || !IsWindowVisible(hWnd)) return true;
        RECT r;
        if (!GetClientRect(hWnd, out r)) return true;
        if (r.Right - r.Left <= 0 || r.Bottom - r.Top <= 0) return true;
        hit = hWnd;
        return false;
    }

    // FindWindow returns the first visible top-level window of pid.
    public static IntPtr FindWindow(int pid) {
        wanted = pid;
        hit = IntPtr.Zero;
        EnumWindows(new EnumWindowsProc(look), IntPtr.Zero);
        return hit;
    }

    // CenterOnWindow puts the pointer at the middle of the window's client
    // area, where a wheel notch goes to it, and returns whether it did.
    public static bool CenterOnWindow(IntPtr hWnd) {
        RECT r;
        if (!GetClientRect(hWnd, out r)) return false;
        POINT p = new POINT();
        p.X = (r.Right - r.Left) / 2;
        p.Y = (r.Bottom - r.Top) / 2;
        if (!ClientToScreen(hWnd, ref p)) return false;
        return SetCursorPos(p.X, p.Y);
    }

    // Wheel sends notches of 120 units, negative for down.
    public static void Wheel(int notches) { mouse_event(MOUSEEVENTF_WHEEL, 0, 0, -120 * notches, UIntPtr.Zero); }
}
"@
}

if (-not $LogPath) {
    $LogPath = Join-Path $env:TEMP ("mygo-frames-{0:yyyyMMdd-HHmmss}.log" -f (Get-Date))
}
$path = (Resolve-Path -LiteralPath $Exe).Path
$p = Start-AppTree -Exe $path -AppArgs $AppArgs -ExtraEnv @{ MYGO_FRAME_STATS = "all" } -LogPath $LogPath
$title = ""
$ms = [PerfWin32]::WaitFirstWindow($p.Id, 30000, [ref]$title)
if ($ms -lt 0) {
    Stop-AppTree -Process $p
    throw "no window appeared within 30 s"
}
Start-Sleep -Milliseconds 800
$hwnd = [PerfInput]::FindWindow($p.Id)
if ($hwnd -eq [IntPtr]::Zero -or -not [PerfInput]::CenterOnWindow($hwnd)) {
    Stop-AppTree -Process $p
    throw "could not put the pointer over the window"
}
Write-Host ("scrolling '{0}' for {1} s at {2} notches/s, log {3}" -f $title, $Seconds, $NotchesPerSecond, $LogPath)
$interval = [int](1000 / [math]::Max(1, $NotchesPerSecond))
$deadline = (Get-Date).AddSeconds($Seconds)
while ((Get-Date) -lt $deadline) {
    [PerfInput]::Wheel(1)
    Start-Sleep -Milliseconds $interval
}
Stop-AppTree -Process $p
Start-Sleep -Milliseconds 300
& "$PSScriptRoot/Measure-FrameStats.ps1" -Path $LogPath -BudgetMs $BudgetMs -DurationSeconds $Seconds
