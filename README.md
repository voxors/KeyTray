# KeyTray

![KeyTray Logo](src/cmd/assets/keytray.svg)

**KeyTray** is a lightweight Linux application that displays the battery percentage of a
Keychron mouse in your system tray. It uses the `go-hid` library to read the
battery level from the HID device and `deedles.dev/tray` for a native tray icon.

---

## Features

- **Real‑time battery status** – Updates the tray icon and tooltip as the
  battery percentage changes.
- **Portable binary** – Written in Go, with a single binary.
- **Supported devices**
  - Keychron M3
  - Keychron M6

---

## Prerequisites

### For Building from Source
- Go 1.26.2 or newer
- `libudev` development headers
  On Debian/Ubuntu, install with: `sudo apt install libudev-dev`

### For Running the Binary
- `libudev.so.1` (usually provided by `libudev1` on Debian/Ubuntu or `systemd-libs` on Fedora)


---

## Installation

You can download the latest releases from the [GitHub Releases](https://github.com/voxors/KeyTray/releases) page.  
You can download the public key of the `deb` and `rpm` package at [Public Key](https://github.com/voxors/KeyTray/blob/main/gpg/public_key.asc)

### Debian/Ubuntu (`.deb`)

```bash
sudo dpkg -i KeyTray_1.1.0_linux_amd64.deb
sudo apt-get install -f  # Fix any missing dependencies
```

### Fedora/RHEL/CentOS (`.rpm`)

```bash
sudo rpm -i KeyTray_1.1.0_linux_amd64.rpm
```

### Generic Linux (`.tar.gz`)

1. Extract the archive:
   ```bash
   tar -xzf KeyTray_linux_amd64.tar.gz
   ```
2. Move the binary to your `$PATH`:
   ```bash
   sudo mv keytray /usr/local/bin/
   ```

---

## Usage

Simply run the binary:

```bash
keytray
```

The icon in the system tray will show the current battery percentage.
Hovering over the icon shows a tooltip with the exact value.
Clicking the icon will open a menu with the same values and a quit option.
## Development

Feel free to file issues or pull requests.

---

## License

This project is licensed under the MIT License – see the [LICENSE](LICENSE) file for details.

---

## Contact

Author: Jonathan Croteau‑Dicaire
Email: jonathan.croteau.dicaire@gmail.com
