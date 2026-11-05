#Requires -Version 7
if (-not $IsWindows) { throw "the perf scripts use Win32: run them on Windows" }

# Shared Win32 helpers for the perf scripts: the first window of a process,
# its private working set, its descendants, and starting and stopping an app
# tree. Dot-source it: . "$PSScriptRoot/PerfCommon.ps1"
#
# Private working set is what Task Manager shows as the Memory of a process,
# the figure MyGo's internal/idlemem reports, so numbers stay comparable.

if (-not ("PerfWin32" -as [type])) {
Add-Type -TypeDefinition @"
using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Text;

public static class PerfWin32 {
    delegate bool EnumWindowsProc(IntPtr hWnd, IntPtr lParam);

    [DllImport("user32.dll")]
    static extern bool EnumWindows(EnumWindowsProc cb, IntPtr lParam);
    [DllImport("user32.dll")]
    static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint pid);
    [DllImport("user32.dll")]
    static extern bool IsWindowVisible(IntPtr hWnd);
    [DllImport("user32.dll")]
    static extern bool GetClientRect(IntPtr hWnd, out RECT r);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    static extern int GetWindowTextW(IntPtr hWnd, StringBuilder s, int n);
    [DllImport("dwmapi.dll")]
    static extern int DwmGetWindowAttribute(IntPtr hWnd, int attr, out int val, int size);
    [DllImport("kernel32.dll")]
    static extern IntPtr CreateToolhelp32Snapshot(uint flags, uint pid);
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode)]
    static extern bool Process32FirstW(IntPtr snap, ref PROCESSENTRY32W e);
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode)]
    static extern bool Process32NextW(IntPtr snap, ref PROCESSENTRY32W e);
    [DllImport("kernel32.dll")]
    static extern bool CloseHandle(IntPtr h);
    [DllImport("kernel32.dll")]
    static extern bool K32GetProcessMemoryInfo(IntPtr h, ref PROCESS_MEMORY_COUNTERS_EX2 c, uint cb);
    [DllImport("kernel32.dll")]
    static extern IntPtr OpenProcess(uint access, bool inherit, uint pid);

    const uint TH32CS_SNAPPROCESS = 0x2;
    const uint PROCESS_QUERY_LIMITED_INFORMATION = 0x1000;
    const int DWMWA_CLOAKED = 14;

    [StructLayout(LayoutKind.Sequential)]
    public struct RECT { public int Left, Top, Right, Bottom; }

    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    public struct PROCESSENTRY32W {
        public uint dwSize;
        public uint cntUsage;
        public uint th32ProcessID;
        public IntPtr th32DefaultHeapID;
        public uint th32ModuleID;
        public uint cntThreads;
        public uint th32ParentProcessID;
        public int pcPriClassBase;
        public uint dwFlags;
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 260)] public string szExeFile;
    }

    [StructLayout(LayoutKind.Sequential)]
    public struct PROCESS_MEMORY_COUNTERS_EX2 {
        public uint cb;
        public uint PageFaultCount;
        public UIntPtr PeakWorkingSetSize;
        public UIntPtr WorkingSetSize;
        public UIntPtr QuotaPeakPagedPoolUsage;
        public UIntPtr QuotaPagedPoolUsage;
        public UIntPtr QuotaPeakNonPagedPoolUsage;
        public UIntPtr QuotaNonPagedPoolUsage;
        public UIntPtr PagefileUsage;
        public UIntPtr PeakPagefileUsage;
        public UIntPtr PrivateUsage;
        public UIntPtr PrivateWorkingSetSize;
        public ulong SharedCommitUsage;
    }

    static IntPtr found;
    static string foundTitle;

    static bool lookFor(IntPtr hWnd, IntPtr lParam) {
        uint pid;
        GetWindowThreadProcessId(hWnd, out pid);
        if (pid != (uint)(int)lParam) return true;
        if (!IsWindowVisible(hWnd)) return true;
        RECT r;
        if (!GetClientRect(hWnd, out r)) return true;
        if (r.Right - r.Left <= 0 || r.Bottom - r.Top <= 0) return true;
        int cloaked;
        if (DwmGetWindowAttribute(hWnd, DWMWA_CLOAKED, out cloaked, 4) == 0 && cloaked != 0) return true;
        StringBuilder sb = new StringBuilder(512);
        GetWindowTextW(hWnd, sb, sb.Capacity);
        found = hWnd;
        foundTitle = sb.ToString();
        return false;
    }

    // WaitFirstWindow returns when pid's first visible, uncloaked, non-empty
    // top-level window appeared: the time from the call, in milliseconds, or
    // -1 when none appeared within timeoutMs. It spins rather than sleeps:
    // a sleep of a millisecond takes 15 while the timer resolution is coarse.
    public static double WaitFirstWindow(int pid, int timeoutMs, out string title) {
        Stopwatch sw = Stopwatch.StartNew();
        while (true) {
            found = IntPtr.Zero;
            foundTitle = "";
            EnumWindows(new EnumWindowsProc(lookFor), (IntPtr)pid);
            if (found != IntPtr.Zero) {
                title = foundTitle;
                return sw.Elapsed.TotalMilliseconds;
            }
            if (sw.ElapsedMilliseconds > timeoutMs) {
                title = "";
                return -1;
            }
            System.Threading.Thread.Sleep(0);
        }
    }

    static bool info(uint pid, out PROCESS_MEMORY_COUNTERS_EX2 c) {
        c = new PROCESS_MEMORY_COUNTERS_EX2();
        IntPtr h = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, pid);
        if (h == IntPtr.Zero) return false;
        try {
            c.cb = (uint)Marshal.SizeOf(typeof(PROCESS_MEMORY_COUNTERS_EX2));
            return K32GetProcessMemoryInfo(h, ref c, c.cb);
        } finally {
            CloseHandle(h);
        }
    }

    // PrivateWorkingSet returns the pages of pid no other process shares.
    public static long PrivateWorkingSet(int pid) {
        PROCESS_MEMORY_COUNTERS_EX2 c;
        if (!info((uint)pid, out c)) return -1;
        return (long)c.PrivateWorkingSetSize.ToUInt64();
    }

    // PeakWorkingSet returns the most working set pid held.
    public static long PeakWorkingSet(int pid) {
        PROCESS_MEMORY_COUNTERS_EX2 c;
        if (!info((uint)pid, out c)) return -1;
        return (long)c.PeakWorkingSetSize.ToUInt64();
    }

    // Descendants returns the processes pid started, and theirs.
    public static int[] Descendants(int pid) {
        Dictionary<int, List<int>> children = new Dictionary<int, List<int>>();
        IntPtr snap = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
        if (snap == new IntPtr(-1)) return new int[0];
        try {
            PROCESSENTRY32W e = new PROCESSENTRY32W();
            e.dwSize = (uint)Marshal.SizeOf(typeof(PROCESSENTRY32W));
            if (Process32FirstW(snap, ref e)) {
                do {
                    int parent = (int)e.th32ParentProcessID;
                    List<int> list;
                    if (!children.TryGetValue(parent, out list)) {
                        list = new List<int>();
                        children[parent] = list;
                    }
                    list.Add((int)e.th32ProcessID);
                } while (Process32NextW(snap, ref e));
            }
        } finally {
            CloseHandle(snap);
        }
        List<int> outPids = new List<int>();
        HashSet<int> seen = new HashSet<int>();
        seen.Add(pid);
        Queue<int> queue = new Queue<int>();
        List<int> starts;
        if (children.TryGetValue(pid, out starts)) foreach (int c in starts) queue.Enqueue(c);
        while (queue.Count > 0) {
            int p = queue.Dequeue();
            if (!seen.Add(p)) continue;
            outPids.Add(p);
            List<int> more;
            if (children.TryGetValue(p, out more)) foreach (int c in more) queue.Enqueue(c);
        }
        return outPids.ToArray();
    }
}
"@
}

