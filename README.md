# KeyTray

![KeyTray Logo](src/cmd/assets/keytray.svg)

**KeyTray** is a lightweight Linux application that displays the battery percentage of a
Keychron M3 mouse in your system tray.  It uses the `go-hid` library to read the
battery level from the HID device and `deedles.dev/tray` for a native tray icon.

---

## Features

- **Real‑time battery status** – Updates the tray icon and tooltip as the
  battery percentage changes.
- **Zero external dependencies** – Written in Go, with a single binary.

---

## Prerequisites

- Go 1.26.2 or newer
- `libudev` development headers

On Debian/Ubuntu you can install the headers with:

```bash
sudo apt install libudev-dev
```

---

## Installation

```bash
git clone https://github.com/voxors/KeyTray.git
cd KeyTray

# Build the binary
go build -o keytray ./src/cmd

# (Optional) Move it to a location in your $PATH
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
Clicking the icon will activate the application (currently a placeholder).

---

## Development

Feel free to file issues or pull requests.

---

## License

This project is licensed under the MIT License – see the [LICENSE](LICENSE) file for details.

---

## Contact

Author: Jonathan Croteau‑Dicaire
Email: jonathan.croteau.dicaire@gmail.com
