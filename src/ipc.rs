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
    AllowSetForegroundWindow, ASFW_ANY, FindWindowW, SetForegroundWindow, ShowWindow, IsIconic, SW_RESTORE,
    GetWindowThreadProcessId
};
#[cfg(windows)]
use windows_sys::Win32::System::Threading::{AttachThreadInput, GetCurrentThreadId};
#[cfg(windows)]
use std::ffi::OsStr;
#[cfg(windows)]
use std::os::windows::ffi::OsStrExt;

const PIPE_NAME: &str = "peekdown.pipe";

#[derive(Serialize, Deserialize, Debug)]
pub enum IpcMessage {
    OpenFiles(Vec<PathBuf>),
    FocusWindow,
}

pub fn send_message(message: IpcMessage) -> anyhow::Result<()> {
    #[cfg(windows)]
    unsafe {
        let title: Vec<u16> = OsStr::new("Peekdown").encode_wide().chain(Some(0)).collect();
        let hwnd = FindWindowW(std::ptr::null(), title.as_ptr());
        
        if hwnd != std::ptr::null_mut() {
            let target_thread_id = GetWindowThreadProcessId(hwnd, std::ptr::null_mut());
            let current_thread_id = GetCurrentThreadId();

            // 1. Allow any process to take foreground
            AllowSetForegroundWindow(ASFW_ANY);

            // 2. Attach input processing mechanism
            let attached = if target_thread_id != current_thread_id {
                AttachThreadInput(current_thread_id, target_thread_id, 1) != 0
            } else {
                false
            };

            // 3. Restore if iconic
            if IsIconic(hwnd) != 0 {
                ShowWindow(hwnd, SW_RESTORE);
            }

            // 4. Force foreground
            SetForegroundWindow(hwnd);

            // 5. Detach
            if attached {
                AttachThreadInput(current_thread_id, target_thread_id, 0);
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