# Start-AppTree starts Exe and returns the process. ExtraEnv replaces what the
# environment inherits, for the app alone. With LogPath, the app's standard
# error goes to that file and its standard output beside it, as
# Start-Process redirects, so what the app logs survives ending it.
function Start-AppTree {
    param(
        [Parameter(Mandatory = $true)][string]$Exe,
        [string]$AppArgs = "",
        [hashtable]$ExtraEnv = @{},
        [string]$LogPath = ""
    )
    $path = (Resolve-Path -LiteralPath $Exe).Path
    $saved = @{}
    foreach ($k in $ExtraEnv.Keys) {
        $saved[$k] = [Environment]::GetEnvironmentVariable($k)
        [Environment]::SetEnvironmentVariable($k, [string]$ExtraEnv[$k])
    }
    try {
        $start = @{
            FilePath         = $path
            WorkingDirectory = (Split-Path -Parent $path)
            PassThru         = $true
        }
        if ($AppArgs) { $start.ArgumentList = $AppArgs }
        if ($LogPath) {
            $start.RedirectStandardError = $LogPath
            $start.RedirectStandardOutput = "$LogPath.out"
        }
        $p = Start-Process @start
    } finally {
        foreach ($k in $saved.Keys) { [Environment]::SetEnvironmentVariable($k, $saved[$k]) }
    }
    return $p
}

# Stop-AppTree ends a process and its descendants, and waits for them to go.
function Stop-AppTree {
    param([Parameter(Mandatory = $true)][System.Diagnostics.Process]$Process)
    $pids = @()
    $pids += [PerfWin32]::Descendants($Process.Id)
    $pids += $Process.Id
    foreach ($procId in $pids) {
        $proc = Get-Process -Id $procId -ErrorAction SilentlyContinue
        if ($proc) { $proc.Kill() }
    }
    try { [void]$Process.WaitForExit(10000) } catch { }
}

# Get-Percentile returns the p-th of Values (0..1), nearest rank.
function Get-Percentile {
    param([double[]]$Values, [double]$P)
    if ($Values.Count -eq 0) { return 0 }
    $s = @($Values | Sort-Object)
    if ($s.Count -eq 1) { return $s[0] }
    $i = [int][math]::Floor($P * ($s.Count - 1))
    return $s[$i]
}

# Get-Median returns the median of Values.
function Get-Median {
    param([double[]]$Values)
    return Get-Percentile -Values $Values -P 0.5
}

# Get-MachineInfo describes the machine a measurement was taken on.
function Get-MachineInfo {
    $os = Get-CimInstance Win32_OperatingSystem
    $cpu = (Get-CimInstance Win32_Processor | Select-Object -First 1).Name
    return [ordered]@{
        os         = "$($os.Caption) $($os.Version)"
        arch       = $env:PROCESSOR_ARCHITECTURE
        cpu        = $cpu.Trim()
        cores      = $env:NUMBER_OF_PROCESSORS
        memoryGB   = [math]::Round($os.TotalVisibleMemorySize / 1MB, 1)
        powershell = $PSVersionTable.PSVersion.ToString()
        measuredAt = (Get-Date).ToString("o")
    }
}

