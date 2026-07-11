# NekSSH architecture

NekSSH keeps both distribution modes:

- Browser/server mode: the root Go application, installed as a systemd service or run as a portable executable.
- Desktop mode: the Wails application in `desktop/`, producing a native application window without opening an external browser.

The ongoing refactor moves SSH and SFTP operations into a shared Go package so both entry points use the same validated implementation. The browser entry point exposes it over WebSocket; the desktop entry point exposes it through local Wails bindings and events.

Neither entry point is intended to replace the other.
