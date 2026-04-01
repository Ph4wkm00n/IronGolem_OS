// IronGolem OS - Tauri Desktop Shell
// Entry point for the local-first desktop application.

#![cfg_attr(
    all(not(debug_assertions), target_os = "windows"),
    windows_subsystem = "windows"
)]

use tauri::{
    CustomMenuItem, Manager, SystemTray, SystemTrayEvent, SystemTrayMenu, SystemTrayMenuItem,
};

/// Greet command exposed to the frontend via Tauri's invoke API.
#[tauri::command]
fn greet(name: &str) -> String {
    format!("Hello, {}! Welcome to IronGolem OS.", name)
}

/// Build the system tray menu with standard controls.
fn build_system_tray() -> SystemTray {
    let show = CustomMenuItem::new("show".to_string(), "Show Window");
    let health = CustomMenuItem::new("health".to_string(), "Health Center");
    let quit = CustomMenuItem::new("quit".to_string(), "Quit");

    let tray_menu = SystemTrayMenu::new()
        .add_item(show)
        .add_native_item(SystemTrayMenuItem::Separator)
        .add_item(health)
        .add_native_item(SystemTrayMenuItem::Separator)
        .add_item(quit);

    SystemTray::new().with_menu(tray_menu)
}

fn main() {
    tauri::Builder::default()
        .system_tray(build_system_tray())
        .on_system_tray_event(|app, event| match event {
            SystemTrayEvent::LeftClick { .. } => {
                if let Some(window) = app.get_window("main") {
                    let _ = window.show();
                    let _ = window.set_focus();
                }
            }
            SystemTrayEvent::MenuItemClick { id, .. } => match id.as_str() {
                "show" => {
                    if let Some(window) = app.get_window("main") {
                        let _ = window.show();
                        let _ = window.set_focus();
                    }
                }
                "health" => {
                    // TODO: Open health center panel in the frontend.
                }
                "quit" => {
                    app.exit(0);
                }
                _ => {}
            },
            _ => {}
        })
        .invoke_handler(tauri::generate_handler![greet])
        .run(tauri::generate_context!())
        .expect("error while running IronGolem OS desktop shell");
}
