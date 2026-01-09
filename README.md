# README

## About

This is the official Wails Vanilla template.

You can configure the project by editing `wails.json`. More information about the project settings can be found
here: https://wails.io/docs/reference/project-config

## System Requirements

### Linux

On Linux systems, you need to install clipboard tools for the clipboard synchronization to work:

**Debian/Ubuntu:**
```bash
sudo apt-get install xclip
```

**Fedora/RHEL:**
```bash
sudo dnf install xclip
```

**Arch Linux:**
```bash
sudo pacman -S xclip
```

Alternatively, you can use `xsel` or `wl-clipboard` (for Wayland) instead of `xclip`.

## Live Development

To run in live development mode, run `wails dev` in the project directory. This will run a Vite development
server that will provide very fast hot reload of your frontend changes. If you want to develop in a browser
and have access to your Go methods, there is also a dev server that runs on http://localhost:34115. Connect
to this in your browser, and you can call your Go code from devtools.

## Building

To build a redistributable, production mode package, use `wails build`.
