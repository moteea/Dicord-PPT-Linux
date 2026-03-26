# Discord Push-to-Talk (PTT) for Linux

Discord Push-to-Talk for Linux is now a Go tool that enables global PTT for Discord on Linux, including Wayland setups where Discord push-to-talk may not work reliably.

Discord on Linux can have trouble with global Push-to-Talk, especially on Wayland. This project works around that by listening for an input event and sending the configured Discord shortcut.

Use this project if you want global Push-to-Talk for Discord on Linux using a mouse side button or another input device.

## What this project does
- Watches a Linux input device such as a mouse side button.
- Detects and saves your device path and button code.
- Sends your configured Discord Push-to-Talk shortcut when that button is pressed and released.
- Can run directly from the repo, from a built binary, or through the included Home Manager module.

## Quick start
1. Run Discord in X11 mode so you can set the Discord Push-to-Talk keybind.
2. Install the required packages for your Linux distribution.
3. Build or run the Go tool.
4. Detect your input device and button with `setup`.
5. Start the daemon so push-to-talk works while Discord is running.

## Installation
### 1) Open Discord in X11 mode (important on Wayland)
Run Discord with:

```bash
discord --enable-features=UseOzonePlatform --ozone-platform=x11
```

Use this Discord session to set your keybind.

### 2) Set Discord PTT keybind
In Discord:
1. `User Settings -> Voice & Video`
2. Set `Input Mode = Push to Talk`
3. Set PTT keybind to `Shift + =`

If you use another combo, it must match your tool config later.

### 3) Install required packages
Core (required for PTT):

### Ubuntu / Debian
```bash
sudo apt update
sudo apt install golang-go xdotool
```

### Fedora
```bash
sudo dnf install golang xdotool
```

### Arch
```bash
sudo pacman -S go xdotool
```

Optional (for the Home Manager Rofi menu and notifications):

- Ubuntu / Debian: `sudo apt install rofi libnotify-bin`
- Fedora: `sudo dnf install rofi libnotify`
- Arch: `sudo pacman -S rofi libnotify`

### 4) Normal install
Build the binary from this repo:

```bash
go build -o discord-ptt-go .
```

Install it to a stable user path:

```bash
mkdir -p ~/.local/bin
cp ./discord-ptt-go ~/.local/bin/discord-ptt-go
chmod +x ~/.local/bin/discord-ptt-go
```

Verify the install:

```bash
~/.local/bin/discord-ptt-go help
```

### 5) Nix install
Build with Nix:

```bash
nix-build default.nix
```

Verify the install:

```bash
./result/bin/discord-ptt-go help
```

If you want a development shell instead:

```bash
nix-shell
go run . help
```

## Configuration
### 6) Run setup to detect your device and button
Using the built binary:

```bash
~/.local/bin/discord-ptt-go setup
```

Using Nix:

```bash
./result/bin/discord-ptt-go setup
```

Or directly with Go:

```bash
go run . setup
```

`setup` prompts for the Discord shortcut, defaults to `Shift + =`, then waits for your button press and saves the detected values.

### 7) Runtime config location
By default, the tool stores runtime state in:

```text
./state
```

The directory may contain:

- `config.json`
- `config_detected.json`
- `shortcut_override.json`

If you want to use another location:

```bash
~/.local/bin/discord-ptt-go setup --config-dir ~/.config/ptt-go
~/.local/bin/discord-ptt-go daemon --config-dir ~/.config/ptt-go
```

### 8) Start the daemon
Using the built binary:

```bash
~/.local/bin/discord-ptt-go daemon
```

Using Nix:

```bash
./result/bin/discord-ptt-go daemon
```

Using Go directly:

```bash
go run . daemon
```

### 9) Auto-start at login (systemd user service)
If you want it to start automatically, create `~/.config/systemd/user/discord-ptt.service`:

```ini
[Unit]
Description=Discord Mouse PTT

[Service]
ExecStart=%h/.local/bin/discord-ptt-go daemon --config-dir %h/.config/ptt-go
Restart=always
Environment=DISPLAY=:0

[Install]
WantedBy=default.target
```

If you built the binary somewhere else, update `ExecStart` to match your real path.

Enable it:

```bash
systemctl --user daemon-reload
systemctl --user enable --now discord-ptt.service
```

## Alternative setup: Nix / Home Manager
This repo includes a Home Manager module:

`./ptt.nix`

Add it to your Home Manager config, for example:

```nix
{
  imports = [
    ./ptt.nix
  ];
}
```

Apply Home Manager:

```bash
home-manager switch
```

This module:
- Builds and installs `discord-ptt-go`
- Writes `~/.config/ptt-go/config.json`
- Seeds `~/.config/ptt-go/config_detected.json`
- Writes `~/.config/ptt-go/shortcut_override.json`
- Installs `~/.config/ptt-go/PTTManager.sh`
- Installs `~/.config/ptt-go/RofiPTT.sh`

## Optional Rofi menu
When using the Home Manager module, this repo generates:

- `~/.config/ptt-go/PTTManager.sh`
- `~/.config/ptt-go/RofiPTT.sh`

Run it with:

```bash
~/.config/ptt-go/RofiPTT.sh
```

What it does:
- Start/stop/restart the PTT daemon
- Run setup to detect a mouse button
- View daemon logs
- Show a simple help entry

## Troubleshooting
1. Confirm the Discord keybind matches `DISCORD_SHORTCUT` in your config.
2. Check the saved config:

```bash
go run . print-config
```

3. Make sure Discord is running with:

```bash
discord --enable-features=UseOzonePlatform --ozone-platform=x11
```

4. If the daemon starts but does not react to button presses, confirm your user can read the chosen input device.
5. If you are using a custom config location, make sure you pass the same `--config-dir` value to both `setup` and `daemon`.

## License
MIT. See [LICENSE](./LICENSE).
