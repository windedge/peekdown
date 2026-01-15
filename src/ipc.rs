use std::io::{prelude::*, BufReader, BufWriter};
use std::path::PathBuf;
use interprocess::local_socket::{
    prelude::*,
    GenericFilePath,
    ListenerOptions,
    Stream,
};
use serde::{Deserialize, Serialize};
use smol::channel::Sender;
use std::thread;

#[cfg(windows)]
use windows_sys::Win32::UI::WindowsAndMessaging::{
    AllowSetForegroundWindow, EnumWindows, GetWindowTextW, GetWindowTextLengthW,
    GetWindowThreadProcessId,
};
#[cfg(windows)]
use windows_sys::Win32::Foundation::HWND;

const PIPE_NAME: &str = "peekdown.pipe";

#[derive(Serialize, Deserialize, Debug)]
pub enum IpcMessage {
    OpenFiles(Vec<PathBuf>),
    FocusWindow,
}

pub fn send_message(message: IpcMessage) -> anyhow::Result<()> {
    // Authorize the target process to set foreground window
    // The actual SetForegroundWindow call must happen in the target process
    #[cfg(windows)]
    unsafe {
        let hwnd = find_peekdown_window();

        if !hwnd.is_null() {
            let mut pid: u32 = 0;
            GetWindowThreadProcessId(hwnd, &mut pid);
            if pid != 0 {
                AllowSetForegroundWindow(pid);
            }
        }
    }

    let name_str = if cfg!(windows) {
        format!("\\\\.\\pipe\\ருங்கள்{}", PIPE_NAME)
    } else {
        format!("/tmp/{}.sock", PIPE_NAME)
    };
    let name = name_str.to_fs_name::<GenericFilePath>()?;

    let mut conn = Stream::connect(name)?;
    let mut writer = BufWriter::new(&conn);
    
    serde_json::to_writer(&mut writer, &message)?;
    writer.flush()?;
    
    Ok(())
}

pub fn spawn_ipc_server(tx: Sender<IpcMessage>) -> anyhow::Result<()> {
    let name_str = if cfg!(windows) {
        format!("\\\\.\\pipe\\ருங்கள்{}", PIPE_NAME)
    } else {
        format!("/tmp/{}.sock", PIPE_NAME)
    };
    let name = name_str.to_fs_name::<GenericFilePath>()?;

    let listener = match ListenerOptions::new().name(name).create_sync() {
        Ok(l) => l,
        Err(e) if e.kind() == std::io::ErrorKind::AddrInUse => {
             eprintln!("Socket address in use. If on Unix, verify if socket file is stale.");
             return Err(e.into());
        },
        Err(e) => return Err(e.into()),
    };

    thread::spawn(move || {
        for conn in listener.incoming().filter_map(|x| x.ok()) {
            let tx = tx.clone();
            thread::spawn(move || {
                let reader = BufReader::new(conn);
                let deserializer = serde_json::Deserializer::from_reader(reader);
                let iterator = deserializer.into_iter::<IpcMessage>();

                for msg in iterator {
                    match msg {
                        Ok(msg) => {
                             let _ = smol::block_on(tx.send(msg));
                        }
                        Err(e) => {
                            eprintln!("IPC deserialize error: {}", e);
                            break;
                        }
                    }
                }
            });
        }
    });

    Ok(())
}

/// Find the Peekdown window by enumerating windows and checking title.
/// Window title can be "Peekdown" or "{filename} - Peekdown".
#[cfg(windows)]
fn find_peekdown_window() -> HWND {
    use std::sync::atomic::{AtomicIsize, Ordering};

    static FOUND_HWND: AtomicIsize = AtomicIsize::new(0);
    FOUND_HWND.store(0, Ordering::SeqCst);

    unsafe extern "system" fn enum_callback(hwnd: HWND, _: isize) -> i32 {
        unsafe {
            let len = GetWindowTextLengthW(hwnd);
            if len == 0 {
                return 1; // Continue enumeration
            }

            let mut buffer: Vec<u16> = vec![0; (len + 1) as usize];
            GetWindowTextW(hwnd, buffer.as_mut_ptr(), len + 1);

            // Convert to string and check if it ends with "Peekdown" or equals "Peekdown"
            let title = String::from_utf16_lossy(&buffer[..len as usize]);
            if title == "Peekdown" || title.ends_with(" - Peekdown") {
                FOUND_HWND.store(hwnd as isize, Ordering::SeqCst);
                return 0; // Stop enumeration
            }

            1 // Continue enumeration
        }
    }

    unsafe {
        EnumWindows(Some(enum_callback), 0);
        FOUND_HWND.load(Ordering::SeqCst) as HWND
    }
}